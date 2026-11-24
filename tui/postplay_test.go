package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// openPostPlay puts the model on the post-playback page.
func openPostPlay(t *testing.T, m *SearchModel) *SearchModel {
	t.Helper()
	m.textInput.Blur()
	m.state = postPlayState
	return m
}

// TestPostPlayStateInView checks the finished-playing page renders and
// carries the three actions in their fixed order.
func TestPostPlayStateInView(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.state = postPlayState

	view := m.View().Content
	for _, want := range []string{"Next", "Replay", "Quality"} {
		if !strings.Contains(shellStrip(view), want) {
			t.Errorf("the post-play page is missing %q:\n%s", want, shellStrip(view))
		}
	}
}

// TestPostPlayArrowsMoveTheSelection checks the cursor moves across the
// three actions and stays within bounds.
func TestPostPlayArrowsMoveTheSelection(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.state = postPlayState
	m.confirmSelect = 0

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 1 {
		t.Errorf("right: confirmSelect = %d, want 1", m.confirmSelect)
	}
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 2 {
		t.Errorf("right: confirmSelect = %d, want 2", m.confirmSelect)
	}
	// The third action is the last: right must not move past it.
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 2 {
		t.Errorf("right at the end: confirmSelect = %d, want 2", m.confirmSelect)
	}
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.confirmSelect != 1 {
		t.Errorf("left: confirmSelect = %d, want 1", m.confirmSelect)
	}
}

// TestPostPlayEscapeReturnsToDetail checks esc leaves the page without
// touching the player.
func TestPostPlayEscapeReturnsToDetail(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.state = postPlayState
	m.prevState = detailState

	m = applyKey(t, m, keyRune('n'))
	if m.state != detailState {
		t.Errorf("esc: state = %v, want detailState", m.state)
	}
}
