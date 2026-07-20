package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
