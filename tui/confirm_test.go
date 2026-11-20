package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

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

// actionRow returns the raw dialog row that carries the actions, escape
// sequences included, so the test can inspect the styling of each segment.
func actionRow(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Yes") && strings.Contains(line, "No") {
			return line
		}
	}
	t.Fatalf("the dialog has no action row:\n%s", shellStrip(view))
	return ""
}

// actionLine returns the action row without the escape sequences, so the
// test can look at the order of what is drawn.
func actionLine(t *testing.T, view string) string {
	t.Helper()
	return shellStrip(actionRow(t, view))
}

// styleBefore returns the escape sequence that opens the styled run of the
// label, i.e. what sits between the previous reset and the label itself.
func styleBefore(row, label string) string {
	i := strings.Index(row, label)
	if i < 0 {
		return ""
	}
	j := strings.LastIndex(row[:i], "\x1b[m")
	if j < 0 {
		return row[:i]
	}
	return row[j+len("\x1b[m") : i]
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
// visible: the selected action is the filled pill of the segmented switch,
// exactly like the SUB/DUB toggle, and the labels never swap places.
func TestQuitDialogShowsTheSelectedAction(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	fill := theme.PrimaryContainer.Color()
	if fill == nil {
		t.Fatal("expected a primary container colour in truecolor mode")
	}
	r, g, b, _ := fill.RGBA()
	code := fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)

	m := NewSearchModel()
	m.resize(100, 30)
	m.state = confirmQuitState

	for _, tc := range []struct {
		choice int
		filled string
		ghost  string
	}{
		{0, "Yes", "No"},
		{1, "No", "Yes"},
	} {
		m.confirmSelect = tc.choice
		view := m.viewConfirmQuit()
		row := actionRow(t, view)

		// The labels keep their fixed order: only the highlight moves.
		line := actionLine(t, view)
		if yes, no := strings.Index(line, "Yes"), strings.Index(line, "No"); yes < 0 || no < 0 || yes > no {
			t.Errorf("choice %d: the labels swapped: %q", tc.choice, line)
		}

		if !strings.Contains(styleBefore(row, tc.filled), code) {
			t.Errorf("choice %d: %s is not the filled segment", tc.choice, tc.filled)
		}
		if strings.Contains(styleBefore(row, tc.ghost), code) {
			t.Errorf("choice %d: %s carries the fill too", tc.choice, tc.ghost)
		}
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
