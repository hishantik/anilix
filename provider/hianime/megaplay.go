package hianime

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hishantik/anilix/curl"
)

// MegaPlay (the "HD-1" server) does not expose a window.__P blob. Its player
// calls /stream/getSources, which returns either an encrypted "enc" payload or
// a plain "sources" list. The encrypted payload is AES-256-CBC with a fixed
// key/IV embedded in e1-player.min.js, and the resulting CDN URL needs a
// short-lived HMAC token before the master playlist can be fetched.
const (
	megaPlayHost     = "megaplay.buzz"
	megaPlayOrigin   = "https://" + megaPlayHost
	megaPlayReferer  = megaPlayOrigin + "/"
	megaPlayAPI      = megaPlayOrigin + "/stream/getSources"
	megaPlayKey      = "i?LMTAx0Q6,:}50U"
	megaPlayIV       = "W0;27ToaUpl_P%'c"
	megaPlayTokenKey = "MpCdnT0k3n!9f2K#xQ7vL5mR8wN1pY4s"
	megaPlayTokenTTL = 90 * time.Second
)

// isMegaPlay reports whether an embed URL belongs to the MegaPlay player.
func isMegaPlay(embedURL string) bool {
	return strings.Contains(embedURL, megaPlayHost)
}

var megaPlayPlayerRe = regexp.MustCompile(`(?s)<[a-z]+[^>]*\bid="megaplay-player"[^>]*>`)

// megaPlayDataID extracts the player's data-id from the embed page. The id in
// the URL path is not equivalent: it can address a different rendition.
func megaPlayDataID(page string) string {
	tag := megaPlayPlayerRe.FindString(page)
	if tag == "" {
		return ""
	}
	return attr(tag, "data-id")
}

// megaPlayType maps the embed's /sub or /dub path to the player's audio type.
func megaPlayType(embedURL string) string {
	u, err := url.Parse(embedURL)
	if err != nil {
		return "sub"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return "sub"
	}
	switch strings.ToLower(parts[len(parts)-1]) {
	case "dub":
		return "dub"
	default:
		return "sub"
	}
}

// GetMegaPlayConfig resolves a MegaPlay embed into a playable master URL.
func GetMegaPlayConfig(ctx context.Context, embedURL string) (*StreamJSON, error) {
	page, err := get(ctx, embedURL, megaPlayReferer)
	if err != nil {
		return nil, fmt.Errorf("megaplay embed failed: %w", err)
	}

	dataID := megaPlayDataID(page)
	if dataID == "" {
		return nil, fmt.Errorf("megaplay embed has no data-id")
	}

	query := url.Values{}
	query.Set("id", dataID)
	query.Set("platform", "OTHER")
	query.Set("st", "other")
	// NOTE: the embed page's own query (e.g. ?s=tcdn) must NOT be forwarded.
	// With s=tcdn the API returns a decoy payload pointing at megap.shiora.top,
	// whose "segments" are tiktokcdn ad images; without it we get the real CDN.

	headers := map[string]string{
		"User-Agent":       UserAgent,
		"Referer":          embedURL,
		"Origin":           megaPlayOrigin,
		"X-Requested-With": "XMLHttpRequest",
	}
	body, err := curl.GetResolved(ctx, megaPlayAPI+"?"+query.Encode(), headers)
	if err != nil {
		return nil, fmt.Errorf("megaplay sources failed: %w", err)
	}

	var response megaPlayResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("megaplay sources returned non-JSON: %s", bodySnippet(body))
	}

	rawURL, err := response.streamURL()
	if err != nil {
		return nil, err
	}

	src, err := withAudioType(rawURL, megaPlayType(embedURL))
	if err != nil {
		return nil, fmt.Errorf("megaplay source URL invalid: %w", err)
	}
	if src, err = attachMegaPlayToken(src, time.Now()); err != nil {
		return nil, fmt.Errorf("megaplay token failed: %w", err)
	}

	cfg := &StreamJSON{Src: src}
	for _, track := range response.Tracks {
		if track.File == "" {
			continue
		}
		if track.Kind != "" && track.Kind != "captions" {
			continue // thumbnails and other non-subtitle tracks
		}
		cfg.Subtitles = append(cfg.Subtitles, SubtitleTrack{
			Src:     track.File,
			Default: track.Default,
			Label:   track.Label,
		})
	}
	return cfg, nil
}

