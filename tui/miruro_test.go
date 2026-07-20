package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hishantik/anilix/aniskip"
	"github.com/hishantik/anilix/provider/miruro"
	"github.com/hishantik/anilix/source"
)

func TestTryMiruroCandidatesResolvesLazilyUntilPlayerSucceeds(t *testing.T) {
	var resolved []string
	candidates := []miruro.StreamCandidate{
		{Provider: "bonk", Resolve: func() ([]*source.Stream, error) {
			resolved = append(resolved, "bonk")
			return []*source.Stream{{Provider: "bonk", URL: "https://bad.example/video.m3u8"}}, nil
		}},
		{Provider: "ally", Resolve: func() ([]*source.Stream, error) {
			resolved = append(resolved, "ally")
			return []*source.Stream{{Provider: "ally", URL: "https://good.example/video.m3u8"}}, nil
		}},
		{Provider: "kiwi", Resolve: func() ([]*source.Stream, error) {
			resolved = append(resolved, "kiwi")
			return []*source.Stream{{Provider: "kiwi", URL: "https://unused.example/video.m3u8"}}, nil
		}},
	}
	play := func(streams []*source.Stream, _, _ string, _ []aniskip.SkipInterval, _ string) *source.Stream {
		if streams[0].Provider == "ally" {
			return streams[0]
		}
		return nil
	}

	provider, failures := tryMiruroCandidates(candidates, "Naruto", "1", nil, "auto", play)
	if provider != "ally" || len(failures) != 1 {
		t.Fatalf("provider=%q failures=%v", provider, failures)
	}
	if !slices.Equal(resolved, []string{"bonk", "ally"}) {
		t.Fatalf("resolved = %v", resolved)
	}
}

func TestMiruroPlaybackErrorOffersBrowserFallback(t *testing.T) {
	model := NewSearchModel()
	defer model.close()
	model.state = detailState
	model.width = 100
	model.height = 40

	updated, _ := model.Update(TUIErrorMsg{
		Err:        errors.New("native resolution failed"),
		BrowserURL: "https://www.miruro.tv/watch/20/naruto",
	})
	got := updated.(*SearchModel)
	if got.episodeState.BrowserURL == "" {
		t.Fatal("browser fallback URL was not retained")
	}
	if !strings.Contains(strings.ToLower(got.View().Content), "press o") {
		t.Fatalf("detail view did not offer browser fallback: %s", got.View().Content)
	}
}

func TestOpenFallbackKeyUsesConfiguredBrowserOpener(t *testing.T) {
	model := NewSearchModel()
	defer model.close()
	model.state = detailState
	model.episodeState.BrowserURL = "https://www.miruro.tv/watch/20/naruto"
	var opened string
	model.openBrowser = func(rawURL string) error {
		opened = rawURL
		return nil
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	if opened != model.episodeState.BrowserURL {
		t.Fatalf("opened %q, want %q", opened, model.episodeState.BrowserURL)
	}
}
