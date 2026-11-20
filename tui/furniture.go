package tui

import (
	"image/color"
	"strings"

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

// PillSwitch is a segmented toggle rendered as adjacent pill segments
// (e.g. SUB / DUB). The options keep their order no matter which one is
// active: only the highlight moves, so the controls never swap places.
type PillSwitch struct {
	// Options are the labels, in their fixed left-to-right order.
	Options []string
	// Current is the label of the active (selected) option.
	Current string
}

// Render renders the pill switch: the active option as a filled pill and
// the other options as ghost labels, each in its fixed position.
func (p PillSwitch) Render() string {
	parts := make([]string, 0, len(p.Options))
	for _, option := range p.Options {
		if option == p.Current {
			parts = append(parts, theme.Bg(
				theme.Fg(lipgloss.NewStyle().Bold(true), theme.OnPrimaryContainer),
				theme.PrimaryContainer,
			).Padding(0, 1).Render(option))
			continue
		}
		parts = append(parts, theme.Fg(lipgloss.NewStyle(), theme.Muted).Render(" "+option+" "))
	}
	return strings.Join(parts, "")
}

// renderSubDubSwitch renders the sub/dub toggle of the app bar.
//
// It is a pill switch with a fixed order - SUB first, DUB second - so
// toggling between them moves only the highlight, never the labels.
func renderSubDubSwitch(translationType string) string {
	current := "SUB"
	if translationType != "sub" {
		current = "DUB"
	}
	return PillSwitch{Options: []string{"SUB", "DUB"}, Current: current}.Render()
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
