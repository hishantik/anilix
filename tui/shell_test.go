package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/ui"
)

var shellAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// shellStrip removes the escape sequences so the visible text can be
// measured.
func shellStrip(s string) string {
	return shellAnsiRe.ReplaceAllString(s, "")
}

// TestShellNeverExceedsTheTerminal checks that no row of the shell is wider
// than the terminal, at every width.
func TestShellNeverExceedsTheTerminal(t *testing.T) {
	for _, width := range []int{20, 40, 59, 60, 80, 100, 200} {
		m := NewSearchModel()
		m.resize(width, 30)

		view := m.renderShell(m.viewSearchState())
		for i, line := range strings.Split(shellStrip(view), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("width %d: row %d is %d cells: %q", width, i, got, line)
			}
		}
	}
}

// TestShellHasAnAppBarAndAFooter checks the structure of the shell.
func TestShellHasAnAppBarAndAFooter(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	view := shellStrip(m.renderShell(m.viewSearchState()))
	lines := strings.Split(view, "\n")

	if len(lines) < 3 {
		t.Fatalf("the shell has %d rows, want at least 3", len(lines))
	}
	if !strings.Contains(lines[0], "ANILIX") {
		t.Errorf("the first row is not the app bar: %q", lines[0])
	}
	if !strings.Contains(lines[0], "Search") {
		t.Errorf("the app bar does not carry the screen title: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "menu") {
		t.Errorf("the last row is not the footer: %q", lines[len(lines)-1])
	}
}

// TestPersistentDrawerTakesPartOfTheScreen checks the wide layout: the
// sidebar is a column of its own and the content is narrower than the
// terminal.
func TestPersistentDrawerTakesPartOfTheScreen(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	if !m.navPersistent() {
		t.Fatal("a 100 column terminal must support a persistent drawer")
	}
	if !m.navVisible() {
		t.Fatal("the drawer must be visible by default on a wide terminal")
	}

	drawerWidth := ui.DrawerWidth(100)
	if drawerWidth <= 0 {
		t.Fatalf("the drawer has no width at 100 columns")
	}
	if m.contentWidth != 100-drawerWidth {
		t.Errorf("the content is %d cells wide, want %d", m.contentWidth, 100-drawerWidth)
	}

	view := shellStrip(m.renderShell(m.viewSearchState()))
	if !strings.Contains(view, "Discover") || !strings.Contains(view, "Settings") {
		t.Error("the sidebar does not render the destinations")
	}
}

// TestNarrowTerminalUsesTheModalDrawer checks the compact layout: no
// sidebar, the drawer opens as an overlay.
func TestNarrowTerminalUsesTheModalDrawer(t *testing.T) {
	m := NewSearchModel()
	m.resize(40, 30)

	if m.navPersistent() {
		t.Fatal("a 40 column terminal must not support a persistent drawer")
	}
	if m.navVisible() {
		t.Error("the drawer must not be visible before it is opened")
	}
	if m.contentWidth != 40 {
		t.Errorf("the content is %d cells wide, want the full 40", m.contentWidth)
	}

	// Opening the drawer covers the content region, but it never exceeds
	// the terminal.
	m.toggleDrawer()
	if !m.navVisible() {
		t.Fatal("the menu key did not open the drawer")
	}

	view := shellStrip(m.renderShell(m.viewSearchState()))
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Errorf("row %d is %d cells wide: %q", i, got, line)
		}
	}
	if !strings.Contains(view, "Discover") {
		t.Error("the modal drawer does not render the destinations")
	}
}

// TestMenuKeyTogglesTheDrawer checks the "m" binding.
func TestMenuKeyTogglesTheDrawer(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	model := applyKey(t, m, keyRune('m'))
	if !model.navHasFocus() {
		t.Error("the menu key did not give the keyboard to the drawer")
	}

	model = applyKey(t, model, keyRune('m'))
	if model.navHasFocus() {
		t.Error("the menu key did not close the drawer")
	}
}

// TestEscapeClosesTheDrawer checks the escape binding.
func TestEscapeClosesTheDrawer(t *testing.T) {
	m := NewSearchModel()
	m.resize(40, 30)

	model := applyKey(t, m, keyRune('m'))
	if !model.navVisible() {
		t.Fatal("the drawer did not open")
	}

	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.navVisible() {
		t.Error("escape did not close the drawer")
	}
}

// TestArrowKeysMoveTheDrawerCursor checks the navigation keys of the drawer.
func TestArrowKeysMoveTheDrawerCursor(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	model := applyKey(t, m, keyRune('m'))
	if model.nav.cursor != navIndexOf(navSearch) {
		t.Fatalf("the drawer did not open on the current page")
	}

	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyDown})
	if got := navDestinations[model.nav.cursor].id; got != navHistory {
		t.Fatalf("the down key moved the cursor to %v, want History", got)
	}

	// The destinations that cannot be opened are skipped.
	model = applyKey(t, model, keyRune('j'))
	if got := navDestinations[model.nav.cursor].id; got != navAniList {
		t.Fatalf("the j key moved the cursor to %v, want AniList", got)
	}

	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyUp})
	if got := navDestinations[model.nav.cursor].id; got != navHistory {
		t.Errorf("the up key moved the cursor to %v, want History", got)
	}
}

// TestEnterOpensTheDestination checks that the drawer navigates.
func TestEnterOpensTheDestination(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	model := applyKey(t, m, keyRune('m'))
	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyDown}) // History
	model = applyKey(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})

	if model.state != historyState {
		t.Errorf("enter opened %v, want the history screen", model.state)
	}
	if model.navHasFocus() {
		t.Error("the drawer kept the keyboard after the destination was opened")
	}
}

// TestDrawerKeepsTheKeyboardAwayFromTheContent checks that the drawer is
// modal: the content does not react to the navigation keys.
func TestDrawerKeepsTheKeyboardAwayFromTheContent(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.searchState.Results = nil

	model := applyKey(t, m, keyRune('m'))
	before := model.nav.cursor

	model = applyKey(t, model, keyRune('j'))
	if model.nav.cursor == before {
		t.Error("the j key did not reach the drawer")
	}
	if model.textInput.Value() != "" {
		t.Errorf("the j key was typed into the search field: %q", model.textInput.Value())
	}
}

// TestResizeSwitchesTheDrawerMode checks that resizing the terminal moves
// the drawer between its persistent and its modal form.
func TestResizeSwitchesTheDrawerMode(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)

	if !m.navPersistent() {
		t.Fatal("expected a persistent drawer at 100 columns")
	}

	model := applyKey(t, m, tea.WindowSizeMsg{Width: 40, Height: 30})
	if model.navPersistent() {
		t.Error("expected a modal drawer at 40 columns")
	}
	if model.nav.overlay {
		t.Error("the drawer must not open by itself when the terminal is resized")
	}
	if model.contentWidth != 40 {
		t.Errorf("the content is %d cells wide, want 40", model.contentWidth)
	}
}
