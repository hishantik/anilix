package Allanime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/curl"
	"github.com/hishantik/anilix/extractor"
	"github.com/hishantik/anilix/source"
)

// fixedProvider defines ani-cli's 5 fixed providers with their markers
// These match the sed patterns in ani-cli's generate_link function
var fixedProviders = []struct {
	markers  []string // SourceName markers to match (case-insensitive)
	name     string   // Internal provider name
	priority int      // ani-cli order: 1=wixmp, 2=youtube, 3=sharepoint, 4=filemoon, 5=hianime
}{
	{[]string{"default"}, "wixmp", 1},
	{[]string{"yt-mp4", "youtube"}, "youtube", 2},
	{[]string{"s-mp4", "sharepoint"}, "sharepoint", 3},
	{[]string{"fm-mp4", "filemoon", "fm-hls"}, "filemoon", 4},
	{[]string{"luf-mp4", "hianime"}, "hianime", 5},
}

// isFixedProvider checks if a source name matches one of the 5 fixed providers
func isFixedProvider(sourceName string) (string, bool) {
	name := strings.ToLower(sourceName)
	name = strings.ReplaceAll(name, " ", "")

	for _, fp := range fixedProviders {
		for _, marker := range fp.markers {
			if name == marker {
				return fp.name, true
			}
		}
	}
	return "", false
}

// AllanimeProvider implements source.Source for AllAnime GraphQL API
type AllanimeProvider struct {
	client      *AllanimeClient
	translation string // "sub" or "dub"
}

func NewAllanimeProvider() *AllanimeProvider {
	return &AllanimeProvider{
		client:      NewAllanimeClient(),
		translation: "sub",
	}
}

func (a *AllanimeProvider) Name() string {
	return "AllAnime"
}

func (a *AllanimeProvider) ID() string {
	return "allanime"
}

// Search implements source.Source - searches anime via AllAnime GraphQL
func (a *AllanimeProvider) Search(query string) ([]*source.Anime, error) {
	ctx := context.Background()

	edges, err := a.client.SearchShows(ctx, query, 10, 1, a.translation)
	if err != nil {
		return nil, fmt.Errorf("AllAnime search failed: %w", err)
	}

	animes := make([]*source.Anime, 0, len(edges))
	for _, show := range edges {
		anime := a.client.MapToAnime(&show)
		animes = append(animes, anime)
	}

	return animes, nil
}

// SeasonsOf returns seasons for the anime
func (a *AllanimeProvider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	return []source.Season{{Number: 1, Name: "Season 1"}}, nil
}

// EpisodesOf returns episodes for a specific season
func (a *AllanimeProvider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	if anime.AllAnimeID == "" {
		return nil, fmt.Errorf("anime has no AllAnimeID")
	}

	ctx := context.Background()
	episodesMap, err := a.client.GetShowEpisodes(ctx, anime.AllAnimeID, a.translation)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %w", err)
	}

	epList, ok := episodesMap[a.translation]
	if !ok {
		epList, ok = episodesMap["sub"]
		if !ok {
			return nil, fmt.Errorf("no episodes found")
		}
	}

	episodes := make([]*source.Episode, 0, len(epList))
	for _, epNum := range epList {
		num, _ := strconv.ParseFloat(epNum, 64)
		ep := &source.Episode{
			Number: num,
			URL:    fmt.Sprintf("https://allmanga.to/watch/%s/%s", anime.AllAnimeID, epNum),
			Anime:  anime,
			Season: season,
		}
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

// StreamsOf returns stream sources for an episode
// Optimized: try ALL providers in parallel, return as soon as we have streams
func (a *AllanimeProvider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	if episode.Anime == nil || episode.Anime.AllAnimeID == "" {
		return nil, fmt.Errorf("episode has no anime or AllAnimeID")
	}

	ctx := context.Background()
	epNum := strconv.FormatFloat(episode.Number, 'f', -1, 64)
	sources, err := a.client.GetEpisodeSources(ctx, episode.Anime.AllAnimeID, epNum, a.translation)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode sources: %w", err)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources found")
	}

	// Use a timeout for extraction (10 seconds)
	extractCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Extract streams from all providers in parallel
	type extractResult struct {
		provider string
		streams  []*source.Stream
	}

	resultCh := make(chan extractResult)
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(s SourceUrl) {
			defer wg.Done()
			streams := extractStreams(extractCtx, s)
			if len(streams) > 0 {
				resultCh <- extractResult{provider: s.SourceName, streams: streams}
			}
		}(src)
	}

	// Wait for first result or timeout
	select {
	case result := <-resultCh:
		// Got streams! Collect any other quick results
		allStreams := result.streams

		// Briefly wait for more results (500ms)
		timeout := time.After(500 * time.Millisecond)
		for {
			select {
			case r := <-resultCh:
				allStreams = append(allStreams, r.streams...)
			case <-timeout:
				sortStreamsByQuality(allStreams)
				return allStreams, nil
			}
		}
	case <-extractCtx.Done():
		// Timeout - no streams found
		return nil, fmt.Errorf("no streams found from any provider")
	}
}

