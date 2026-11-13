package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/hishantik/anilix/tui/theme"
)

// TestDrawerRendersExactSize checks that the drawer fills the box it was
// given: a column of items must not leave a hole in the surface.
func TestDrawerRendersExactSize(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	drawer := Drawer{
		Title: "Menu",
		Items: []NavItem{
			{Label: "Discover", Icon: theme.Icon(theme.IconDiscover), Enabled: false},
			{Label: "Search", Icon: theme.Icon(theme.IconSearch), Active: true, Selected: true},
			{Label: "History", Icon: theme.Icon(theme.IconHistory), Enabled: true},
		},
		Footer:  "esc close",
		Width:   22,
		Height:  12,
		Focused: true,
		Fill:    theme.Surface,
	}

	lines := strings.Split(strip(drawer.Render()), "\n")
	if len(lines) != 12 {
		t.Fatalf("the drawer has %d rows, want 12", len(lines))
	}
	for i, line := range lines {
		if got := visibleWidth(line); got != 22 {
			t.Errorf("row %d is %d cells, want 22: %q", i, got, line)
		}
	}
}

// TestDrawerRuleTakesOneCell checks that the separator is part of the
// width of the drawer.
func TestDrawerRuleTakesOneCell(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	drawer := Drawer{
		Items:  []NavItem{{Label: "Search", Enabled: true}},
		Width:  20,
		Height: 4,
		Rule:   true,
	}

	for i, line := range strings.Split(strip(drawer.Render()), "\n") {
		if got := visibleWidth(line); got != 20 {
			t.Errorf("row %d is %d cells, want 20: %q", i, got, line)
		}
	}
}

// TestDrawerKeepsItsLabels checks that the destinations survive the
// rendering, including the ones that cannot be opened.
func TestDrawerKeepsItsLabels(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	drawer := Drawer{
		Items: []NavItem{
			{Label: "Discover", Enabled: false},
			{Label: "Search", Enabled: true},
			{Label: "Watchlist", Enabled: false},
		},
		Width: 22,
	}

	out := strip(drawer.Render())
	for _, label := range []string{"Discover", "Search", "Watchlist"} {
		if !strings.Contains(out, label) {
			t.Errorf("the drawer lost the %q destination", label)
		}
	}
}

// TestMenuItemNeverOverflows checks that a long label and a long hint
// still fit into the row.
func TestMenuItemNeverOverflows(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	items := []MenuItem{
		{Label: "A very long destination name", Hint: "with a hint"},
		{Label: "Short", Hint: "a very long hint as well"},
		{Label: "Short", Disabled: true},
		{Label: "Short", Selected: true, Focused: true, Active: true},
	}
	for _, item := range items {
		out := item.Render(18)
		if got := visibleWidth(out); got != 18 {
			t.Errorf("menu item is %d cells, want 18: %q", got, out)
		}
	}
}

// TestAppBarNeverOverflows checks that the bar fits into the terminal and
// that the least important status is dropped first.
func TestAppBarNeverOverflows(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	bar := AppBar{
		Brand: "ANILIX",
		Title: "Search",
		Menu:  true,
		Right: []string{"SUB", "1080p", "user"},
		Width: 40,
	}

	out := bar.Render()
	if got := visibleWidth(out); got != 40 {
		t.Errorf("the app bar is %d cells, want 40: %q", got, out)
	}
	if !strings.Contains(strip(out), "ANILIX") {
		t.Error("the app bar lost the brand")
	}

	// A bar that is far too narrow still has to fit.
	bar.Width = 12
	if got := visibleWidth(bar.Render()); got > 12 {
		t.Errorf("the app bar is %d cells, want at most 12", got)
	}
}

// TestDrawerWidthRules checks the width of the persistent drawer.
func TestDrawerWidthRules(t *testing.T) {
	if SupportsPersistentDrawer(40) {
		t.Error("a compact terminal must not have a persistent drawer")
	}
	if !SupportsPersistentDrawer(60) {
		t.Error("a medium terminal must have a persistent drawer")
	}
	if got := DrawerWidth(40); got != 0 {
		t.Errorf("the drawer has width %d on a compact terminal, want 0", got)
	}
	if got := DrawerWidth(100); got <= 0 || got >= 100 {
		t.Errorf("the drawer has width %d at 100 columns", got)
	}
	// The content keeps at least 36 columns.
	if got := DrawerWidth(60); got > 60-36 {
		t.Errorf("the drawer has width %d at 60 columns, it leaves too little for the content", got)
	}
}

// TestOverlayDrawerWidthIsClamped checks that the modal drawer fits on a
// phone screen.
func TestOverlayDrawerWidthIsClamped(t *testing.T) {
	if got := OverlayDrawerWidth(40); got > 40 {
		t.Errorf("the drawer is %d cells wide on a 40 column terminal", got)
	}
	if got := OverlayDrawerWidth(120); got != 30 {
		t.Errorf("the drawer is %d cells wide, want 30", got)
	}
}
