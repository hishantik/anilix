package miruro

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/source"
)

type animeSearcher interface {
	Search(query string) ([]*source.Anime, error)
}

type Provider struct {
	client   *Client
	searcher animeSearcher

	mu                 sync.RWMutex
	translation        string
	successfulProvider string
}

type StreamCandidate struct {
	Provider string
	Resolve  func() ([]*source.Stream, error)
}

func NewProvider(client *Client, searcher animeSearcher) *Provider {
	if searcher == nil {
		searcher = anilist.NewAniListProvider()
	}
	return &Provider{client: client, searcher: searcher, translation: "sub"}
}

func (p *Provider) Name() string { return "Miruro" }
func (p *Provider) ID() string   { return "miruro" }

func (p *Provider) Search(query string) ([]*source.Anime, error) {
	return p.searcher.Search(query)
}

func (p *Provider) SeasonsOf(*source.Anime) ([]source.Season, error) {
	return []source.Season{{Number: 1, Name: "Season 1"}}, nil
}

func (p *Provider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	if anime == nil || anime.AniListID <= 0 {
		return nil, fmt.Errorf("anime has no AniList ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	catalog, err := p.client.Catalog(ctx, anime.AniListID)
	if err != nil {
		return nil, err
	}
	p.mu.RLock()
	translation := p.translation
	p.mu.RUnlock()
	entries := catalog.entries[translation]
	if len(entries) == 0 {
		return nil, fmt.Errorf("Miruro has no %s episodes for %s", translation, anime.Name)
	}
	numbers := make([]string, 0, len(entries))
	for number := range entries {
		numbers = append(numbers, number)
	}
	sort.Slice(numbers, func(i, j int) bool {
		left, _ := strconv.ParseFloat(numbers[i], 64)
		right, _ := strconv.ParseFloat(numbers[j], 64)
		return left < right
	})
	episodes := make([]*source.Episode, 0, len(numbers))
	for _, number := range numbers {
		value, err := strconv.ParseFloat(number, 64)
		if err != nil {
			continue
		}
		title := ""
		if candidates := entries[number]; len(candidates) > 0 {
			title = candidates[0].Title
		}
		episodes = append(episodes, &source.Episode{Number: value, Title: title, Anime: anime, Season: season})
	}
	return episodes, nil
}

func (p *Provider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	candidates, err := p.CandidateStreams(episode)
	if err != nil {
		return nil, err
	}
	var failures []error
	var resolved []*source.Stream
	for _, candidate := range candidates {
		streams, err := candidate.Resolve()
		if err == nil && len(streams) > 0 {
			resolved = append(resolved, streams...)
			continue
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	if len(resolved) > 0 {
		return resolved, nil
	}
	return nil, fmt.Errorf("no playable Miruro source: %v", failures)
}

func (p *Provider) CandidateStreams(episode *source.Episode) ([]StreamCandidate, error) {
	if episode == nil || episode.Anime == nil || episode.Anime.AniListID <= 0 {
		return nil, fmt.Errorf("episode has no anime or AniList ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	catalog, err := p.client.Catalog(ctx, episode.Anime.AniListID)
	if err != nil {
		return nil, err
	}
	p.mu.RLock()
	translation := p.translation
	p.mu.RUnlock()
	number := strconv.FormatFloat(episode.Number, 'f', -1, 64)
	candidates := catalog.Candidates(translation, number)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("Miruro has no %s source candidates for episode %s", translation, number)
	}
	ordered := orderEpisodeCandidates(candidates, p.PreferredProvider())
	result := make([]StreamCandidate, 0, len(ordered))
	for _, candidate := range ordered {
		candidate := candidate
		result = append(result, StreamCandidate{
			Provider: candidate.Provider,
			Resolve: func() ([]*source.Stream, error) {
				resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer resolveCancel()
				return p.client.Sources(resolveCtx, candidate)
			},
		})
	}
	return result, nil
}

func orderEpisodeCandidates(candidates []EpisodeCandidate, preferred string) []EpisodeCandidate {
	byProvider := make(map[string]EpisodeCandidate, len(candidates))
	for _, candidate := range candidates {
		if _, exists := byProvider[candidate.Provider]; !exists {
			byProvider[candidate.Provider] = candidate
		}
	}
	ordered := make([]EpisodeCandidate, 0, len(byProvider))
	for _, name := range []string{preferred, "ally"} {
		if candidate, ok := byProvider[name]; ok {
			ordered = append(ordered, candidate)
			delete(byProvider, name)
		}
	}
	for _, candidate := range candidates {
		if remaining, ok := byProvider[candidate.Provider]; ok {
			ordered = append(ordered, remaining)
			delete(byProvider, candidate.Provider)
		}
	}
	return ordered
}

func (p *Provider) MarkSuccessfulProvider(name string) {
	p.mu.Lock()
	p.successfulProvider = name
	p.mu.Unlock()
}

func (p *Provider) PreferredProvider() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.successfulProvider
}

func (p *Provider) SetTranslation(translation string) {
	if translation != "sub" && translation != "dub" {
		return
	}
	p.mu.Lock()
	p.translation = translation
	p.mu.Unlock()
}

func (p *Provider) TranslationType() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.translation
}

func (p *Provider) Close() error { return p.client.Close() }

var _ source.Source = (*Provider)(nil)