type megaPlayTrack struct {
	File    string `json:"file"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Default bool   `json:"default"`
}

type megaPlayResponse struct {
	Enc     string          `json:"enc"`
	Sources json.RawMessage `json:"sources"`
	Tracks  []megaPlayTrack `json:"tracks"`
}

// streamURL returns the decrypted enc payload, or the plain sources list when
// no enc key is present.
func (r *megaPlayResponse) streamURL() (string, error) {
	if enc := strings.TrimSpace(r.Enc); enc != "" {
		rawURL, err := decryptMegaPlaySources(enc)
		if err != nil {
			return "", fmt.Errorf("megaplay decrypt failed: %w", err)
		}
		return rawURL, nil
	}

	if len(r.Sources) > 0 {
		var list []struct {
			File string `json:"file"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(r.Sources, &list); err == nil {
			for _, source := range list {
				if u := firstNonEmpty(source.File, source.URL); u != "" {
					return u, nil
				}
			}
		}

		var single struct {
			File string `json:"file"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(r.Sources, &single); err == nil {
			if u := firstNonEmpty(single.File, single.URL); u != "" {
				return u, nil
			}
		}

		var plain string
		if err := json.Unmarshal(r.Sources, &plain); err == nil && plain != "" {
			return plain, nil
		}
	}

	return "", fmt.Errorf("megaplay response has no enc or sources")
}

// decryptMegaPlaySources decrypts the base64url AES-CBC payload returned in
// the "enc" field. The plaintext is JSON: {"file":"https://.../master.m3u8"}.
func decryptMegaPlaySources(enc string) (string, error) {
	raw, err := decodeBase64URL(enc)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(padKey(megaPlayKey, 32))
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || len(raw)%block.BlockSize() != 0 {
		return "", fmt.Errorf("payload is not a whole number of blocks")
	}

	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, padKey(megaPlayIV, block.BlockSize())).CryptBlocks(out, raw)
	out, err = pkcs7Unpad(out, block.BlockSize())
	if err != nil {
		return "", err
	}

	var payload struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("decrypted payload is not JSON: %w", err)
	}
	if payload.File == "" {
		return "", fmt.Errorf("decrypted payload has no file")
	}
	return payload.File, nil
}

// decodeBase64URL accepts both raw and padded base64url.
func decodeBase64URL(s string) ([]byte, error) {
	if raw, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// padKey zero-pads (or truncates) a string to size bytes.
func padKey(s string, size int) []byte {
	key := make([]byte, size)
	copy(key, s)
	return key
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded length")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("invalid padding size")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	return data[:len(data)-pad], nil
}

// withAudioType appends the audio selection (t=sub/t=dub) before the CDN
// token is generated, matching the player's withAudioType().
func withAudioType(rawURL, mediaType string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := u.Query()
	query.Set("t", mediaType)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

var megaPlayPathRe = regexp.MustCompile(`/([a-f0-9]{32})/([a-f0-9]{32})/`)

// attachMegaPlayToken adds the master-playlist token required by the MegaPlay
// CDN. Tokens expire after 90 seconds; the playlist itself is fetched right
// away and variant/segment URLs do not need a token.
func attachMegaPlayToken(rawURL string, now time.Time) (string, error) {
	if strings.Contains(rawURL, "token=") {
		return rawURL, nil
	}
	m := megaPlayPathRe.FindStringSubmatch(rawURL)
	if m == nil {
		// Some mirrors do not use the tokenized path layout.
		return rawURL, nil
	}

	message := fmt.Sprintf("%d|%s/%s", now.Add(megaPlayTokenTTL).Unix(), m[1], m[2])
	mac := hmac.New(sha256.New, []byte(megaPlayTokenKey))
	mac.Write([]byte(message))
	token := base64.RawURLEncoding.EncodeToString([]byte(message)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + "token=" + url.QueryEscape(token), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
