package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TestDiscoveryScreenShowsTheFactsOfTheEntry checks that a result carries
// everything the user needs to recognise it: the title, the alternative
// title, the format, the year, the episodes and the status.
func TestDiscoveryScreenShowsTheFactsOfTheEntry(t *testing.T) {
	m := modelWithResults(120, 30)

	view := shellStrip(m.viewSearchState())

	for _, want := range []string{
		"One Piece",
		"ワンピース",
		"TV · 1999 · 1180 eps",
		"Finished Airing",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the discovery screen lost %q:\n%s", want, view)
		}
	}
}

// TestDiscoveryScreenHasTwoColumns checks that a wide terminal shows the
// results next to the preview of the selected entry.
func TestDiscoveryScreenHasTwoColumns(t *testing.T) {
	m := modelWithResults(120, 30)

	view := shellStrip(m.viewSearchState())

	if !strings.Contains(view, "RESULTS") {
		t.Errorf("the discovery screen lost its result panel:\n%s", view)
	}
	if !strings.Contains(view, "NOW VIEWING") {
		t.Errorf("the discovery screen lost its preview panel:\n%s", view)
	}
	if !strings.Contains(view, "│") {
		t.Errorf("the two panels are not separated:\n%s", view)
	}
}

// TestCompactDiscoveryHasNoPreview checks that a narrow terminal drops the
// preview column instead of squeezing it.
func TestCompactDiscoveryHasNoPreview(t *testing.T) {
	m := modelWithResults(40, 24)

	view := shellStrip(m.viewSearchState())

	if strings.Contains(view, "NOW VIEWING") {
		t.Errorf("a compact terminal must not show a preview panel:\n%s", view)
	}
	if !strings.Contains(view, "RESULTS") {
		t.Errorf("the discovery screen lost its result panel:\n%s", view)
	}
}

// TestDiscoveryScreenFitsTheTerminal checks that the discovery screen never
// grows past the terminal, at any width.
func TestDiscoveryScreenFitsTheTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{40, 24}, {60, 24}, {80, 24}, {120, 40}, {200, 50},
	} {
		m := modelWithResults(size.width, size.height)

		view := m.renderShell(m.viewSearchState())
		if rows := strings.Count(view, "\n") + 1; rows > size.height {
			t.Errorf("the discovery screen at %dx%d is %d rows", size.width, size.height, rows)
		}
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.width {
				t.Errorf("the discovery screen at %dx%d: row %d is %d cells", size.width, size.height, i, got)
			}
		}
	}
}

// TestEnterOpensTheDetailsOfTheEntry checks the walk of the discovery
// screen: a result opens the details of the entry, and the details open the
// episodes.
func TestEnterOpensTheDetailsOfTheEntry(t *testing.T) {
	m := modelWithResults(120, 30)

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.state != animeState {
		t.Fatalf("enter on a result opened %v, want the details of the entry", m.state)
	}

	view := shellStrip(m.viewAnimeState())
	for _, want := range []string{"One Piece", "ワンピース", "Finished Airing", "Adventure"} {
		if !strings.Contains(view, want) {
			t.Errorf("the details screen lost %q:\n%s", want, view)
		}
	}
}

// TestEscapeWalksBack checks that the escape key returns to the view the
// user came from.
func TestEscapeWalksBack(t *testing.T) {
	m := modelWithResults(120, 30)

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.state != animeState {
		t.Fatalf("enter on a result opened %v", m.state)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != searchState {
		t.Errorf("escape returned to %v, want the results", m.state)
	}
}

// TestTheDetailsScreenOpensTheEpisodes checks that the details of an entry
// lead to its episodes.
func TestTheDetailsScreenOpensTheEpisodes(t *testing.T) {
	m := modelWithResults(120, 30)
	m.searchState.Results[0].ShowID = "12345"

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.state != animeState {
		t.Fatalf("enter on a result opened %v", m.state)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.state != detailState {
		t.Fatalf("enter on the details opened %v, want the episodes", m.state)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != animeState {
		t.Errorf("escape returned to %v, want the details of the entry", m.state)
	}
}

// TestTheDetailsScreenScrolls checks that the details of an entry can be
// scrolled: a compact terminal cannot show the whole entry at once.
func TestTheDetailsScreenScrolls(t *testing.T) {
	m := modelWithResults(40, 24)
	m.state = animeState

	if m.animeScroll != 0 {
		t.Fatalf("the details screen starts at row %d", m.animeScroll)
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.animeScroll == 0 {
		t.Error("the down key did not scroll the details")
	}

	m = applyKey(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.animeScroll != 0 {
		t.Errorf("the up key left the details at row %d", m.animeScroll)
	}
}

// TestTheArtworkFallsBackWithoutACover checks that a missing cover leaves a
// clean placeholder instead of a hole in the layout.
func TestTheArtworkFallsBackWithoutACover(t *testing.T) {
	m := modelWithResults(120, 30)

	view := shellStrip(m.viewSearchState())
	if !strings.Contains(view, "No cover art") {
		t.Errorf("the missing cover left no placeholder:\n%s", view)
	}
}

// TestTheSearchKeyActivatesTheField checks that "/" focuses the search
// field.
func TestTheSearchKeyActivatesTheField(t *testing.T) {
	m := modelWithResults(120, 30)
	m.textInput.Blur()

	m = applyKey(t, m, keyRune('/'))
	if !m.textInput.Focused() {
		t.Error("the search key did not focus the search field")
	}
}

// TestTheResultListMovesWithJK checks that the results can be walked with
// the vi keys.
func TestTheResultListMovesWithJK(t *testing.T) {
	m := modelWithResults(120, 30)
	m.textInput.Blur()

	m = applyKey(t, m, keyRune('j'))
	if m.searchList.Index() != 1 {
		t.Errorf("j moved the cursor to %d, want 1", m.searchList.Index())
	}

	m = applyKey(t, m, keyRune('k'))
	if m.searchList.Index() != 0 {
		t.Errorf("k moved the cursor to %d, want 0", m.searchList.Index())
	}
}
