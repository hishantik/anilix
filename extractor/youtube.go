package extractor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hishantik/anilix/curl"
	"github.com/hishantik/anilix/source"
)

const youtubeReferer = "https://www.youtube.com/"

type YoutubeExtractor struct{}

func NewYoutubeExtractor() *YoutubeExtractor {
	return &YoutubeExtractor{}
}

func (e *YoutubeExtractor) Name() string {
	return "youtube"
}

func (e *YoutubeExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "youtube.com") ||
		strings.Contains(lower, "youtu.be") ||
		strings.Contains(lower, "yewtu.be") ||
		strings.Contains(lower, "invidious")
}

func (e *YoutubeExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = youtubeReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	// Try to find direct video URL (mp4)
	videoURL := e.extractVideoURL(html)
	if videoURL != "" {
		return []*source.Stream{{
			Provider: "youtube",
			Quality:  "auto",
			URL:      videoURL,
			Referer:  referer,
		}}, nil
	}

	return nil, fmt.Errorf("no video found")
}

func (e *YoutubeExtractor) extractVideoURL(html string) string {
	patterns := []string{
		`"url"\s*:\s*"([^"]+\.mp4[^"]*)"`,
		`"fmtStreamMap"\s*:\s*"[^"]*url%3D([^&]+)`,
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

func (e *YoutubeExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}
