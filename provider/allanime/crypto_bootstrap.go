package Allanime

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type cryptoMaterial struct {
	epoch     int
	buildID   string
	key       []byte
	legacyCTR bool
}

var (
	appURLPattern = regexp.MustCompile(`https://cdn\.(?:mkissa\.net|allanime\.day)/all/mk/_app/immutable/entry/app\.[^"']+\.js`)
	epochPattern  = regexp.MustCompile(`"epoch"\s*:\s*(\d+)`)
	partBPattern  = regexp.MustCompile(`"partB"\s*:\s*"([^"]+)"`)
	maskPattern   = regexp.MustCompile(`(?i)([0-9a-f]{64})`)
	buildPattern  = regexp.MustCompile(`(?is)[0-9a-f]{64}.{0,512}?"(\d+)"`)
	chunkPattern  = regexp.MustCompile(`\.\./chunks/[^"',\]]+\.js`)
)

type cryptoBootstrap struct {
	client  *http.Client
	pageURL string
	now     func() time.Time

	mu        sync.Mutex
	cached    cryptoMaterial
	cachedErr error
	expiresAt time.Time
}

func newCryptoBootstrap(client *http.Client, pageURL string, now func() time.Time) *cryptoBootstrap {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	clientCopy := *client
	previousRedirect := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0].URL.Hostname() != "mkissa.to" {
			if _, err := validateCryptoAssetURL(req.URL.String()); err != nil {
				return fmt.Errorf("blocked crypto asset redirect: %w", err)
			}
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
	return &cryptoBootstrap{client: &clientCopy, pageURL: pageURL, now: now}
}

func (b *cryptoBootstrap) material(ctx context.Context) (cryptoMaterial, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	if now.Before(b.expiresAt) {
		return b.cached, b.cachedErr
	}

	material, err := b.discover(ctx)
	b.cached = material
	b.cachedErr = err
	if err != nil {
		b.expiresAt = now.Add(time.Minute)
	} else {
		b.expiresAt = now.Add(30 * time.Minute)
	}
	return material, err
}

func (b *cryptoBootstrap) discover(ctx context.Context) (cryptoMaterial, error) {
	page, err := b.fetchText(ctx, b.pageURL)
	if err != nil {
		return cryptoMaterial{}, fmt.Errorf("fetch bootstrap page: %w", err)
	}
	appURL, epoch, partB, err := parseBootstrapPage(page)
	if err != nil {
		return cryptoMaterial{}, err
	}
	validatedAppURL, err := validateCryptoAssetURL(appURL)
	if err != nil {
		return cryptoMaterial{}, err
	}
	app, err := b.fetchText(ctx, validatedAppURL.String())
	if err != nil {
		return cryptoMaterial{}, fmt.Errorf("fetch application bundle: %w", err)
	}

	seen := make(map[string]bool)
	for _, chunkPath := range chunkPattern.FindAllString(app, -1) {
		chunkURL := validatedAppURL.ResolveReference(&url.URL{Path: chunkPath})
		if seen[chunkURL.String()] {
			continue
		}
		seen[chunkURL.String()] = true
		if _, err := validateCryptoAssetURL(chunkURL.String()); err != nil {
			continue
		}
		chunk, err := b.fetchText(ctx, chunkURL.String())
		if err != nil {
			continue
		}
		mask, buildID, err := parseCryptoChunk(chunk)
		if err != nil {
			continue
		}
		key, err := deriveCryptoKey(mask, partB)
		if err != nil {
			return cryptoMaterial{}, err
		}
		return cryptoMaterial{epoch: epoch, buildID: buildID, key: key}, nil
	}
	return cryptoMaterial{}, fmt.Errorf("crypto chunks did not expose usable material")
}

func (b *cryptoBootstrap) fetchText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", allAnimeUserAgent)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseBootstrapPage(page string) (string, int, string, error) {
	appURL := appURLPattern.FindString(page)
	epochMatch := epochPattern.FindStringSubmatch(page)
	partBMatch := partBPattern.FindStringSubmatch(page)
	if appURL == "" || len(epochMatch) != 2 || len(partBMatch) != 2 {
		return "", 0, "", fmt.Errorf("bootstrap page did not expose crypto material")
	}
	epoch, err := strconv.Atoi(epochMatch[1])
	if err != nil || epoch <= 0 {
		return "", 0, "", fmt.Errorf("invalid bootstrap epoch %q", epochMatch[1])
	}
	return appURL, epoch, partBMatch[1], nil
}

func parseCryptoChunk(chunk string) (string, string, error) {
	maskMatch := maskPattern.FindStringSubmatch(chunk)
	buildMatch := buildPattern.FindStringSubmatch(chunk)
	if len(maskMatch) != 2 || len(buildMatch) != 2 {
		return "", "", fmt.Errorf("chunk did not expose crypto material")
	}
	return maskMatch[1], buildMatch[1], nil
}

func deriveCryptoKey(maskHex, partB string) ([]byte, error) {
	mask, err := hex.DecodeString(maskHex)
	if err != nil {
		return nil, fmt.Errorf("decode crypto mask: %w", err)
	}
	part, err := base64.StdEncoding.DecodeString(partB)
	if err != nil {
		return nil, fmt.Errorf("decode crypto partB: %w", err)
	}
	if len(mask) != 32 || len(part) != 32 {
		return nil, fmt.Errorf("crypto material must contain two 32-byte values")
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = mask[i] ^ part[i]
	}
	return key, nil
}

func createAAReq(queryHash string, material cryptoMaterial, now time.Time) (string, error) {
	if len(material.key) != 32 {
		return "", fmt.Errorf("aaReq requires a 32-byte key")
	}
	timestamp := now.UnixMilli() / 300000 * 300000
	payload, err := json.Marshal(struct {
		Version int    `json:"v"`
		TS      int64  `json:"ts"`
		Epoch   int    `json:"epoch"`
		BuildID string `json:"buildId"`
		Hash    string `json:"qh"`
	}{1, timestamp, material.epoch, material.buildID, queryHash})
	if err != nil {
		return "", fmt.Errorf("encode aaReq payload: %w", err)
	}
	ivHash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s:%d", material.epoch, material.buildID, queryHash, timestamp)))
	iv := ivHash[:12]
	block, err := aes.NewCipher(material.key)
	if err != nil {
		return "", fmt.Errorf("create aaReq cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create aaReq GCM: %w", err)
	}
	envelope := make([]byte, 13)
	envelope[0] = 1
	copy(envelope[1:], iv)
	envelope = gcm.Seal(envelope, iv, payload, nil)
	return base64.StdEncoding.EncodeToString(envelope), nil
}

func validateCryptoAssetURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("crypto asset must use HTTPS")
	}
	switch parsed.Hostname() {
	case "cdn.mkissa.net", "cdn.allanime.day":
		return parsed, nil
	default:
		return nil, fmt.Errorf("crypto asset host %q is not allowed", parsed.Hostname())
	}
}
