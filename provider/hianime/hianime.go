package hianime

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/source"
)

// Provider implements source.Source for hianime.at (the provider current
// ani-cli scrapes).
type Provider struct {
	translation string

	// The episode list of a long series is over a megabyte and the server
	// list is fetched again for every episode, so both are cached briefly.
	cacheMu  sync.Mutex
	episodes map[string]cachedEpisodes
	servers  map[string]cachedServers
}

type cachedEpisodes struct {
	entries []EpisodeEntry
	fetched time.Time
}

type cachedServers struct {
	candidates []EmbedCandidate
	fetched    time.Time
}

const (
	episodeCacheTTL = 30 * time.Minute
	serverCacheTTL  = 10 * time.Minute
)

// NewProvider creates a hianime provider with the default translation type.
func NewProvider() *Provider {
	return &Provider{
		translation: "sub",
		episodes:    make(map[string]cachedEpisodes),
		servers:     make(map[string]cachedServers),
	}
}

func (p *Provider) Name() string { return "Hianime" }
func (p *Provider) ID() string   { return "hianime" }

// SetTranslation switches between "sub" and "dub" playback.
func (p *Provider) SetTranslation(t string) {
	if t == "dub" {
		p.translation = "dub"
	} else {
		p.translation = "sub"
	}
}

// episodeEntries returns the episode list of a show, using the cache when it
// is still fresh.
func (p *Provider) episodeEntries(ctx context.Context, showID string) ([]EpisodeEntry, error) {
	p.cacheMu.Lock()
	if cached, ok := p.episodes[showID]; ok && time.Since(cached.fetched) < episodeCacheTTL {
		p.cacheMu.Unlock()
		return cached.entries, nil
	}
	p.cacheMu.Unlock()

	entries, err := GetEpisodes(ctx, showID)
	if err != nil {
		return nil, err
	}

	p.cacheMu.Lock()
	p.episodes[showID] = cachedEpisodes{entries: entries, fetched: time.Now()}
	p.cacheMu.Unlock()
	return entries, nil
}

// serverCandidates returns the servers of an episode, using the cache when it
// is still fresh.
func (p *Provider) serverCandidates(ctx context.Context, epDataID string) ([]EmbedCandidate, error) {
	p.cacheMu.Lock()
	if cached, ok := p.servers[epDataID]; ok && time.Since(cached.fetched) < serverCacheTTL {
		p.cacheMu.Unlock()
		return cached.candidates, nil
	}
	p.cacheMu.Unlock()

	cands, err := GetEmbeds(ctx, epDataID)
	if err != nil {
		return nil, err
	}

	p.cacheMu.Lock()
	p.servers[epDataID] = cachedServers{candidates: cands, fetched: time.Now()}
	p.cacheMu.Unlock()
	return cands, nil
}

// Search scrapes hianime.at. MALID is resolved later by the TUI (via the
// AniList title match) or at episode-fetch time (the embed URL carries
// /mal/<id>/).
func (p *Provider) Search(query string) ([]*source.Anime, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	results, err := Search(ctx, query)
	if err != nil {
		return nil, err
	}

	anime := make([]*source.Anime, 0, len(results))
	for _, r := range results {
		a := &source.Anime{
			Name:         r.Title,
			URL:          r.URL,
			Cover:        r.Poster,
			Type:         r.Type,
			EpisodeCount: r.Episodes,
			Source:       p,
		}
		if a.URL == "" {
			a.URL = fmt.Sprintf("%s/watch/%s", BaseURL, r.Slug)
		}
		a.ShowID = r.ShowID
		anime = append(anime, a)
	}
	return anime, nil
}

// SeasonsOf returns nothing: hianime entries are already per-season.
func (p *Provider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	return nil, nil
}

