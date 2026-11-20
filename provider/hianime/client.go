package hianime

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hishantik/anilix/curl"
)

const (
	BaseURL   = "https://hianime.at"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// SearchResult is a show scraped from the search page.
type SearchResult struct {
	ShowID   string // numeric hianime show id (e.g. "1281")
	Slug     string // e.g. "cowboy-bebop-1281"
	Title    string // display title
	Poster   string // image URL
	Type     string // "TV", "MOVIE", "SPECIAL", ...
	Episodes int    // episode count reported by the site
	URL      string // detail page URL
}

// EpisodeEntry maps an episode number to hianime's internal episode data-id,
// which is required by the servers API (the number alone is not enough).
type EpisodeEntry struct {
	Number string
	DataID string
}

// Variant is one quality rendition from a master HLS playlist.
type Variant struct {
	Quality string // e.g. "1080p"
	URL     string // absolute playlist URL
}

// StreamJSON is the decrypted embed config (subset of fields we use).
type StreamJSON struct {
	Src       string
	Subtitles []SubtitleTrack
}

type SubtitleTrack struct {
	Src     string `json:"src"`
	Default bool   `json:"default"`
	Label   string `json:"label"`
}

// get fetches a URL through curl with the hianime browser user agent.
// GetResolved pins the DNS answer to a public resolver, because streaming
// CDNs are commonly blocklisted by device-level DNS filters (which would
// otherwise return a blockpage and break parsing/playback).
func get(ctx context.Context, url, referer string) (string, error) {
	headers := map[string]string{"User-Agent": UserAgent}
	if referer != "" {
		headers["Referer"] = referer
	}
	return curl.GetResolved(ctx, url, headers)
}

// Search scrapes the search page for show id, title, poster and type.
func Search(ctx context.Context, query string) ([]SearchResult, error) {
	page, err := get(ctx, BaseURL+"/search?keyword="+searchEscape(query), "")
	if err != nil {
		return nil, fmt.Errorf("hianime search failed: %w", err)
	}
	if strings.Contains(strings.ToLower(page), "just a moment") {
		return nil, fmt.Errorf("hianime search blocked by Cloudflare")
	}
	return parseSearchResults(page), nil
}

// The search results grid renders one <div class="flw-item"> block per hit.
// Each block holds the poster (with the show id on the inner poster anchor),
// the title link, the type and the episode count. Other sections of the page
// (carousels for latest/top anime) use a different markup with
// 'film-poster item-qtip' and must not be mixed in - so the grid is what we
// split on.
var (
	flwItemRe   = regexp.MustCompile(`<div class="flw-item[^"]*">`)
	anchorRe    = regexp.MustCompile(`<a\s[^>]*>`)
	filmNameRe  = regexp.MustCompile(`(?s)<h3 class="film-name">(.*?)</h3>`)
	posterImgRe = regexp.MustCompile(`<img\s[^>]*class="film-poster-img"[^>]*>`)
	imgSrcRe    = regexp.MustCompile(`src="([^"]*)"`)
	typeRe      = regexp.MustCompile(`<span class="fdi-item">([^<]*)</span>`)
	epsTickRe   = regexp.MustCompile(`tick-item tick-eps">(\d+)<`)
)

// attr extracts an attribute value from a raw HTML tag.
func attr(tag, name string) string {
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

func parseSearchResults(page string) []SearchResult {
	parts := flwItemRe.Split(page, -1)
	results := make([]SearchResult, 0, len(parts))

	for _, part := range parts[1:] {
		var res SearchResult

		// Show id + watch link live on the poster anchor.
		for _, tag := range anchorRe.FindAllString(part, -1) {
			if !strings.Contains(tag, "film-poster-ahref") {
				continue
			}
			res.ShowID = attr(tag, "data-id")
			if res.ShowID == "" {
				continue
			}
			break
		}

		// Title + detail link live in the film-name heading.
		if m := filmNameRe.FindStringSubmatch(part); m != nil {
			for _, tag := range anchorRe.FindAllString(m[1], -1) {
				title := attr(tag, "title")
				if title == "" {
					continue
				}
				res.Title = unescapeHTML(title)
				res.URL = attr(tag, "href")
				break
			}
		}

		// Poster image.
		if m := posterImgRe.FindString(part); m != "" {
			if s := imgSrcRe.FindStringSubmatch(m); s != nil {
				res.Poster = s[1]
			}
		}

		// Type (TV/MOVIE/SPECIAL) and episode count.
		if m := typeRe.FindStringSubmatch(part); m != nil {
			res.Type = strings.ToUpper(strings.TrimSpace(m[1]))
		}
		if m := epsTickRe.FindStringSubmatch(part); m != nil {
			res.Episodes, _ = strconv.Atoi(m[1])
		}

		if res.ShowID == "" || res.Title == "" {
			continue
		}
		res.Slug = slugFromURL(res.URL)
		results = append(results, res)
	}
	return results
}

// slugFromURL extracts the trailing slug (e.g. "cowboy-bebop-1281").
func slugFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	trimmed := rawURL
	if i := strings.IndexAny(trimmed, "?#"); i >= 0 {
		trimmed = trimmed[:i]
	}
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	return trimmed
}

// GetEpisodes scrapes the episode list API for (number, dataId) pairs.
func GetEpisodes(ctx context.Context, showID string) ([]EpisodeEntry, error) {
	page, err := get(ctx, BaseURL+"/api/theme/episode/list/"+showID, "")
	if err != nil {
		return nil, fmt.Errorf("hianime episodes failed: %w", err)
	}

	entries := parseEpisodeList(page)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no episodes found for show %s", showID)
	}
	return entries, nil
}

