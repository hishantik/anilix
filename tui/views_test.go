package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/tui/theme"
)

// TestScreensNeverExceedTheTerminal checks that no screen of the
// application renders wider than the terminal, at every width.
func TestScreensNeverExceedTheTerminal(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120, 200} {
		m := modelWithResults(width, 30)

		screens := map[string]string{
			"search":   m.viewSearchState(),
			"detail":   m.viewDetailState(),
			"history":  m.viewHistoryState(),
			"settings": m.viewSettings(),
			"quit":     m.viewConfirmQuit(),
		}

		for name, content := range screens {
			view := shellStrip(m.renderShell(content))
			for i, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Errorf("%s at width %d: row %d is %d cells: %q", name, width, i, got, line)
				}
			}
		}
	}
}

// TestScreensNeverExceedTheTerminalHeight checks that the shell never grows
// beyond the terminal: a screen that overflows pushes the footer off the
// screen.
func TestScreensNeverExceedTheTerminalHeight(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{40, 24}, {80, 24}, {120, 40}, {200, 50},
	} {
		m := modelWithResults(size.width, size.height)

		screens := map[string]string{
			"search":   m.viewSearchState(),
			"detail":   m.viewDetailState(),
			"history":  m.viewHistoryState(),
			"settings": m.viewSettings(),
			"quit":     m.viewConfirmQuit(),
		}

		for name, content := range screens {
			view := m.renderShell(content)
			if rows := strings.Count(view, "\n") + 1; rows > size.height {
				t.Errorf("%s at %dx%d: %d rows, want at most %d", name, size.width, size.height, rows, size.height)
			}
		}
	}
}

// TestSearchScreenShowsTheResults checks that the result list and the
// metadata panel are part of the search screen.
func TestSearchScreenShowsTheResults(t *testing.T) {
	m := modelWithResults(120, 30)

	view := shellStrip(m.viewSearchState())

	for _, want := range []string{"One Piece", "Frieren", "TV · 1999 · 1180 eps", "Finished Airing"} {
		if !strings.Contains(view, want) {
			t.Errorf("the search screen lost %q:\n%s", want, view)
		}
	}
}

// TestSearchScreenShowsNoResults checks that a query that came back empty
// renders the empty state instead of a blank screen.
func TestSearchScreenShowsNoResults(t *testing.T) {
	m := NewSearchModel()
	m.resize(80, 30)
	m.searchState.Loading = false
	m.searchState.Err = nil
	m.searchState.Results = nil
	m.textInput.SetValue("zzzzqqqq")
	m.textInput.Blur()

	view := shellStrip(m.viewSearchState())

	for _, want := range []string{"No results for zzzzqqqq", "Try a shorter title"} {
		if !strings.Contains(view, want) {
			t.Errorf("the empty search screen lost %q:\n%s", want, view)
		}
	}
}

// TestSearchScreenShowsTheField checks that the search field is rendered
// with its placeholder.
func TestSearchScreenShowsTheField(t *testing.T) {
	m := NewSearchModel()
	m.resize(80, 30)

	view := shellStrip(m.viewSearchState())
	if !strings.Contains(view, "Search anime...") {
		t.Errorf("the search field lost its placeholder:\n%s", view)
	}
}

// TestSettingsDialogIsClamped checks that the settings sheet fits on a
// phone screen.
func TestSettingsDialogIsClamped(t *testing.T) {
	m := NewSearchModel()
	m.resize(40, 30)

	view := shellStrip(m.viewSettings())
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Errorf("row %d is %d cells wide: %q", i, got, line)
		}
	}
	if !strings.Contains(view, "Settings") {
		t.Errorf("the settings sheet lost its title:\n%s", view)
	}
}

// TestEpisodeRowsUseTheDesignSystem checks that the episode list is built
// from the list component: the episode number is the title, the name of the
// episode is the description.
func TestEpisodeRowsUseTheDesignSystem(t *testing.T) {
	m := modelWithResults(120, 30)
	m.state = detailState
	m.episodeState.Episodes = []string{"1", "2"}
	m.episodeState.EpisodeTitles = []string{"The Endless Journey", ""}
	m.updateEpisodeList()

	view := shellStrip(m.viewDetailState())
	if !strings.Contains(view, "Episode 1") {
		t.Errorf("the episode list lost the episode number:\n%s", view)
	}
	if !strings.Contains(view, "The Endless Journey") {
		t.Errorf("the episode list lost the episode title:\n%s", view)
	}
}

// TestEpisodeRowsMarkWatchedEpisodes checks the tracking badges of the
// episode list: a watched episode carries the check, the first one after
// the progress carries the play glyph.
func TestEpisodeRowsMarkWatchedEpisodes(t *testing.T) {
	m := modelWithResults(120, 50)
	m.state = detailState
	m.trackingEnabled = true
	m.episodeState.TrackingStatus = "CURRENT"
	m.episodeState.TrackingProgress = 2
	m.episodeState.Episodes = []string{"1", "2", "3", "4"}
	m.updateEpisodeList()

	view := shellStrip(m.viewDetailState())

	if got := strings.Count(view, theme.Icon(theme.IconCheck)); got != 2 {
		t.Errorf("the list marked %d episodes as watched, want 2 (progress):\n%s", got, view)
	}
	if got := strings.Count(view, theme.Icon(theme.IconPlay)); got != 1 {
		t.Errorf("the list marked %d episodes as next up, want 1:\n%s", got, view)
	}
}

