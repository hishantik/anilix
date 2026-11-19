package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// openQuitPrompt opens the quit dialog from the search screen.
func openQuitPrompt(t *testing.T, m *SearchModel) *SearchModel {
	t.Helper()
	m.textInput.Blur()
	m = applyKey(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.state != confirmQuitState {
		t.Fatalf("state after ctrl+c = %v, want confirmQuitState", m.state)
	}
	return m
}

// actionLine returns the dialog row that carries the buttons, without the
// escape sequences, so the test can look at the order of what is drawn.
func actionLine(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(shellStrip(view), "\n") {
		if strings.Contains(line, "Yes") && strings.Contains(line, "No") {
			return line
		}
	}
	t.Fatalf("the dialog has no action row:\n%s", shellStrip(view))
	return ""
}

// TestQuitDialogArrowsMoveTheSelection checks that the arrow keys move the
// cursor between the two actions.
func TestQuitDialogArrowsMoveTheSelection(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m = openQuitPrompt(t, m)

	// The prompt opens on "No": a stray enter must cancel, not quit.
	if m.confirmSelect != 1 {
		t.Errorf("the prompt opens with confirmSelect = %d, want 1 (No)", m.confirmSelect)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.confirmSelect != 0 {
		t.Errorf("left: confirmSelect = %d, want 0 (Yes)", m.confirmSelect)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 1 {
		t.Errorf("right: confirmSelect = %d, want 1 (No)", m.confirmSelect)
	}
}

// TestQuitDialogVerticalKeysToggleTheSelection checks that the keys the
// logout prompt already accepts work on the quit prompt too.
func TestQuitDialogVerticalKeysToggleTheSelection(t *testing.T) {
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyTab},
		{Code: 'k', Text: "k"},
		{Code: 'j', Text: "j"},
	} {
		m := NewSearchModel()
		m.resize(100, 30)
		m = openQuitPrompt(t, m)
		start := m.confirmSelect

		m = applyKey(t, m, msg)
		if m.confirmSelect == start {
			t.Errorf("%s did not move the selection", msg.String())
		}
	}
}

// TestQuitDialogShowsTheSelectedAction checks that moving the cursor is
// visible: the selected action carries the accent bar. Colour alone is not
// enough, because the focused and the hovered shades of the accent are
// nearly the same.
func TestQuitDialogShowsTheSelectedAction(t *testing.T) {
	bar := theme.Icon(theme.IconSelectionBar)

	m := NewSearchModel()
	m.resize(100, 30)
	m.state = confirmQuitState

	m.confirmSelect = 0
	line := actionLine(t, m.viewConfirmQuit())
	if i, yes := strings.Index(line, bar), strings.Index(line, "Yes"); i < 0 || i > yes {
		t.Errorf("the bar is not in front of the selected Yes: %q", line)
	}

	m.confirmSelect = 1
	line = actionLine(t, m.viewConfirmQuit())
	if i, yes := strings.Index(line, bar), strings.Index(line, "Yes"); i < 0 || i < yes {
		t.Errorf("the bar is not in front of the selected No: %q", line)
	}
}

// TestLogoutDialogArrowsMoveTheSelection checks the same cursor on the
// AniList logout prompt, which shares the dialog.
func TestLogoutDialogArrowsMoveTheSelection(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.state = confirmLogoutState
	m.confirmSelect = 1

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.confirmSelect != 0 {
		t.Errorf("left: confirmSelect = %d, want 0 (Yes)", m.confirmSelect)
	}
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 1 {
		t.Errorf("right: confirmSelect = %d, want 1 (No)", m.confirmSelect)
	}
}