var episodeItemRe = regexp.MustCompile(`data-number="([^"]*)"[^>]*data-id="(\d+)"`)

func parseEpisodeList(page string) []EpisodeEntry {
	// The API returns escaped HTML; normalize it and split into per-item chunks
	// so data-number/data-id pairs never bleed across entries.
	clean := strings.ReplaceAll(page, "\\", "")
	clean = strings.ReplaceAll(clean, "ep-item", "\nep-item")

	entries := make([]EpisodeEntry, 0, 32)
	for _, chunk := range strings.Split(clean, "\n") {
		m := episodeItemRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		num := strings.TrimSpace(m[1])
		if num == "" {
			continue
		}
		entries = append(entries, EpisodeEntry{Number: num, DataID: m[2]})
	}
	return entries
}

// EmbedCandidate is one playable server for an episode.
type EmbedCandidate struct {
	Type   string // "sub" or "dub"
	Name   string // server name shown on the site (not stable - don't rely on it)
	Embed  string // embed URL
	HasMAL bool   // embed carries the MAL id (/stream/mal/<malid>/...)
}

// GetEmbeds lists the servers available for an episode.
//
// The server *names* change over time (ZokoAnime, HD-1, ...) while the embed
// host stays stable, so candidates are ranked by embed, not by name: only
// embeds on the known player host carry the decodable config blob.
func GetEmbeds(ctx context.Context, epDataID string) ([]EmbedCandidate, error) {
	page, err := get(ctx, BaseURL+"/api/theme/episode/servers?episodeId="+epDataID, "")
	if err != nil {
		return nil, fmt.Errorf("hianime servers failed: %w", err)
	}
	cands := parseServers(page)
	if len(cands) == 0 {
		return nil, fmt.Errorf("no servers found for episode %s", epDataID)
	}
	return cands, nil
}

var serverItemRe = regexp.MustCompile(
	`data-type="([^"]*)"[^>]*data-server-name="([^"]*)"[^>]*data-hash="([^"]*)"`)

func parseServers(page string) []EmbedCandidate {
	clean := strings.ReplaceAll(page, "\\", "")
	clean = strings.ReplaceAll(clean, "server-item", "\nserver-item")

	cands := make([]EmbedCandidate, 0, 4)
	for _, chunk := range strings.Split(clean, "\n") {
		m := serverItemRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m[3])
		if err != nil || len(raw) == 0 {
			continue
		}
		url := string(raw)
		cands = append(cands, EmbedCandidate{
			Type:   m[1],
			Name:   m[2],
			Embed:  url,
			HasMAL: malIDFromEmbed(url) > 0,
		})
	}
	return cands
}

// rankEmbeds orders candidates by preference: the requested translation
// first, then sub, then dub, then anything else. Within a translation, servers
// on a known player host come first. Translation must outrank host: a dub on
// an alternate player is more useful to a dub viewer than a sub on the
// preferred player.
func rankEmbeds(cands []EmbedCandidate, translation string) []EmbedCandidate {
	typeRank := func(c EmbedCandidate) int {
		switch {
		case c.Type == translation:
			return 0
		case c.Type == "sub":
			return 1
		case c.Type == "dub":
			return 2
		default:
			return 3
		}
	}

	ordered := make([]EmbedCandidate, len(cands))
	copy(ordered, cands)
	sort.SliceStable(ordered, func(i, j int) bool {
		ri, rj := typeRank(ordered[i]), typeRank(ordered[j])
		if ri != rj {
			return ri < rj
		}
		return playerHost(ordered[i].Embed) && !playerHost(ordered[j].Embed)
	})
	return ordered
}

// playerHost reports whether the embed is served by a player we can decode.
// Zoko exposes an obfuscated window.__P blob; MegaPlay uses getSources + AES.
func playerHost(embedURL string) bool {
	return strings.Contains(embedURL, "zokoanime.video") || isMegaPlay(embedURL)
}

// obfKey is the repeating XOR key the embed page uses for its base64 config.
const obfKey = "otaku-embed-v1"

