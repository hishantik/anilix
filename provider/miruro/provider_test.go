package miruro

import (
	"slices"
	"testing"

	"github.com/hishantik/anilix/source"
)

func TestProviderEpisodesAndStreamsUseAniListIdentity(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"episodes":     []byte(`{"providers":{"bonk":{"episodes":{"sub":[{"id":"bonk-1","number":1},{"id":"bonk-1.5","number":1.5}]}},"kiwi":{"episodes":{"sub":[{"id":"kiwi-1","number":1}]}}}}`),
		"sources:bonk": []byte(`{"streams":[]}`),
		"sources:kiwi": []byte(`{"streams":[{"url":"https://video.example/episode-1.m3u8","quality":"720p"}]}`),
	}}
	provider := NewProvider(NewClient(transport), nil)
	provider.SetTranslation("sub")
	anime := &source.Anime{AniListID: 20, Name: "Naruto"}

	episodes, err := provider.EpisodesOf(anime, 1)
	if err != nil {
		t.Fatalf("EpisodesOf: %v", err)
	}
	if len(episodes) != 2 || episodes[0].String() != "1" || episodes[1].String() != "1.5" {
		t.Fatalf("episodes = %+v", episodes)
	}

	streams, err := provider.StreamsOf(episodes[0])
	if err != nil {
		t.Fatalf("StreamsOf: %v", err)
	}
	if len(streams) != 1 || streams[0].Provider != "kiwi" {
		t.Fatalf("streams = %+v", streams)
	}
}

func TestCandidateStreamsAreLazyAndPreferSuccessfulProviderThenAlly(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"episodes": []byte(`{"providers":{"bonk":{"episodes":{"sub":[{"id":"bonk-1","number":1}]}},"ally":{"episodes":{"sub":[{"id":"ally-1","number":1}]}},"kiwi":{"episodes":{"sub":[{"id":"kiwi-1","number":1}]}}}}`),
	}}
	provider := NewProvider(NewClient(transport), nil)
	provider.MarkSuccessfulProvider("kiwi")
	candidates, err := provider.CandidateStreams(&source.Episode{Number: 1, Anime: &source.Anime{AniListID: 20}})
	if err != nil {
		t.Fatalf("CandidateStreams: %v", err)
	}
	var names []string
	for _, candidate := range candidates {
		names = append(names, candidate.Provider)
	}
	if !slices.Equal(names, []string{"kiwi", "ally", "bonk"}) {
		t.Fatalf("candidate order = %v", names)
	}
	if slices.Contains(transport.calls, "sources:kiwi") || slices.Contains(transport.calls, "sources:ally") {
		t.Fatalf("constructing candidates resolved sources: %v", transport.calls)
	}
}

func TestProviderRejectsAnimeWithoutAniListID(t *testing.T) {
	provider := NewProvider(NewClient(&routeTransport{}), nil)
	if _, err := provider.EpisodesOf(&source.Anime{Name: "Unknown"}, 1); err == nil {
		t.Fatal("EpisodesOf succeeded without AniList ID")
	}
}

func TestProviderReturnsStreamsFromEveryCandidateForPlayerFallback(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"episodes":     []byte(`{"providers":{"bonk":{"episodes":{"sub":[{"id":"bonk-1","number":1}]}},"ally":{"episodes":{"sub":[{"id":"ally-1","number":1}]}}}}`),
		"sources:bonk": []byte(`{"streams":[{"url":"https://custom.example/episode.m3u8"}]}`),
		"sources:ally": []byte(`{"streams":[{"url":"https://native.example/episode.m3u8"}]}`),
	}}
	provider := NewProvider(NewClient(transport), nil)
	streams, err := provider.StreamsOf(&source.Episode{Number: 1, Anime: &source.Anime{AniListID: 20}})
	if err != nil {
		t.Fatalf("StreamsOf: %v", err)
	}
	if len(streams) != 2 || streams[0].Provider != "ally" || streams[1].Provider != "bonk" {
		t.Fatalf("streams = %+v, want ally then bonk", streams)
	}
}
