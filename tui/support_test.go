package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/version"
)

// tempHome isolates the stores the supporting screens read and write.
//
// The tests do not run config.Setup, so the defaults the production code
// gets from the config file are set here where a screen reads them.
func tempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", "") // windows equivalent, ignored here
	config.Set("history.enabled", true)
	config.Set("artwork.renderer", "auto")
	config.Set("translation_type", "sub")
}

// TestRecordWatchKeepsTheRecentEpisodes checks the store: newest first,
// a replay moves an entry to the top instead of duplicating it, and the
// store stays bounded.
func TestRecordWatchKeepsTheRecentEpisodes(t *testing.T) {
	tempHome(t)

	recordWatch("Frieren", "21", "1")
	time.Sleep(2 * time.Millisecond)
	recordWatch("One Piece", "21x", "1100")
	time.Sleep(2 * time.Millisecond)
	recordWatch("Frieren", "21", "1") // replay: moves, does not duplicate

	entries := loadWatchHistory()
	if len(entries) != 2 {
		t.Fatalf("the history holds %d entries, want 2:\n%v", len(entries), entries)
	}
	if entries[0].Name != "Frieren" || entries[0].Episode != "1" {
		t.Errorf("the replay did not move to the top: %+v", entries[0])
	}
	if entries[1].Name != "One Piece" {
		t.Errorf("the other entry was lost: %+v", entries[1])
	}

	// The store is bounded: the oldest entry goes.
	for i := 0; i < watchHistoryLimit+5; i++ {
		recordWatch("Frieren", "21", itoaTest(i))
	}
	if got := len(loadWatchHistory()); got > watchHistoryLimit {
		t.Errorf("the history grew to %d entries", got)
	}
}

// itoaTest renders a small number for the test above.
func itoaTest(i int) string {
	return string(rune('0'+i%10)) + string(rune('0'+i/10))
}

// TestRecordWatchHonoursTheSwitch checks that history.enabled turned off
// stops the writes: the store stays empty.
func TestRecordWatchHonoursTheSwitch(t *testing.T) {
	tempHome(t)
	config.Set("history.enabled", false)
	defer config.Set("history.enabled", true)

	recordWatch("Frieren", "21", "1")
	if got := loadWatchHistory(); len(got) != 0 {
		t.Errorf("the disabled history still wrote %v", got)
	}
}

// TestTheHistoryScreenShowsTheEntries checks the content of the history
// screen and its empty state.
func TestTheHistoryScreenShowsTheEntries(t *testing.T) {
	tempHome(t)
	recordWatch("Frieren", "21", "3")

	m := NewSearchModel()
	m.resize(80, 30)
	m.state = historyState
	m.watchHistoryCache = loadWatchHistory()

	view := shellStrip(m.viewHistoryState())
	for _, want := range []string{"watch history", "frieren", "episode 3"} {
		if !strings.Contains(strings.ToLower(view), want) {
			t.Errorf("the history screen lost %q:\n%s", want, view)
		}
	}

	// An empty history says so instead of going blank.
	m.watchHistoryCache = nil
	if view := shellStrip(m.viewHistoryState()); !strings.Contains(view, "Nothing watched yet") {
		t.Errorf("the empty history screen lost its state:\n%s", view)
	}
}

// TestHistoryEnterResumesTheEntry checks that enter on a history row
// searches for the entry and remembers its streaming id, so the results
// land on the watched anime.
func TestHistoryEnterResumesTheEntry(t *testing.T) {
	tempHome(t)
	recordWatch("Frieren", "21", "3")

	m := NewSearchModel()
	m.resize(80, 30)
	m.state = historyState
	m.watchHistoryCache = loadWatchHistory()
	m.textInput.SetValue("")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	resumed := updated.(*SearchModel)

	if resumed.state != searchState {
		t.Errorf("enter did not open the search: state %d", resumed.state)
	}
	if resumed.pendingShowID != "21" {
		t.Errorf("enter lost the streaming id of the entry: %q", resumed.pendingShowID)
	}
	if resumed.textInput.Value() != "Frieren" {
		t.Errorf("enter lost the query: %q", resumed.textInput.Value())
	}
}

