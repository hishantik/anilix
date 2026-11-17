package extractor

import (
	"context"

	"github.com/hishantik/anilix/source"
)

// Extractor is the interface for resolving playable stream URLs from embed/hosting pages.
// Each extractor handles a specific hosting provider (e.g., hianime, filemoon, wixmp).
// Extractors are registered at init time and resolved by URL pattern matching.
type Extractor interface {
	// Name returns the extractor's identifier, used for priority ordering and logging.
	Name() string

	// CanHandle returns true if this extractor recognizes the URL's hosting provider.
	CanHandle(url string) bool

	// Extract fetches the embed page and resolves direct stream URLs (m3u8/mp4).
	Extract(ctx context.Context, url, referer string) ([]*source.Stream, error)

	// ExtractSubtitles extracts subtitle track URLs from the embed page (optional).
	ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error)
}

// NoSubtitlesExtractor is a convenience wrapper for extractors that don't support subtitles.
// It implements Extractor by embedding another extractor and returning nil for ExtractSubtitles.
type NoSubtitlesExtractor struct {
	Extractor
}

// ExtractSubtitles returns nil since this extractor doesn't support subtitle extraction.
func (e *NoSubtitlesExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}

// ProviderPriority controls the extraction order when multiple extractors match a URL.
// Lower number = higher priority. This mirrors ani-cli's provider preference order.
var ProviderPriority = map[string]int{
	"hianime":  1,
	"filemoon": 2,
	"wixmp":    3,
	"youtube":  4,
}

// extractors holds all registered extractors, populated via init() calls.
var extractors []Extractor

// Register adds an extractor to the global registry. Called from each extractor's init().
func Register(e Extractor) {
	extractors = append(extractors, e)
}

// All returns all registered extractors for iteration during stream resolution.
func All() []Extractor {
	return extractors
}

// Resolve finds the first registered extractor that can handle the given URL.
// Returns nil if no extractor matches, allowing callers to skip unsupported URLs.
func Resolve(url string) Extractor {
	for _, e := range extractors {
		if e.CanHandle(url) {
			return e
		}
	}
	return nil
}
