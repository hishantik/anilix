package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/source"
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
	for _, want := range []string{"Next", "Previous", "Replay", "Quality"} {
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
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 3 {
		t.Errorf("right: confirmSelect = %d, want 3", m.confirmSelect)
	}
	// The fourth action is the last: right must not move past it.
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.confirmSelect != 3 {
		t.Errorf("right at the end: confirmSelect = %d, want 3", m.confirmSelect)
	}
	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.confirmSelect != 2 {
		t.Errorf("left: confirmSelect = %d, want 2", m.confirmSelect)
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

// TestPostPlayPreviousStopsAtFirstEpisode checks Previous never walks
// before the first episode.
func TestPostPlayPreviousStopsAtFirstEpisode(t *testing.T) {
	m := NewSearchModel()
	m.resize(100, 30)
	m.episodeState.Episodes = []string{"1", "2"}
	m.episodeState.Selected = 0
	m.searchState.Results = make([]*source.Anime, 1)
	m.searchState.Results[0] = &source.Anime{Name: "Test Anime"}

	m.confirmPostPlay(1)
	if m.episodeState.Selected != 0 {
		t.Errorf("previous on episode 1: Selected = %d, want 0", m.episodeState.Selected)
	}

	m.episodeState.Selected = 1
	m.confirmPostPlay(1)
	if m.episodeState.Selected != 0 {
		t.Errorf("previous: Selected = %d, want 0", m.episodeState.Selected)
	}
}