// TestTheResultsFollowTheResume checks that a search that came back from
// a resume selects the result with the remembered id.
func TestTheResultsFollowTheResume(t *testing.T) {
	m := modelWithResults(80, 30)
	m.pendingShowID = "wanted-id"
	m.searchState.Results = []*source.Anime{
		{Name: "Other", ShowID: "other-id"},
		{Name: "Wanted", ShowID: "wanted-id"},
	}
	m.metadataCache = map[int]*MetadataPanel{}

	m.Update(SearchResultsMsg{Results: m.searchState.Results})

	if m.pendingShowID != "" {
		t.Error("the resume id survived the results")
	}
	if m.searchState.Selected != 1 {
		t.Errorf("the resume selected index %d, want 1", m.searchState.Selected)
	}
}

// TestTheAniListScreenShowsTheAccount checks the AniList screen: the
// connection state, the tracking and the action it offers.
func TestTheAniListScreenShowsTheAccount(t *testing.T) {
	tempHome(t)
	config.Set("anilist.token", "")
	config.Set("anilist.username", "")

	m := NewSearchModel()
	m.resize(80, 30)
	m.state = anilistState

	view := shellStrip(m.viewAniListState())
	for _, want := range []string{"account", "not connected", "log in", "synchronisation"} {
		if !strings.Contains(strings.ToLower(view), want) {
			t.Errorf("the AniList screen lost %q:\n%s", want, view)
		}
	}

	config.Set("anilist.token", "tok")
	config.Set("anilist.username", "pirate-king")
	defer func() {
		config.Set("anilist.token", "")
		config.Set("anilist.username", "")
	}()
	m.trackingEnabled = true

	view = shellStrip(m.viewAniListState())
	for _, want := range []string{"pirate-king", "Log out", "Tracking", "Connected"} {
		if !strings.Contains(view, want) {
			t.Errorf("the connected AniList screen lost %q:\n%s", want, view)
		}
	}
	// The header of the account section states the status in words, not
	// only in colour.
	if !strings.Contains(view, "Username") {
		t.Error("the connected AniList screen lost the account information")
	}
}

// TestLogoutIsConfirmed checks the destructive action: enter on log out
// asks first, and only a confirmed answer drops the token.
func TestLogoutIsConfirmed(t *testing.T) {
	tempHome(t)
	defer config.Set("anilist.token", "")
	config.Set("anilist.token", "tok")

	m := NewSearchModel()
	m.resize(80, 30)
	m.state = anilistState
	m.anilistToken = "tok"
	m.anilistCursor = 1

	// Enter opens the confirmation instead of logging out.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(*SearchModel).state != confirmLogoutState {
		t.Fatal("log out did not ask for a confirmation")
	}

	// "No" keeps the token.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if updated.(*SearchModel).state != anilistState {
		t.Fatal("escape did not return to the AniList screen")
	}
	if updated.(*SearchModel).anilistToken != "tok" {
		t.Error("escape dropped the token")
	}

	// A confirmed answer logs out: the dialog opens with "No" focused,
	// the cursor moves to "Yes", and "y" confirms.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // opens the dialog again
	updated, _ = updated.Update(keyRune('l'))                        // focus moves to "Yes"
	updated, _ = updated.Update(keyRune('y'))
	if updated.(*SearchModel).state != anilistState {
		t.Fatal("the confirmed logout did not return to the AniList screen")
	}
	if updated.(*SearchModel).anilistToken != "" {
		t.Error("the confirmed logout kept the token")
	}
	if config.GetString("anilist.token") != "" {
		t.Error("the confirmed logout kept the stored token")
	}
}

// TestTheSettingsSheetIsGrouped checks that the settings screen carries
// the groups the design calls for, with the values of the real keys.
func TestTheSettingsSheetIsGrouped(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(80, 30)

	view := shellStrip(m.viewSettings())
	for _, want := range []string{
		"Playback", "Quality", "AniSkip",
		"Audio & Subtitles", "Sub/Dub",
		"Appearance", "Sidebar",
		"Images", "Renderer",
		"Player",
		"AniList", "Account",
		"Application", "Version", "Update",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the settings screen lost %q:\n%s", want, view)
		}
	}
}

// TestTheVersionRowIsReadOnly checks that the version row shows the
// version of the running application and accepts no changes.
func TestTheVersionRowIsReadOnly(t *testing.T) {
	tempHome(t)
	config.Set("quality", "auto")
	defer config.Set("quality", "auto")
	m := NewSearchModel()
	m.resize(80, 30)
	m.state = settingsState
	m.settingsState.Cursor = settingsRowVersion

	view := shellStrip(m.viewSettings())
	if !strings.Contains(view, version.Version) {
		t.Errorf("the version row does not name the version %q:\n%s", version.Version, view)
	}

	m.settingsAdjust(true)
	m.settingsAdjust(false)
	if got := config.GetString("quality"); got != "auto" {
		t.Errorf("the version row changed the wrong setting: quality is %q", got)
	}
}

