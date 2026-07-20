package miruro

import (
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

func TestProviderRejectsAnimeWithoutAniListID(t *testing.T) {
	provider := NewProvider(NewClient(&routeTransport{}), nil)
	if _, err := provider.EpisodesOf(&source.Anime{Name: "Unknown"}, 1); err == nil {
		t.Fatal("EpisodesOf succeeded without AniList ID")
	}
}