// EpisodesOf lists episodes and opportunistically enriches the anime's MALID
// from the first episode's embed URL, which enables AniSkip and metadata.
func (p *Provider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	if anime == nil || anime.ShowID == "" {
		return nil, fmt.Errorf("episode has no anime or show ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), episodeTimeout)
	defer cancel()

	entries, err := p.episodeEntries(ctx, anime.ShowID)
	if err != nil {
		return nil, err
	}

	episodes := make([]*source.Episode, 0, len(entries))
	for _, e := range entries {
		num, _ := strconv.ParseFloat(e.Number, 64)
		episodes = append(episodes, &source.Episode{
			Number: num,
			URL:    fmt.Sprintf("%s/watch?id=%s&ep=%s", BaseURL, anime.ShowID, e.DataID),
			Anime:  anime,
			Season: season,
		})
	}

	if anime.MALID == 0 && len(entries) > 0 {
		if malID := p.resolveMALID(ctx, entries[0].DataID); malID > 0 {
			anime.MALID = malID
		}
	}

	return episodes, nil
}

// resolveMALID extracts the MAL id from an embed URL. Only some embeds carry
// it (/stream/mal/<malid>/); others use an internal id instead.
func (p *Provider) resolveMALID(ctx context.Context, epDataID string) int {
	cands, err := p.serverCandidates(ctx, epDataID)
	if err != nil {
		return 0
	}
	for _, c := range rankEmbeds(cands, p.translation) {
		if id := malIDFromEmbed(c.Embed); id > 0 {
			return id
		}
	}
	return 0
}

var malIDRe = regexp.MustCompile(`/mal/(\d+)/`)

func malIDFromEmbed(embedURL string) int {
	if m := malIDRe.FindStringSubmatch(embedURL); m != nil {
		if id, err := strconv.Atoi(m[1]); err == nil {
			return id
		}
	}
	return 0
}

// StreamsOf resolves playable quality variants for an episode.
func (p *Provider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	if episode == nil || episode.Anime == nil || episode.Anime.ShowID == "" {
		return nil, fmt.Errorf("episode has no anime or show ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), streamTimeout)
	defer cancel()

	// The servers API needs hianime's internal data-id, not the episode
	// number; look it up in the episode list.
	dataID, err := p.lookupDataID(ctx, episode)
	if err != nil {
		return nil, err
	}

	cands, err := p.serverCandidates(ctx, dataID)
	if err != nil {
		return nil, err
	}

	// Try the ranked servers until one yields a decodable config: server
	// names and player formats change, so a single hardcoded server is not
	// reliable.
	var (
		cfg      *StreamJSON
		embedURL string
		lastErr  error
	)
	for _, c := range rankEmbeds(cands, p.translation) {
		embedURL = c.Embed
		cfg, lastErr = GetStreamConfig(ctx, c.Embed)
		if lastErr == nil {
			break
		}
	}
	if cfg == nil {
		return nil, fmt.Errorf("no playable server for episode: %w", lastErr)
	}

	// Enrich MALID for AniSkip if the TUI doesn't have it yet.
	if episode.Anime.MALID == 0 {
		if malID := malIDFromEmbed(embedURL); malID > 0 {
			episode.Anime.MALID = malID
		}
	}

	referer := embedReferer(embedURL)
	variants, err := GetVariants(ctx, cfg.Src, referer)
	if err != nil {
		return nil, err
	}

	streams := make([]*source.Stream, 0, len(variants))
	for _, v := range variants {
		s := &source.Stream{
			Provider:      "hianime",
			Quality:       v.Quality,
			URL:           v.URL,
			Referer:       referer,
			NeedsReferrer: true,
		}
		if sub := cfg.defaultSubtitle(); sub != "" {
			s.Subtitles = []source.Subtitle{{Language: "eng", URL: sub}}
		}
		streams = append(streams, s)
	}

	sortStreamsByQuality(streams)
	return streams, nil
}

// lookupDataID maps the episode number to hianime's internal data-id.
func (p *Provider) lookupDataID(ctx context.Context, episode *source.Episode) (string, error) {
	// If the episode carries its own watch URL from EpisodesOf, use it.
	if episode.URL != "" {
		if m := regexp.MustCompile(`[?&]ep=(\d+)`).FindStringSubmatch(episode.URL); m != nil {
			return m[1], nil
		}
	}

	entries, err := p.episodeEntries(ctx, episode.Anime.ShowID)
	if err != nil {
		return "", err
	}

	want := strconv.FormatFloat(episode.Number, 'f', -1, 64)
	for _, e := range entries {
		if e.Number == want {
			return e.DataID, nil
		}
	}
	// Fall back to the first episode if the exact number is absent.
	if len(entries) > 0 {
		return entries[0].DataID, nil
	}
	return "", fmt.Errorf("episode %s not found on hianime", want)
}

// embedReferer returns the embed site root, which the stream host expects as
// Referer (e.g. "https://zokoanime.video/").
func embedReferer(embedURL string) string {
	if i := strings.Index(embedURL, "://"); i >= 0 {
		rest := embedURL[i+3:]
		if end := strings.Index(rest, "/"); end >= 0 {
			return embedURL[:i+3+end+1]
		}
	}
	return embedURL
}

func sortStreamsByQuality(streams []*source.Stream) {
	sort.SliceStable(streams, func(i, j int) bool {
		return qualityNum(streams[i].Quality) > qualityNum(streams[j].Quality)
	})
}

func qualityNum(q string) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(q, "p"))
	return n
}