// extractStreams tries all registered extractors for a single source URL
// Optimized: use per-extractor timeout, skip known embed URLs
func extractStreams(ctx context.Context, src SourceUrl) []*source.Stream {
	// Decode hex-encoded URL if needed
	streamUrl := src.SourceUrl
	if isHexEncoded(streamUrl) && !strings.Contains(streamUrl, "/clock") {
		streamUrl = decodeHexProviderID(streamUrl)
	}

	// Determine provider name
	providerName, isFixed := isFixedProvider(src.SourceName)
	if !isFixed {
		// Use actual provider name
		providerName = strings.ToLower(src.SourceName)
		providerName = strings.ReplaceAll(providerName, " ", "")
	}

	// Handle tools.fast4speed.rsvp URLs - direct video URLs (instant, no extraction needed)
	if strings.Contains(streamUrl, "tools.fast4speed.rsvp") {
		return []*source.Stream{{
			Provider:      "youtube",
			Quality:       "auto",
			URL:           streamUrl,
			Referer:       Referer,
			NeedsReferrer: false,
		}}
	}

	// Handle clock URLs - fetch and extract video link
	if strings.Contains(streamUrl, "/clock") {
		return extractClockURL(ctx, streamUrl, providerName)
	}

	// Skip empty URLs
	if streamUrl == "" {
		return nil
	}

	// Skip known embed URLs that won't have direct video
	if isEmbedURL(streamUrl) {
		return nil
	}

	// Use a shorter per-source timeout (5 seconds)
	extCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try all registered extractors
	for _, ext := range extractor.All() {
		if !ext.CanHandle(streamUrl) {
			continue
		}

		// Use a channel to timeout individual extractor calls
		type extResult struct {
			streams []*source.Stream
			err     error
		}
		resultCh := make(chan extResult, 1)
		go func() {
			streams, err := ext.Extract(extCtx, streamUrl, Referer)
			resultCh <- extResult{streams: streams, err: err}
		}()

		select {
		case res := <-resultCh:
			if res.err != nil || len(res.streams) == 0 {
				continue
			}

			result := make([]*source.Stream, 0, len(res.streams))
			for _, s := range res.streams {
				url := s.URL
				if strings.HasPrefix(url, "//") {
					url = "https:" + url
				}
				if isPlayableURL(url) {
					s.URL = url
					s.Provider = providerName
					s.Referer = streamUrl
					s.NeedsReferrer = needsReferrerProvider(providerName)
					result = append(result, s)
				}
			}

			if len(result) > 0 {
				return result
			}
		case <-extCtx.Done():
			// Extractor timed out, try next one
			continue
		}
	}

	return nil
}

// extractClockURL fetches the clock API and extracts the video link
// ani-cli fetches https://allanime.day/apivtwo/clock?id=... and extracts the link
func extractClockURL(ctx context.Context, clockPath, providerName string) []*source.Stream {
	// Build full URL
	clockURL := fixClockPath(clockPath)
	if !strings.HasPrefix(clockPath, "http") {
		clockURL = "https://allanime.day" + fixClockPath(clockPath)
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    Referer,
	}

	body, err := curl.Get(ctx, clockURL, headers)
	if err != nil {
		return nil
	}

	streams, err := parseClockLinks([]byte(body), providerName)
	if err != nil {
		return nil
	}
	return streams
}

