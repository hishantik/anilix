package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestShowcaseRenders checks that the design system showcase builds and
// covers the parts of the system a screen depends on.
func TestShowcaseRenders(t *testing.T) {
	out := Showcase()

	sections := []string{
		"Anilix Design System",
		"TERMINAL CAPABILITIES",
		"COLOR TOKENS",
		"TYPOGRAPHY",
		"SPACING (CELLS)",
		"LAYOUT BREAKPOINTS",
		"BUTTONS",
		"CHIPS AND BADGES",
		"TABS",
		"LIST ITEMS",
		"MENU ITEMS",
		"CARD",
		"DIALOG",
		"EMPTY AND ERROR STATES",
		"DIVIDERS AND HEADERS",
		"ICONS",
		"SEARCH FIELD",
		"KEY HINTS",
	}
	for _, section := range sections {
		if !strings.Contains(out, section) {
			t.Errorf("the showcase is missing the %q section", section)
		}
	}
}

// TestThemeKeyOpensTheShowcase checks the ctrl+g binding and that esc
// returns to the previous screen.
func TestThemeKeyOpensTheShowcase(t *testing.T) {
	m := NewSearchModel()
	m.width, m.height = 100, 40

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	model, ok := updated.(*SearchModel)
	if !ok {
		t.Fatalf("Update returned %T, want *SearchModel", updated)
	}
	if model.state != themeState {
		t.Fatalf("ctrl+g did not open the showcase, state is %v", model.state)
	}

	view := model.viewThemeShowcase()
	if !strings.Contains(view, "Anilix Design System") {
		t.Error("the showcase view does not render the design system")
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model, ok = updated.(*SearchModel)
	if !ok {
		t.Fatalf("Update returned %T, want *SearchModel", updated)
	}
	if model.state != searchState {
		t.Errorf("esc did not close the showcase, state is %v", model.state)
	}
}