// TestTheWatchlistScreenStates checks the three phases of the watchlist:
// loading, error and content, plus the states of an account that is not
// connected or has nothing saved.
func TestTheWatchlistScreenStates(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(80, 30)
	m.state = watchlistState
	m.watchlist = &WatchlistState{}

	// Not connected: the empty state says so, without a spinner.
	m.watchlist.Loading = false
	view := shellStrip(m.viewWatchlistState())
	if !strings.Contains(strings.ToLower(view), "not connected") {
		t.Errorf("a disconnected watchlist lost its state:\n%s", view)
	}

	// Loading.
	m.watchlist.Loading = true
	view = shellStrip(m.viewWatchlistState())
	if !strings.Contains(strings.ToLower(view), "loading") {
		t.Errorf("a loading watchlist lost its state:\n%s", view)
	}

	// Error.
	m.watchlist.Loading = false
	m.watchlist.Err = errors.New("the list could not be reached")
	view = shellStrip(m.viewWatchlistState())
	if !strings.Contains(strings.ToLower(view), "could not load") {
		t.Errorf("a failed watchlist lost its state:\n%s", view)
	}

	// Empty and connected: nothing is saved yet.
	config.Set("anilist.token", "tok")
	defer config.Set("anilist.token", "")
	m.watchlist.Err = nil
	view = shellStrip(m.viewWatchlistState())
	if !strings.Contains(view, "Nothing saved yet") {
		t.Errorf("an empty watchlist lost its state:\n%s", view)
	}
}

// TestTheWatchlistListsTheEntries checks the content of the watchlist:
// the title, the status with the progress, and the selection state.
func TestTheWatchlistListsTheEntries(t *testing.T) {
	tempHome(t)
	config.Set("anilist.token", "tok")
	defer config.Set("anilist.token", "")
	m := NewSearchModel()
	m.resize(100, 30)
	m.state = watchlistState
	m.watchlist = &WatchlistState{
		Entries: []*anilist.WatchlistEntry{
			{AniListID: 21, Title: "Frieren", Status: "CURRENT", Progress: 5, Total: 28, Year: 2023, Score: 9.0, Format: "TV"},
			{AniListID: 1, Title: "One Piece", Status: "PLANNING", Progress: 0, Year: 1999},
		},
	}

	view := shellStrip(m.viewWatchlistState())
	for _, want := range []string{"watchlist", "frieren", "watching", "ep 5/28", "one piece", "planned", "tv · 2023"} {
		if !strings.Contains(strings.ToLower(view), want) {
			t.Errorf("the watchlist lost %q:\n%s", want, view)
		}
	}

	// The counter names where the cursor is.
	if !strings.Contains(view, "1 of 2") {
		t.Errorf("the watchlist lost its counter:\n%s", view)
	}
}

