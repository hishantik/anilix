package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// AppBar is the top application bar of the application shell.
//
// The bar carries the navigation control (the hamburger), the brand, the
// title of the active screen and the status items on the right. It is one
// row tall: a terminal has a single font size, so the hierarchy comes from
// weight and colour, not from size.
//
// The bar never overflows. When the terminal is too narrow for the status
// items, the least important ones are dropped; the brand and the screen
// title always survive.
type AppBar struct {
	// Brand is the wordmark. It is usually pre-styled by the caller.
	Brand string
	// Title is the title of the active screen.
	Title string
	// Menu shows the navigation control.
	Menu bool
	// MenuOpen marks the navigation control as active.
	MenuOpen bool
	// Right holds the status items, most important first. Items are dropped
	// from the end when the bar does not fit.
	Right []string
	// Width is the width of the bar.
	Width int
}

// Render renders the app bar.
func (a AppBar) Render() string {
	width := a.Width
	if width < 1 {
		width = 1
	}

	left := a.renderLeft()
	if lipgloss.Width(left) > width {
		left = lipgloss.NewStyle().MaxWidth(width).Render(left)
	}
	right := a.fitRight(left, width)
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	gap := width - leftWidth - rightWidth
	if rightWidth > 0 && gap < 1 {
		gap = 1
	}
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderLeft renders the hamburger, the brand and the screen title.
func (a AppBar) renderLeft() string {
	parts := make([]string, 0, 5)

	if a.Menu {
		role := theme.Muted
		if a.MenuOpen {
			role = theme.Primary
		}
		parts = append(parts, theme.Fg(lipgloss.NewStyle().Bold(true), role).Render(theme.Icon(theme.IconMenu)))
		parts = append(parts, " ")
	}
	if a.Brand != "" {
		parts = append(parts, a.Brand)
		parts = append(parts, " ")
	}
	if a.Title != "" {
		if a.Brand != "" {
			parts = append(parts, theme.Fg(lipgloss.NewStyle(), theme.Outline).Render("│"))
			parts = append(parts, " ")
		}
		parts = append(parts, theme.Headline.Render(a.Title))
	}

	return strings.Join(parts, "")
}

// fitRight drops the least important status items until the bar fits.
func (a AppBar) fitRight(left string, width int) string {
	items := make([]string, len(a.Right))
	copy(items, a.Right)

	for {
		right := strings.Join(items, "  ")
		if lipgloss.Width(left)+lipgloss.Width(right)+1 <= width || len(items) == 0 {
			return right
		}
		items = items[:len(items)-1]
	}
}