// GetStreamConfig fetches the embed page and decodes its obfuscated config.
func GetStreamConfig(ctx context.Context, embedURL string) (*StreamJSON, error) {
	referer := embedURL
	if i := strings.Index(embedURL, "://"); i >= 0 {
		if end := strings.Index(embedURL[i+3:], "/"); end >= 0 {
			referer = embedURL[:i+3+end+1]
		}
	}

	page, err := get(ctx, embedURL, referer)
	if err != nil {
		return nil, fmt.Errorf("hianime embed failed: %w", err)
	}
	return parseStreamConfig(page)
}

var streamBlobRe = regexp.MustCompile(`window\.__P="([^"]*)"`)

func parseStreamConfig(page string) (*StreamJSON, error) {
	m := streamBlobRe.FindStringSubmatch(page)
	if m == nil {
		return nil, fmt.Errorf("no __P config found in embed page")
	}

	cfg, err := deobfuscateBlob(m[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode embed config: %w", err)
	}

	stream := &StreamJSON{}
	if s := cfg.Src; s != "" {
		stream.Src = s
	}
	stream.Subtitles = cfg.Subtitles
	if stream.Src == "" {
		return nil, fmt.Errorf("no m3u8 src in embed config")
	}
	return stream, nil
}

// GetVariants fetches a master HLS playlist and extracts quality variants.
// A body that is neither a master nor a media playlist is an error: CDNs
// often return HTML or a plain-text error with HTTP 200/5xx, and treating
// that as a playable URL makes the player open and close immediately.
func GetVariants(ctx context.Context, masterURL, referer string) ([]Variant, error) {
	playlist, err := get(ctx, masterURL, referer)
	if err != nil {
		return nil, fmt.Errorf("hianime playlist failed: %w", err)
	}
	return variantsFromPlaylist(playlist, masterURL)
}

// variantsFromPlaylist validates and parses an HLS response. It is separated
// from GetVariants so the validation can be tested without a network call.
func variantsFromPlaylist(playlist, masterURL string) ([]Variant, error) {
	body := strings.TrimSpace(strings.TrimPrefix(playlist, "\ufeff"))
	if !strings.HasPrefix(body, "#EXTM3U") {
		return nil, fmt.Errorf("hianime playlist is not HLS: %s", bodySnippet(body))
	}

	if variants := parseMasterPlaylist(body, masterURL); len(variants) > 0 {
		return variants, nil
	}

	// A media playlist (segments, no variants) is playsafe directly.
	if strings.Contains(body, "#EXTINF") || strings.Contains(body, "#EXT-X-TARGETDURATION") {
		return []Variant{{Quality: "auto", URL: masterURL}}, nil
	}

	return nil, fmt.Errorf("hianime playlist has no variants or segments")
}

// bodySnippet returns a short, single-line version of a response body for
// error messages, without dumping a whole page into the TUI.
func bodySnippet(body string) string {
	snippet := strings.Join(strings.Fields(body), " ")
	if snippet == "" {
		return "(empty response)"
	}
	const max = 120
	if len(snippet) > max {
		snippet = snippet[:max] + "..."
	}
	return fmt.Sprintf("%q", snippet)
}

var (
	streamInfRe  = regexp.MustCompile(`#EXT-X-STREAM-INF:.*?RESOLUTION=\d+x(\d+)`)
	relativeRe   = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://`)
	subDefaultRe = regexp.MustCompile(`"src":"([^"]*)"[^{}]*"default":true`)
)

func parseMasterPlaylist(playlist, masterURL string) []Variant {
	base := masterURL
	if i := strings.LastIndex(masterURL, "/"); i >= 0 {
		base = masterURL[:i+1]
	}

	var variants []Variant
	lines := strings.Split(playlist, "\n")
	for i, line := range lines {
		m := streamInfRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// The variant URL is the next non-comment, non-empty line.
		for j := i + 1; j < len(lines); j++ {
			u := strings.TrimSpace(lines[j])
			if u == "" {
				continue
			}
			if strings.HasPrefix(u, "#") {
				continue
			}
			if !relativeRe.MatchString(u) {
				u = base + u
			}
			variants = append(variants, Variant{Quality: m[1] + "p", URL: u})
			break
		}
	}
	return variants
}

// defaultSubtitle returns the URL of the subtitle marked default, if any.
func (s *StreamJSON) defaultSubtitle() string {
	for _, sub := range s.Subtitles {
		if sub.Default && sub.Src != "" {
			return sub.Src
		}
	}
	// Fall back to the first track with a source.
	for _, sub := range s.Subtitles {
		if sub.Src != "" {
			return sub.Src
		}
	}
	return ""
}

// searchEscape makes a query safe for use in the search URL.
func searchEscape(q string) string {
	r := strings.NewReplacer(" ", "+", "&", "%26", "+", "%2B", "?", "%3F", "#", "%23")
	return r.Replace(strings.TrimSpace(q))
}

// unescapeHTML decodes the handful of entities hianime uses in titles.
func unescapeHTML(s string) string {
	r := strings.NewReplacer("&#039;", "'", "&#39;", "'", "&quot;", `"`, "&amp;", "&", "&lt;", "<", "&gt;", ">")
	return strings.TrimSpace(r.Replace(s))
}