// TestWatchlistKeysWalkTheList tests the keys of the watchlist: the
// cursor walks the entries, escape returns, and enter searches the title.
func TestWatchlistKeysWalkTheList(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(80, 30)
	m.state = watchlistState
	m.watchlist = &WatchlistState{
		Entries: []*anilist.WatchlistEntry{
			{AniListID: 21, Title: "Frieren", Status: "CURRENT"},
			{AniListID: 1, Title: "One Piece", Status: "PLANNING"},
		},
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.watchlist.Cursor != 1 {
		t.Fatalf("the cursor did not move: %d", m.watchlist.Cursor)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != searchState {
		t.Fatalf("escape did not return to the search: state %d", m.state)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyUp}) // results take the keyboard
	m.textInput.Blur()
	m.state = watchlistState
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.state != searchState {
		t.Fatalf("enter did not open the search: state %d", m.state)
	}
	if m.textInput.Value() != "One Piece" {
		t.Errorf("enter searched %q, want the title of the selected entry", m.textInput.Value())
	}
	if m.textInput.Focused() {
		t.Error("enter left the search field focused")
	}
}

// TestTheHistoryScreenGroupsTheEntries checks that the history groups its
// entries under the day they were played on.
func TestTheHistoryScreenGroupsTheEntries(t *testing.T) {
	tempHome(t)

	m := NewSearchModel()
	m.resize(80, 30)
	m.state = historyState
	now := time.Now()
	m.watchHistoryCache = []historyEntry{
		{Name: "Frieren", ShowID: "21", Episode: "3", At: now.Add(-2 * time.Hour)},
		{Name: "One Piece", ShowID: "21x", Episode: "1100", At: now.Add(-30 * time.Hour)},
		{Name: "Dandadan", ShowID: "99", Episode: "1", At: now.Add(-48 * time.Hour)},
	}

	view := shellStrip(m.viewHistoryState())
	for _, want := range []string{"watch history", "frieren", "episode 3", "today", "yesterday"} {
		if !strings.Contains(strings.ToLower(view), want) {
			t.Errorf("the history screen lost %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "Nov") {
		t.Errorf("the older entries are not grouped under their date:\n%s", view)
	}

	// An empty history still says so instead of going blank.
	m.watchHistoryCache = nil
	if view := shellStrip(m.viewHistoryState()); !strings.Contains(view, "Nothing watched yet") {
		t.Errorf("the empty history screen lost its state:\n%s", view)
	}
}

// TestTheSettingsCursorStaysInRange checks the bounds of the cursor over
// the grouped rows.
func TestTheSettingsCursorStaysInRange(t *testing.T) {
	m := NewSearchModel()
	m.resize(80, 30)
	m.state = settingsState
	m.settingsState.Cursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if updated.(*SearchModel).settingsState.Cursor != 0 {
		t.Error("the cursor left the sheet at the top")
	}

	for i := 0; i < settingsRowCount+3; i++ {
		updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := updated.(*SearchModel).settingsState.Cursor; got != settingsRowCount-1 {
		t.Errorf("the cursor ended on row %d, want %d", got, settingsRowCount-1)
	}
}

// TestTheSubDubSettingIsWired checks that the sub/dub row changes the
// real keys: the configuration, the search state and the provider.
func TestTheSubDubSettingIsWired(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(80, 30)
	m.state = settingsState
	m.settingsState.Cursor = settingsRowSubDub
	defer config.Set("translation_type", "sub")

	m.settingsAdjust(false)

	if got := config.GetString("translation_type"); got != "dub" {
		t.Errorf("the configuration kept %q, want dub", got)
	}
	if m.searchState.TranslationType != "dub" {
		t.Errorf("the search state kept %q, want dub", m.searchState.TranslationType)
	}
	if m.hianimeProvider == nil {
		t.Fatal("the provider is missing")
	}
}

// TestTheRendererSettingIsWired checks that the renderer row writes the
// real key and cycles its choices.
func TestTheRendererSettingIsWired(t *testing.T) {
	tempHome(t)
	defer config.Set("artwork.renderer", "auto")
	config.Set("artwork.renderer", "auto")

	m := NewSearchModel()
	m.state = settingsState
	m.settingsState.Cursor = settingsRowRenderer

	m.settingsAdjust(true)
	if got := rendererChoice(); got != "ascii" {
		t.Errorf("the choice wrapped to %q, want ascii", got)
	}
	m.settingsAdjust(false)
	if got := rendererChoice(); got != "auto" {
		t.Errorf("the choice wrapped to %q, want auto", got)
	}
}

// TestTheSidebarSettingIsWired checks that the sidebar row flips the
// drawer and remembers it.
func TestTheSidebarSettingIsWired(t *testing.T) {
	tempHome(t)
	defer config.Set("ui.sidebar", "expanded")
	m := NewSearchModel()
	m.state = settingsState
	m.settingsState.Cursor = settingsRowSidebar
	m.nav.expanded = true

	m.settingsAdjust(false)

	if m.nav.expanded {
		t.Error("the sidebar stayed expanded")
	}
	if got := config.GetString("ui.sidebar"); got != "collapsed" {
		t.Errorf("the configuration kept %q, want collapsed", got)
	}
}

// TestTheAniListRowOpensTheAccountScreen checks that enter on the
// account row of the settings opens the AniList screen.
func TestTheAniListRowOpensTheAccountScreen(t *testing.T) {
	m := NewSearchModel()
	m.state = settingsState
	m.settingsState.Cursor = settingsRowAniList

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(*SearchModel).state != anilistState {
		t.Errorf("enter opened state %d, want the AniList screen", updated.(*SearchModel).state)
	}
}
