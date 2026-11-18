package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestRecentSearchRowsMatchTheClickMapping checks the row math of the
// recent searches list against the rendered screen: every entry has to sit
// on the content row that recentSearchAt maps to it, and the field and the
// header must not be clickable.
func TestRecentSearchRowsMatchTheClickMapping(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(100, 30)
	m.recentSearches = []string{"frieren", "monster", "dandadan"}

	lines := strings.Split(shellStrip(m.viewSearchState()), "\n")

	for i, query := range m.recentSearches {
		row := recentSearchesTop + i
		if row >= len(lines) {
			t.Fatalf("recent search %d is past the rendered screen (%d rows)", i, len(lines))
		}
		if !strings.Contains(lines[row], query) {
			t.Errorf("row %d does not show recent search %q: %q", row, query, lines[row])
		}
		if got := m.recentSearchAt(row); got != i {
			t.Errorf("recentSearchAt(%d) = %d, want %d", row, got, i)
		}
	}

	// The field, the blank lines, the header and the row below the last
	// entry are not recent searches.
	for _, row := range []int{0, 1, 2, 3, 4, 5, recentSearchesTop + len(m.recentSearches)} {
		if got := m.recentSearchAt(row); got != -1 {
			t.Errorf("recentSearchAt(%d) = %d, want -1", row, got)
		}
	}
}

// TestClickingARecentSearchRunsIt checks the click path end to end: the
// terminal row of the second entry selects the second query.
func TestClickingARecentSearchRunsIt(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(100, 30)
	m.recentSearches = []string{"frieren", "monster", "dandadan"}

	// The app bar is terminal row 0, so a content row is one row further
	// down on the terminal.
	click := tea.MouseClickMsg{X: 2, Y: recentSearchesTop + 1 + 1}
	m = applyKey(t, m, click)

	if m.lastQuery != "monster" {
		t.Errorf("the click ran %q, want %q", m.lastQuery, "monster")
	}
	if m.recentCursor != 1 {
		t.Errorf("the cursor is on %d, want 1", m.recentCursor)
	}
}

// TestTheOpenDrawerCoversTheRecentSearches checks that a click does not
// reach the list through the modal drawer: the drawer replaces the content
// region, so the rows under it are not the recent searches any more.
func TestTheOpenDrawerCoversTheRecentSearches(t *testing.T) {
	tempHome(t)
	m := NewSearchModel()
	m.resize(50, 30) // compact: the drawer is a modal overlay
	m.recentSearches = []string{"frieren", "monster"}

	if got := m.recentSearchAt(recentSearchesTop); got != 0 {
		t.Fatalf("recentSearchAt(%d) = %d before the drawer opened, want 0", recentSearchesTop, got)
	}
	m.nav.overlay = true
	if got := m.recentSearchAt(recentSearchesTop); got != -1 {
		t.Errorf("recentSearchAt(%d) = %d with the drawer open, want -1", recentSearchesTop, got)
	}
}
