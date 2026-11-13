package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/source"
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
	for _, want := range []string{"pirate-king", "Log out", "Tracking"} {
		if !strings.Contains(view, want) {
			t.Errorf("the connected AniList screen lost %q:\n%s", want, view)
		}
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

// TestTheSettingsSheetIsGrouped checks that the settings sheet carries
// the groups the design calls for, with the values of the real keys.
func TestTheSettingsSheetIsGrouped(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(80, 30)

	view := shellStrip(m.viewSettings())
	for _, want := range []string{
		"Playback", "Quality", "AniSkip",
		"Audio", "Sub/Dub",
		"Appearance", "Sidebar",
		"Images", "Renderer",
		"Player",
		"AniList", "Account",
		"Application", "Update",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the settings sheet lost %q:\n%s", want, view)
		}
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
