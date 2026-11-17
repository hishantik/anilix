package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/source"
)

// keyRune builds a printable key press.
func keyRune(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// applyKey applies a message to the model and returns it.
func applyKey(t *testing.T, m *SearchModel, msg tea.Msg) *SearchModel {
	t.Helper()
	updated, _ := m.Update(msg)
	model, ok := updated.(*SearchModel)
	if !ok {
		t.Fatalf("Update returned %T, want *SearchModel", updated)
	}
	return model
}

// TestNavigationDestinationsAreKnown checks the content of the drawer.
//
// The drawer has to cover the destinations of the design, and it has to
// keep the order they are listed in.
func TestNavigationDestinationsAreKnown(t *testing.T) {
	want := []string{"Discover", "Search", "History", "Watchlist", "Episodes", "AniList", "Settings"}

	if len(navDestinations) != len(want) {
		t.Fatalf("the drawer has %d destinations, want %d", len(navDestinations), len(want))
	}
	for i, label := range want {
		if navDestinations[i].label != label {
			t.Errorf("destination %d is %q, want %q", i, navDestinations[i].label, label)
		}
	}
}

// TestUnsupportedDestinationsAreNotSelectable checks that the destinations
// the application cannot open stay in the drawer but cannot be reached.
func TestUnsupportedDestinationsAreNotSelectable(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)

	idx := navIndexOf(navDiscover)
	if idx < 0 {
		t.Fatalf("destination %v is missing from the drawer", navDiscover)
	}
	m.nav.cursor = idx
	if m.navAvailable(navDiscover) {
		t.Error("Discover must not be available")
	}
	m.activateNav()
	if m.state != searchState {
		t.Errorf("selecting %v changed the screen to %v", navDiscover, m.state)
	}
}

// TestTheWatchlistDestinationIsReachable checks that the watchlist is a
// destination of its own: it can be selected from the drawer, even while
// the account is not connected - the screen carries its own empty state.
func TestTheWatchlistDestinationIsReachable(t *testing.T) {
	tempHome(t)
	config.Set("anilist.token", "")

	m := NewSearchModel()
	m.resize(100, 40)

	m.nav.cursor = navIndexOf(navWatchlist)
	if !m.navAvailable(navWatchlist) {
		t.Fatal("the watchlist is not available")
	}

	m.activateNav()
	if m.state != watchlistState {
		t.Fatalf("selecting the watchlist opened state %d, want the watchlist screen", m.state)
	}
	if m.watchlist.Loading {
		t.Error("the watchlist started a fetch without a token")
	}
}

// TestDrawerCursorSkipsUnavailableDestinations checks that the cursor never
// lands on a destination that cannot be opened.
func TestDrawerCursorSkipsUnavailableDestinations(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)

	// The cursor starts on Search. Discover is not reachable, and
	// Episodes needs a selection, so the cursor has to jump over them.
	m.nav.cursor = navIndexOf(navSearch)

	m.moveNavCursor(1)
	if got := navDestinations[m.nav.cursor].id; got != navHistory {
		t.Errorf("the cursor landed on %v, want History", got)
	}

	m.moveNavCursor(1)
	if got := navDestinations[m.nav.cursor].id; got != navWatchlist {
		t.Errorf("the cursor landed on %v, want Watchlist", got)
	}

	m.moveNavCursor(1)
	if got := navDestinations[m.nav.cursor].id; got != navAniList {
		t.Errorf("the cursor landed on %v, want AniList (Episodes is unavailable)", got)
	}

	m.moveNavCursor(-1)
	if got := navDestinations[m.nav.cursor].id; got != navWatchlist {
		t.Errorf("the cursor landed on %v, want Watchlist", got)
	}

	m.moveNavCursor(-2)
	if got := navDestinations[m.nav.cursor].id; got != navSearch {
		t.Errorf("the cursor landed on %v, want Search (Discover is unavailable)", got)
	}
}

// TestEpisodesDestinationNeedsASelection checks that the episode list is
// only reachable once an anime is selected.
func TestEpisodesDestinationNeedsASelection(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)

	if m.navAvailable(navEpisodes) {
		t.Error("Episodes must not be available without a selection")
	}

	m.searchState.Results = []*source.Anime{{Name: "One Piece", ShowID: "12345"}}
	if !m.navAvailable(navEpisodes) {
		t.Error("Episodes must be available once an anime is selected")
	}
}

// TestActiveDestinationFollowsTheScreen checks the active page indication.
func TestActiveDestinationFollowsTheScreen(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)

	if got := m.activeNav(); got != navSearch {
		t.Errorf("the active destination is %v, want Search", got)
	}

	m.state = historyState
	if got := m.activeNav(); got != navHistory {
		t.Errorf("the active destination is %v, want History", got)
	}

	m.state = detailState
	if got := m.activeNav(); got != navEpisodes {
		t.Errorf("the active destination is %v, want Episodes", got)
	}

	m.state = settingsState
	m.settingsState.Cursor = 0
	if got := m.activeNav(); got != navSettings {
		t.Errorf("the active destination is %v, want Settings", got)
	}

	m.settingsState.Cursor = settingsRowAniList
	if got := m.activeNav(); got != navAniList {
		t.Errorf("the active destination is %v, want AniList", got)
	}
}

// TestActivatingDestinationsOpensTheMatchingScreen checks that every
// destination of the drawer leads to a screen that exists.
func TestActivatingDestinationsOpensTheMatchingScreen(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)

	m.nav.cursor = navIndexOf(navSearch)
	m.activateNav()
	if m.state != searchState {
		t.Errorf("Search opened %v", m.state)
	}

	m.nav.cursor = navIndexOf(navHistory)
	m.activateNav()
	if m.state != historyState {
		t.Errorf("History opened %v", m.state)
	}

	m.nav.cursor = navIndexOf(navSettings)
	m.activateNav()
	if m.state != settingsState {
		t.Errorf("Settings opened %v", m.state)
	}

	m.nav.cursor = navIndexOf(navAniList)
	m.activateNav()
	if m.state != anilistState {
		t.Errorf("AniList opened %v", m.state)
	}
	if m.prevState != settingsState {
		t.Errorf("AniList lost the screen it came from: %v", m.prevState)
	}
}

// TestEpisodesDestinationOpensTheEpisodeList checks the episode destination.
func TestEpisodesDestinationOpensTheEpisodeList(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)
	m.searchState.Results = []*source.Anime{{Name: "One Piece", ShowID: "12345"}}

	m.nav.cursor = navIndexOf(navEpisodes)
	m.activateNav()

	if m.state != detailState {
		t.Fatalf("Episodes opened %v", m.state)
	}
	if m.episodeState.AnimeID != "12345" {
		t.Errorf("the episode list is for %q, want the selected anime", m.episodeState.AnimeID)
	}
}

// TestHistoryScreenRunsTheSelectedQuery checks the history destination.
func TestHistoryScreenRunsTheSelectedQuery(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 40)
	m.recentSearches = []string{"one piece", "frieren"}

	m.state = historyState
	m.recentCursor = 1

	model := applyKey(t, m, keyRune('j'))
	if model.recentCursor != 1 {
		t.Errorf("the cursor is at %d, want 1", model.recentCursor)
	}

	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.state != searchState {
		t.Errorf("esc did not leave the history screen, state is %v", model.state)
	}
}
