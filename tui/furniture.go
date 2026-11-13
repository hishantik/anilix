package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// The furniture the screens share.
//
// Everything here is specific to Anilix rather than to the design system:
// the design system itself lives in tui/theme (the tokens) and tui/ui (the
// components). A screen assembles the two and never builds a lipgloss style
// of its own.

// brandWordmark renders the wordmark of the app bar.
func brandWordmark() string {
	return theme.Display.Render("ANILIX")
}

// renderSubDubSwitch renders the sub/dub toggle of the app bar.
//
// It is a segmented control: the active translation carries the accent
// colour and the other one stays muted. The separator is drawn in the
// outline colour, so the two halves read as one control instead of two
// labels that happen to sit next to each other.
func renderSubDubSwitch(translationType string) string {
	on, off := "SUB", "DUB"
	if translationType != "sub" {
		on, off = off, on
	}

	active := theme.Fg(lipgloss.NewStyle().Bold(true), theme.Primary).Render(on)
	inactive := theme.Fg(lipgloss.NewStyle(), theme.Muted).Render(off)
	separator := theme.Fg(lipgloss.NewStyle(), theme.Outline).Render("│")

	return active + " " + separator + " " + inactive
}

// loadingLine renders a spinner with a label.
func (m *SearchModel) loadingLine(label string) string {
	if label == "" {
		return m.loading.View()
	}
	return m.loading.View() + " " + theme.Label.Render(label)
}

// statusLine renders a short status message in the success colour.
func statusLine(text string) string {
	return theme.Fg(lipgloss.NewStyle(), theme.Success).Render(text)
}

// mutedLine renders a short status message in the muted colour.
func mutedLine(text string) string {
	return theme.Label.Render(text)
}

// errorLine renders a failure message.
func errorLine(text string) string {
	return theme.Fg(lipgloss.NewStyle(), theme.Error).Render(text)
}

// wrapText renders text in a block of the given width.
//
// The style carries the colour and the weight, the width carries the
// layout: the two never get mixed, so a screen cannot accidentally break
// the wrap of a styled run.
func wrapText(text string, width int, style lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	return style.Width(width).Render(text)
}

// roleColor returns the colour of a role for the widgets that need a
// colour.Color instead of a style.
//
// A terminal without colour support gets the no-op colour, so the widget
// renders without an escape sequence instead of with a broken one.
func roleColor(role theme.Role) color.Color {
	if c := role.Color(); c != nil {
		return c
	}
	return lipgloss.NoColor{}
}