func parseClockLinks(body []byte, providerName string) ([]*source.Stream, error) {
	var response struct {
		Link  string `json:"link"`
		Links []struct {
			Link          string `json:"link"`
			ResolutionStr string `json:"resolutionStr"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode clock response: %w", err)
	}
	if response.Link != "" {
		response.Links = append(response.Links, struct {
			Link          string `json:"link"`
			ResolutionStr string `json:"resolutionStr"`
		}{Link: response.Link})
	}
	streams := make([]*source.Stream, 0, len(response.Links))
	for _, candidate := range response.Links {
		link := candidate.Link
		if isHexEncoded(link) {
			link = decodeHexProviderID(link)
		}
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
		if link == "" || isEmbedURL(link) {
			continue
		}
		quality := candidate.ResolutionStr
		if quality == "" {
			quality = "auto"
		} else if _, err := strconv.Atoi(quality); err == nil {
			quality += "p"
		}
		streams = append(streams, &source.Stream{Provider: providerName, Quality: quality, URL: link, Referer: Referer, NeedsReferrer: needsReferrerProvider(providerName)})
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("clock response contained no playable links")
	}
	return streams, nil
}

// sortStreamsByQuality sorts streams by quality (best first)
// Parses quality like "1080p" -> 1080, "720p" -> 720
// "auto" quality goes to the top
func sortStreamsByQuality(streams []*source.Stream) {
	sort.SliceStable(streams, func(i, j int) bool {
		qi := parseQuality(streams[i].Quality)
		qj := parseQuality(streams[j].Quality)
		return qi > qj
	})
}

// parseQuality extracts numeric quality from string like "1080p", "720p", "auto"
// Returns higher number for better quality, 9999 for "auto"
func parseQuality(q string) int {
	if q == "" || strings.ToLower(q) == "auto" {
		return 9999
	}
	// Remove "p" suffix
	q = strings.TrimSuffix(strings.ToLower(q), "p")
	n, err := strconv.Atoi(q)
	if err != nil {
		return 0
	}
	return n
}

// isEmbedURL checks if URL is an embed page that can't be played directly
func isEmbedURL(url string) bool {
	return strings.Contains(url, "ok.ru/videoembed") ||
		strings.Contains(url, "vk.com/video_ext") ||
		strings.Contains(url, "/videoembed") ||
		strings.HasSuffix(url, "/embed") ||
		strings.Contains(url, "/embed/") ||
		strings.Contains(url, "/e/") ||
		strings.Contains(url, "allanime.uns.bio") ||
		strings.Contains(url, "allanime.bio") ||
		strings.Contains(url, "#") ||
		strings.HasSuffix(url, "/player")
}

// isPlayableURL checks if URL is a direct video (m3u8/mp4) that can be played
func isPlayableURL(url string) bool {
	lower := strings.ToLower(url)
	isVideo := strings.Contains(lower, ".m3u8") || strings.Contains(lower, ".mp4")
	if !isVideo {
		return false
	}
	if isEmbedURL(url) {
		return false
	}
	skipExts := []string{".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".html", ".htm"}
	for _, ext := range skipExts {
		if strings.Contains(lower, ext) {
			return false
		}
	}
	return true
}

// needsReferrerProvider returns true if the provider typically requires a Referer header
func needsReferrerProvider(provider string) bool {
	switch strings.ToLower(provider) {
	case "youtube", "sharepoint":
		return false
	default:
		return true
	}
}

// SetTranslation sets sub or dub preference
func (a *AllanimeProvider) SetTranslation(trans string) {
	if trans == "sub" || trans == "dub" {
		a.translation = trans
	}
}

// TranslationType returns current translation setting
func (a *AllanimeProvider) TranslationType() string {
	return a.translation
}
