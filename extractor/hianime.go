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

const hianimeReferer = "https://hianime.com/"

// HianimeExtractor resolves streams from hianime embed pages.
// It handles both direct m3u8 URLs and AES-encrypted payloads.
type HianimeExtractor struct{}

// NewHianimeExtractor creates a new HianimeExtractor instance for registration.
func NewHianimeExtractor() *HianimeExtractor {
	return &HianimeExtractor{}
}

// Name returns the extractor identifier used for priority lookup.
func (e *HianimeExtractor) Name() string {
	return "hianime"
}

// CanHandle returns true if the URL contains "hianime" in its domain.
func (e *HianimeExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "hianime") ||
		strings.Contains(lower, "hianime.com")
}

// Extract fetches the hianime embed page and resolves stream URLs.
// It tries m3u8 extraction first, falling back to encrypted data decryption.
func (e *HianimeExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = hianimeReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	// Try to extract m3u8 URL from the page
	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL == "" {
		// Try to find encrypted data
		encryptedData := e.extractEncryptedData(html)
		if encryptedData != "" {
			decrypted, err := DecryptPayload(encryptedData)
			if err != nil {
				return nil, fmt.Errorf("decryption failed: %w", err)
			}
			return e.parseDecryptedStream(decrypted, referer)
		}
		return nil, fmt.Errorf("no stream data found")
	}

	// Parse m3u8 and get variants
	return e.extractFromM3U8(ctx, m3u8URL, referer)
}

// extractFromM3U8 parses an m3u8 master playlist and returns all quality variants as streams.
func (e *HianimeExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "hianime",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

// parseDecryptedStream parses the decrypted JSON payload into stream objects.
// Handles both JSON with sources array and raw m3u8 content.
func (e *HianimeExtractor) parseDecryptedStream(data, referer string) ([]*source.Stream, error) {
	var streamData struct {
		Sources []struct {
			File  string `json:"file"`
			Type  string `json:"type"`
			Label string `json:"label"`
		} `json:"sources"`
		Link string `json:"link"`
	}

	if err := json.Unmarshal([]byte(data), &streamData); err != nil {
		if strings.Contains(data, "#EXTM3U") {
			return e.extractFromM3U8(context.Background(), data, referer)
		}
		return nil, fmt.Errorf("failed to parse stream data: %w", err)
	}

	// If there's a direct link, try that
	if streamData.Link != "" {
		return e.extractFromM3U8(context.Background(), streamData.Link, referer)
	}

	streams := make([]*source.Stream, 0)
	for _, s := range streamData.Sources {
		streams = append(streams, &source.Stream{
			Provider: "hianime",
			Quality:  s.Label,
			URL:      s.File,
			Referer:  referer,
		})
	}

	return streams, nil
}

// extractM3U8URL scans HTML for m3u8 URLs in script tags, data attributes, or direct links.
func (e *HianimeExtractor) extractM3U8URL(html string) string {
	// Look for m3u8 URL in the page
	patterns := []string{
		`(?i)src\s*:\s*["']([^"']*\.m3u8[^"']*)["']`,
		`(?i)file\s*:\s*["']([^"']*\.m3u8[^"']*)["']`,
		`https?://[^\s"']+\.m3u8[^\s"']*`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractEncryptedData finds Base64-encoded encrypted stream data in the page HTML.
func (e *HianimeExtractor) extractEncryptedData(html string) string {
	patterns := []string{
		`data-value="([^"]+)"`,
		`data-source="([^"]+)"`,
		`videoSources\s*=\s*["']([^"']+)["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// ExtractSubtitles fetches the embed page and extracts subtitle track URLs from it.
func (e *HianimeExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	if referer == "" {
		referer = hianimeReferer
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
