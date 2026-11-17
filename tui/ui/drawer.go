package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// NavItem is one destination of the navigation drawer.
type NavItem struct {
	// Label is the destination name.
	Label string
	// Icon is the leading glyph. It must be one cell wide.
	Icon string
	// Hint is the right-aligned secondary text, e.g. a count or a reason.
	Hint string
	// Active marks the destination of the current page.
	Active bool
	// Selected marks the cursor row.
	Selected bool
	// Enabled reports whether the destination can be opened.
	Enabled bool
}

// Drawer renders the navigation drawer.
//
// The drawer is the same component whether it is a persistent sidebar or a
// modal overlay: only the width, the height and the fill differ. A drawer
// is a column, so it always renders exactly Height rows and never leaves a
// hole in the surface it sits on.
//
// Three states are told apart, following the design system:
//
//   - the cursor row (Selected) carries the accent bar,
//   - the current page (Active) keeps the accent tint,
//   - a destination the app cannot open is dimmed and stays in place.
type Drawer struct {
	// Title is the optional heading of the drawer.
	Title string
	// Items are the destinations.
	Items []NavItem
	// Footer is the optional hint line at the bottom of the drawer.
	Footer string
	// Width is the width of the drawer in cells.
	Width int
	// Height is the height of the drawer in rows. Zero means "as tall as
	// the content needs".
	Height int
	// Focused reports whether the drawer holds the keyboard.
	Focused bool
	// Fill is the surface the drawer sits on.
	Fill theme.Role
	// Rule draws a vertical rule on the right edge of the drawer. It
	// separates the sidebar from the content without relying on colour.
	Rule bool
}

// Render renders the drawer.
func (d Drawer) Render() string {
	width := d.Width
	if width < 8 {
		width = 8
	}
	inner := width
	if d.Rule {
		inner--
		if inner < 8 {
			inner = 8
		}
	}

	lines := make([]string, 0, len(d.Items)+4)

	if d.Title != "" {
		lines = append(lines, styledLine([]segment{
			{text: strings.ToUpper(d.Title), role: theme.Primary, bold: true},
		}, inner, d.Fill))
	}

	for _, item := range d.Items {
		lines = append(lines, MenuItem{
			Label:    item.Label,
			Icon:     item.Icon,
			Hint:     item.Hint,
			Selected: item.Selected,
			Focused:  d.Focused,
			Active:   item.Active,
			Disabled: !item.Enabled,
			Fill:     d.Fill,
		}.Render(inner))
	}

	if d.Footer != "" {
		// The hint sits at the bottom of the drawer, not right below the
		// items: it belongs to the drawer, not to the last destination.
		pad := d.Height - len(lines) - 1
		for i := 0; i < pad; i++ {
			lines = append(lines, filledLine(inner, d.Fill))
		}
		footer := lipgloss.NewStyle().MaxWidth(inner).Render(d.Footer)
		lines = append(lines, styledLine([]segment{
			{text: footer, role: theme.Muted},
		}, inner, d.Fill))
	}

	for len(lines) < d.Height {
		lines = append(lines, filledLine(inner, d.Fill))
	}

	if d.Rule {
		rule := theme.Bg(theme.Fg(lipgloss.NewStyle(), theme.Outline), d.Fill).Render("│")
		for i := range lines {
			lines[i] += rule
		}
	}

	return strings.Join(lines, "\n")
}