// TestEpisodeRowsStayUnmarkedWithoutTracking checks that a model without
// tracking invents no watched state: the rows carry no badges.
func TestEpisodeRowsStayUnmarkedWithoutTracking(t *testing.T) {
	m := modelWithResults(120, 30)
	m.state = detailState
	m.episodeState.Episodes = []string{"1", "2"}
	m.updateEpisodeList()

	for i, item := range m.episodeList.Items() {
		row := item.(episodeItem)
		if row.watched || row.nextUp {
			t.Errorf("episode %d got a tracking badge without any tracking", i+1)
		}
	}
}

// TestTheEpisodeSectionCarriesTheTranslation checks that the episode list
// says which translation its servers were fetched with.
func TestTheEpisodeSectionCarriesTheTranslation(t *testing.T) {
	m := modelWithResults(120, 30)
	m.state = detailState
	m.episodeState.Episodes = []string{"1"}
	m.updateEpisodeList()

	view := shellStrip(m.viewDetailState())
	if !strings.Contains(view, "SUB") {
		t.Errorf("the episode section lost the translation badge:\n%s", view)
	}
}

// TestTheDetailsScreenShowsTheAniListProgress checks that the details
// screen shows the tracking of the entry when it is known.
func TestTheDetailsScreenShowsTheAniListProgress(t *testing.T) {
	m := modelWithResults(120, 30)
	m.state = animeState
	m.trackingEnabled = true
	m.episodeState.TrackingStatus = "CURRENT"
	m.episodeState.TrackingProgress = 5

	view := shellStrip(m.viewAnimeState())
	if !strings.Contains(view, "Watching (Ep 5)") {
		t.Errorf("the details screen lost the AniList progress:\n%s", view)
	}
}

// TestTheDetailsScreenInventsNoAniListRow checks that a model without a
// tracking status shows no AniList fact: a status that is not known is not
// made up.
func TestTheDetailsScreenInventsNoAniListRow(t *testing.T) {
	m := modelWithResults(120, 30)
	m.state = animeState
	m.trackingEnabled = true

	if view := shellStrip(m.viewAnimeState()); strings.Contains(view, "Watching (Ep") {
		t.Errorf("the details screen fabricated an AniList row:\n%s", view)
	}
}

// TestListMutesWhenTheFieldHasTheFocus checks the focus rule of the design
// system: a list that lost the keyboard keeps its cursor but mutes the row.
func TestListMutesWhenTheFieldHasTheFocus(t *testing.T) {
	m := modelWithResults(120, 30)

	m.textInput.Focus()
	m.syncDelegates()
	if m.searchDelegate.focused {
		t.Error("the list kept the focus while the search field holds the keyboard")
	}

	m.textInput.Blur()
	m.syncDelegates()
	if !m.searchDelegate.focused {
		t.Error("the list did not take the focus back")
	}
}

// modelWithResults builds a model with a search result and its metadata.
func modelWithResults(width, height int) *SearchModel {
	m := NewSearchModel()
	m.searchState.Results = []*source.Anime{
		{Name: "One Piece", Type: "TV", Year: 1999, EpisodeCount: 1180, Score: 8.73},
		{Name: "Frieren: Beyond Journey's End", Type: "TV", Year: 2023, EpisodeCount: 28, Score: 9.03},
	}

	// Every result carries its own metadata: the alternative title and the
	// status are part of the row.
	m.metadataCache[0] = &MetadataPanel{
		Title:       "One Piece",
		TitleNative: "ワンピース",
		Type:        "TV",
		Status:      "Finished Airing",
		Episodes:    1180,
		Year:        1999,
		Score:       8.73,
		Rank:        1,
		Popularity:  1,
		Genres:      []string{"Action", "Adventure", "Fantasy"},
		Synopsis:    "Gol D. Roger was known as the Pirate King, the strongest and most infamous pirate.",
		Source:      "Jikan + AniList",
	}
	m.metadataCache[1] = &MetadataPanel{
		Title:       "Frieren: Beyond Journey's End",
		TitleNative: "葬送のフリーレン",
		Type:        "TV",
		Status:      "Finished Airing",
		Episodes:    28,
		Year:        2023,
		Score:       9.03,
		Rank:        4,
		Popularity:  3,
		Genres:      []string{"Adventure", "Drama", "Fantasy"},
		Synopsis:    "After the party of heroes defeated the Demon King, they restored peace to the land.",
		Source:      "Jikan + AniList",
	}
	m.searchState.Metadata = m.metadataCache[0]

	m.updateSearchList()
	m.resize(width, height)
	return m
}
