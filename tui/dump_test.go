package tui

import (
	"fmt"
	"testing"
)

func TestDumpScreens(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 40}, {80, 24}, {40, 24}} {
		m := modelWithResults(size.w, size.h)
		m.searchState.Selected = 0
		fmt.Printf("\n===== SEARCH %dx%d =====\n%s\n", size.w, size.h, shellStrip(m.renderShell(m.viewSearchState())))

		m.state = animeState
		fmt.Printf("\n===== ANIME %dx%d =====\n%s\n", size.w, size.h, shellStrip(m.renderShell(m.viewAnimeState())))

		m.state = searchState
		m.nav.expanded = false
		m.resize(size.w, size.h)
		fmt.Printf("\n===== SEARCH no drawer %dx%d =====\n%s\n", size.w, size.h, shellStrip(m.renderShell(m.viewSearchState())))
	}
}
