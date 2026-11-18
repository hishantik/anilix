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

// PillSwitch is a two-option toggle, rendered as adjacent pill segments
// (e.g. SUB / DUB). Only one segment is active at a time.
type PillSwitch struct {
	Active   string // label of the active (selected) option
	Inactive string // label of the other option
}

// Render renders the pill switch: the active side as a filled pill and the
// inactive side as a ghost label to the right, reading as one control.
func (p PillSwitch) Render() string {
	filled := theme.Bg(
		theme.Fg(lipgloss.NewStyle().Bold(true), theme.OnPrimaryContainer),
		theme.PrimaryContainer,
	).Padding(0, 1).Render(p.Active)

	ghost := theme.Fg(lipgloss.NewStyle(), theme.Muted).Render(" " + p.Inactive + " ")

	return filled + ghost
}

// renderSubDubSwitch renders the sub/dub toggle of the app bar.
//
// It is a pill switch: the active translation is a filled pill and the
// other one stays muted, so the two read as one tappable control.
func renderSubDubSwitch(translationType string) string {
	on, off := "SUB", "DUB"
	if translationType != "sub" {
		on, off = off, on
	}
	return PillSwitch{Active: on, Inactive: off}.Render()
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
