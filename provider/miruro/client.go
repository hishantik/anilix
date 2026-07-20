package miruro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/source"
)

type cachedCatalog struct {
	catalog *Catalog
	expires time.Time
}

type Client struct {
	transport Transport
	now       func() time.Time

	mu            sync.Mutex
	catalogs      map[int]cachedCatalog
	providerOrder []string
	configLoaded  bool
}

func NewClient(transport Transport) *Client {
	return &Client{transport: transport, now: time.Now, catalogs: make(map[int]cachedCatalog)}
}

func (c *Client) Catalog(ctx context.Context, anilistID int) (*Catalog, error) {
	if anilistID <= 0 {
		return nil, fmt.Errorf("anime has no AniList ID")
	}
	c.mu.Lock()
	if cached, ok := c.catalogs[anilistID]; ok && c.now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.catalog, nil
	}
	c.mu.Unlock()

	body, err := c.transport.Get(ctx, "episodes", url.Values{"anilistId": {strconv.Itoa(anilistID)}})
	if err != nil {
		return nil, fmt.Errorf("fetch Miruro episodes: %w", err)
	}
	var response catalogResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Miruro episodes: %w", err)
	}
	providerOrder := c.loadProviderOrder(ctx)
	catalog := normalizeCatalog(anilistID, response, providerOrder)
	c.mu.Lock()
	c.catalogs[anilistID] = cachedCatalog{catalog: catalog, expires: c.now().Add(5 * time.Minute)}
	c.mu.Unlock()
	return catalog, nil
}

func (c *Client) loadProviderOrder(ctx context.Context) []string {
	c.mu.Lock()
	if c.configLoaded {
		order := append([]string(nil), c.providerOrder...)
		c.mu.Unlock()
		return order
	}
	c.mu.Unlock()

	order := append([]string(nil), defaultProviderOrder...)
	body, err := c.transport.Get(ctx, "config", nil)
	if err == nil {
		var response configResponse
		if json.Unmarshal(body, &response) == nil && len(response.ProviderOrder) > 0 {
			var native []string
			for _, provider := range response.ProviderOrder {
				config, ok := response.Streaming[provider]
				if !ok || (config.Visible && config.Player != "iframe") {
					native = append(native, provider)
				}
			}
			if len(native) > 0 {
				order = native
			}
		}
	}
	c.mu.Lock()
	if !c.configLoaded {
		c.providerOrder = append([]string(nil), order...)
		c.configLoaded = true
	}
	order = append([]string(nil), c.providerOrder...)
	c.mu.Unlock()
	return order
}

func normalizeCatalog(anilistID int, response catalogResponse, providerOrder []string) *Catalog {
	catalog := &Catalog{entries: make(map[string]map[string][]EpisodeCandidate)}
	providers := make([]string, 0, len(response.Providers))
	for provider := range response.Providers {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providerRank(providers[i], providerOrder) < providerRank(providers[j], providerOrder)
	})
	for _, provider := range providers {
		for category, episodes := range response.Providers[provider].Episodes {
			if catalog.entries[category] == nil {
				catalog.entries[category] = make(map[string][]EpisodeCandidate)
			}
			for _, episode := range episodes {
				number := normalizeEpisodeNumber(episode.Number)
				if episode.ID == "" || number == "" {
					continue
				}
				catalog.entries[category][number] = append(catalog.entries[category][number], EpisodeCandidate{
					AniListID: anilistID,
					ID:        episode.ID, Provider: provider, Category: category,
					Number: number, Title: episode.Title,
				})
			}
		}
	}
	return catalog
}

func providerRank(provider string, providerOrder []string) string {
	for i, candidate := range providerOrder {
		if provider == candidate {
			return fmt.Sprintf("%02d", i)
		}
	}
	return "99-" + provider
}

func normalizeEpisodeNumber(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if number, err := strconv.ParseFloat(text, 64); err == nil {
			return strconv.FormatFloat(number, 'f', -1, 64)
		}
		return strings.TrimSpace(text)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return ""
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

func (c *Client) Sources(ctx context.Context, candidate EpisodeCandidate) ([]*source.Stream, error) {
	query := url.Values{
		"episodeId": {candidate.ID},
		"provider":  {candidate.Provider},
		"category":  {candidate.Category},
		"anilistId": {strconv.Itoa(candidate.AniListID)},
	}
	body, err := c.transport.Get(ctx, "sources", query)
	if err != nil {
		return nil, fmt.Errorf("fetch Miruro sources from %s: %w", candidate.Provider, err)
	}
	var response sourcesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Miruro sources from %s: %w", candidate.Provider, err)
	}
	subtitles := mapSubtitles(append(response.Subtitles, response.Captions...))
	streams := make([]*source.Stream, 0, len(response.Streams))
	for _, item := range response.Streams {
		if !safeRemoteURL(item.URL) {
			continue
		}
		quality := item.Quality
		if quality == "" {
			quality = item.Label
		}
		if quality == "" {
			quality = "auto"
		}
		referer := item.Headers["Referer"]
		if referer == "" {
			referer = item.Headers["referer"]
		}
		streams = append(streams, &source.Stream{
			URL: item.URL, Quality: quality, Provider: candidate.Provider,
			Referer: referer, NeedsReferrer: referer != "", Subtitles: subtitles,
		})
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("Miruro provider %s returned no playable streams", candidate.Provider)
	}
	return streams, nil
}

func mapSubtitles(items []subtitleWire) []source.Subtitle {
	result := make([]source.Subtitle, 0, len(items))
	for _, item := range items {
		rawURL := item.URL
		if rawURL == "" {
			rawURL = item.File
		}
		if !safeRemoteURL(rawURL) {
			continue
		}
		language := item.Language
		if language == "" {
			language = item.Lang
		}
		if language == "" {
			language = item.Label
		}
		result = append(result, source.Subtitle{Language: language, URL: rawURL})
	}
	return result
}

func safeRemoteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func (c *Client) Close() error { return c.transport.Close() }
