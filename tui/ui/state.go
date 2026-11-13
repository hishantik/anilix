package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// EmptyState is the placeholder shown when a screen has nothing to show.
//
// It exists so "no results", "nothing here yet" and "an error happened"
// never look like the same blank space.
type EmptyState struct {
	// Icon is the leading glyph. It must be one cell wide.
	Icon string
	// Title is the short statement of what happened.
	Title string
	// Hint is the optional explanation or the action to take.
	Hint string
	// Action is an optional already-rendered action, e.g. a button.
	Action string
}

// Render renders the empty state.
func (e EmptyState) Render(width int) string {
	if width < 1 {
		width = 1
	}
	icon := e.Icon
	if icon == "" {
		icon = theme.Icon(theme.IconInfo)
	}

	lines := make([]string, 0, 3)
	lines = append(lines, styledLine([]segment{
		{text: icon, role: theme.Muted},
		{text: " ", role: theme.RoleUnset},
		{text: e.Title, role: theme.Text, bold: true},
	}, width, theme.RoleUnset))
	if e.Hint != "" {
		lines = append(lines, styledLine([]segment{
			{text: "  " + e.Hint, role: theme.Muted},
		}, width, theme.RoleUnset))
	}
	if e.Action != "" {
		lines = append(lines, styledLine([]segment{
			{text: "  " + e.Action, role: theme.RoleUnset},
		}, width, theme.RoleUnset))
	}
	return strings.Join(lines, "\n")
}

// ErrorState is the placeholder shown when a request failed.
//
// The error colour is reserved for the failure itself; the explanation stays
// in the regular text colour so a failed screen is readable, not alarming.
type ErrorState struct {
	// Title is the short statement of what failed.
	Title string
	// Message is the underlying error.
	Message string
	// Hint is the optional action to take, e.g. "ctrl+r to retry".
	Hint string
}

// Render renders the error state.
func (e ErrorState) Render(width int) string {
	if width < 1 {
		width = 1
	}
	lines := make([]string, 0, 3)
	lines = append(lines, styledLine([]segment{
		{text: theme.Icon(theme.IconWarning), role: theme.Error},
		{text: " ", role: theme.RoleUnset},
		{text: e.Title, role: theme.Error, bold: true},
	}, width, theme.RoleUnset))
	if e.Message != "" {
		lines = append(lines, styledLine([]segment{
			{text: "  " + e.Message, role: theme.Muted},
		}, width, theme.RoleUnset))
	}
	if e.Hint != "" {
		lines = append(lines, styledLine([]segment{
			{text: "  " + e.Hint, role: theme.Disabled},
		}, width, theme.RoleUnset))
	}
	return strings.Join(lines, "\n")
}

// ProgressBar renders a determinate progress bar.
//
// The bar is drawn with block characters so it reads as a solid surface; on
// a terminal without unicode it degrades to a row of hashes.
func ProgressBar(percent int, width int) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := percent * width / 100
	filled = min(filled, width)

	full := "█"
	empty := "░"
	if !theme.HasUnicode() {
		full = "#"
		empty = "."
	}

	bar := strings.Repeat(full, filled) + strings.Repeat(empty, width-filled)
	return theme.Fg(lipgloss.NewStyle(), theme.Primary).Render(bar)
}

// LoadingIndicator renders a spinner with a label.
func LoadingIndicator(label string, tick int) string {
	spinner := theme.Fg(lipgloss.NewStyle(), theme.Primary).Render(theme.SpinnerFrame(tick))
	if label == "" {
		return spinner
	}
	return spinner + " " + theme.Fg(lipgloss.NewStyle(), theme.Muted).Render(label)
}

// Rating renders a score with the star icon, degrading to a plain number.
func Rating(score float64) string {
	if score <= 0 {
		return ""
	}
	if !theme.HasUnicode() {
		return theme.Label.Render(formatScore(score))
	}
	return theme.Fg(lipgloss.NewStyle(), theme.Primary).
		Render(theme.Icon(theme.IconStar) + " " + formatScore(score))
}

// RatingText returns the plain text of a score, e.g. "★ 8.73".
//
// It exists for the rows that colour their own text: a row that styles a
// pre-rendered value would nest two escape sequences, and the inner reset
// would clear the fill of the row.
func RatingText(score float64) string {
	if score <= 0 {
		return ""
	}
	if !theme.HasUnicode() {
		return formatScore(score)
	}
	return theme.Icon(theme.IconStar) + " " + formatScore(score)
}

// formatScore formats a score with two decimals.
func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 2, 64)
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
