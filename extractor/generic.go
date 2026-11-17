package extractor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hishantik/anilix/curl"
	"github.com/hishantik/anilix/source"
)

// GenericExtractor scans any page for video URLs (m3u8, mp4) as a fallback
type GenericExtractor struct{}

func NewGenericExtractor() Extractor {
	return &GenericExtractor{}
}

func (e *GenericExtractor) Name() string {
	return "generic"
}

func (e *GenericExtractor) CanHandle(url string) bool {
	// Generic extractor handles any URL - used as fallback
	return true
}

func (e *GenericExtractor) Extract(ctx context.Context, pageURL, referer string) ([]*source.Stream, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, pageURL, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var streams []*source.Stream
	seen := make(map[string]bool)

	// Pattern 1: m3u8 URLs
	m3u8Pattern := regexp.MustCompile(`https?://[^\s"'\)\]]+\.m3u8[^\s"'\)\]]*`)
	for _, match := range m3u8Pattern.FindAllString(html, -1) {
		url := strings.TrimSpace(match)
		if !seen[url] && isValidVideoURL(url) {
			seen[url] = true
			streams = append(streams, &source.Stream{
				URL:      url,
				Provider: "generic",
				Referer:  pageURL,
				Quality:  "auto",
			})
		}
	}

	// Pattern 2: mp4 URLs (stricter - only match if URL ends with .mp4 or contains /d/ for direct video)
	mp4Pattern := regexp.MustCompile(`https?://[^\s"'\)\]]+\.mp4[^\s"'\)\]]*`)
	for _, match := range mp4Pattern.FindAllString(html, -1) {
		url := strings.TrimSpace(match)
		if !seen[url] && isValidMP4URL(url) {
			seen[url] = true
			streams = append(streams, &source.Stream{
				URL:      url,
				Provider: "generic",
				Referer:  pageURL,
				Quality:  "auto",
			})
		}
	}

	// Pattern 3: JavaScript video players (video.js, player.js, etc.)
	playerPattern := regexp.MustCompile(`(?:src|file|source)\s*[=:]\s*["']([^"']+\.(?:m3u8|mp4)[^"']*)["']`)
	for _, match := range playerPattern.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			if !seen[url] && isValidVideoURL(url) {
				seen[url] = true
				streams = append(streams, &source.Stream{
					URL:      url,
					Provider: "generic",
					Referer:  pageURL,
					Quality:  "auto",
				})
			}
		}
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no video URLs found in page")
	}

	return streams, nil
}

func (e *GenericExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}

// isValidVideoURL checks if URL looks like a valid m3u8 URL
func isValidVideoURL(url string) bool {
	lower := strings.ToLower(url)
	// Must contain .m3u8 as a proper extension
	if !strings.HasSuffix(lower, ".m3u8") && !strings.Contains(lower, ".m3u8?") {
		return false
	}
	// Skip non-video extensions anywhere in URL
	skipExts := []string{".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".html", ".htm"}
	for _, ext := range skipExts {
		if strings.Contains(lower, ext) {
			return false
		}
	}
	skipPatterns := []string{"facebook.com", "twitter.com", "instagram.com", "tiktok.com"}
	for _, p := range skipPatterns {
		if strings.Contains(url, p) {
			return false
		}
	}
	return true
}

// isValidMP4URL checks if URL looks like a valid direct mp4 URL
func isValidMP4URL(url string) bool {
	lower := strings.ToLower(url)
	// Must contain .mp4
	if !strings.HasSuffix(lower, ".mp4") && !strings.Contains(lower, ".mp4?") {
		return false
	}
	// Skip non-video patterns (css, js, jpg, etc.) anywhere in URL
	skipExts := []string{".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".html", ".htm"}
	for _, ext := range skipExts {
		if strings.Contains(lower, ext) {
			return false
		}
	}
	// Must be a direct video path (contains /d/ for mp4upload, or ends with .mp4)
	if !strings.HasSuffix(lower, ".mp4") && !strings.Contains(url, "/d/") {
		return false
	}
	return true
}
