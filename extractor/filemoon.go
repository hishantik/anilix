package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hishantik/anilix/curl"
	"github.com/hishantik/anilix/source"
)

const filemoonReferer = "https://filemoon.sx/"

// FilemoonExtractor resolves streams from filemoon.sx embed pages.
// It handles encrypted payloads and direct m3u8 URLs.
type FilemoonExtractor struct{}

// NewFilemoonExtractor creates a new FilemoonExtractor for registration.
func NewFilemoonExtractor() *FilemoonExtractor {
	return &FilemoonExtractor{}
}

// Name returns the extractor identifier.
func (e *FilemoonExtractor) Name() string {
	return "filemoon"
}

// CanHandle returns true if the URL contains "filemoon" or "moon"+"file" in its domain.
func (e *FilemoonExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "filemoon") ||
		strings.Contains(lower, "moon") && strings.Contains(lower, "file")
}

// Extract fetches the filemoon embed page and resolves streams via encrypted data or m3u8.
func (e *FilemoonExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = filemoonReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
		"Origin":     "https://filemoon.sx",
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	// Try to extract encrypted data from the page
	encryptedData := extractEncryptedData(html)
	if encryptedData == "" {
		// Try alternative: look for direct m3u8 URL
		m3u8URL := extractM3U8URL(html)
		if m3u8URL != "" {
			return e.extractFromM3U8(ctx, m3u8URL, referer)
		}
		return nil, fmt.Errorf("no stream data found")
	}

	// Decrypt the payload
	decrypted, err := DecryptPayload(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Parse decrypted JSON to get stream URL
	return e.parseDecryptedStream(decrypted, referer)
}

// extractFromM3U8 parses an m3u8 master playlist into quality variant streams.
func (e *FilemoonExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
	variants, err := ParseMasterPlaylistCurl(ctx, m3u8URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse m3u8: %w", err)
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no variants found")
	}

	streams := make([]*source.Stream, 0, len(variants))
	for _, v := range variants {
		streams = append(streams, &source.Stream{
			Provider: "filemoon",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

// parseDecryptedStream parses decrypted JSON or raw m3u8 content into stream objects.
func (e *FilemoonExtractor) parseDecryptedStream(data, referer string) ([]*source.Stream, error) {
	// Try to parse as JSON
	var streamData struct {
		Sources []struct {
			File  string `json:"file"`
			Type  string `json:"type"`
			Label string `json:"label"`
		} `json:"sources"`
	}

	if err := json.Unmarshal([]byte(data), &streamData); err != nil {
		// Not JSON - might be a direct URL
		// Try parsing as m3u8
		if strings.Contains(data, "#EXTM3U") {
			return e.extractFromM3U8(context.Background(), data, referer)
		}
		return nil, fmt.Errorf("failed to parse stream data: %w", err)
	}

	streams := make([]*source.Stream, 0)
	for _, s := range streamData.Sources {
		streams = append(streams, &source.Stream{
			Provider: "filemoon",
			Quality:  s.Label,
			URL:      s.File,
			Referer:  referer,
		})
	}

	return streams, nil
}

// extractEncryptedData scans HTML for Base64-encoded encrypted video data in various formats.
func extractEncryptedData(html string) string {
	// Look for data in script tags or data attributes
	patterns := []string{
		`data-value="([^"]+)"`,
		`data-source="([^"]+)"`,
		`window\.atob\("([^"]+)"\)`,
		`sources:\s*\{[^}]*file:\s*"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Try to find Base64-encoded data in a specific format
	re := regexp.MustCompile(`(?i)(?:encrypted|videoSources|playerData)["']?\s*[:=]\s*["']?([A-Za-z0-9+/=]{20,})`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// extractM3U8URL finds m3u8 playlist URLs in the page HTML.
func extractM3U8URL(html string) string {
	re := regexp.MustCompile(`(?i)(https?://[^\s"']+\.m3u8[^\s"']*)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractSubtitles fetches the embed page and extracts subtitle track URLs.
func (e *FilemoonExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	if referer == "" {
		referer = filemoonReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	return extractSubtitlesFromHTML(html), nil
}
