package ui

import (
	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// InputField is a bordered text field.
//
// The field owns its frame, not its text: the value is rendered by the
// caller and handed over, so an editable input and a read-only value share
// the same frame and the same focus treatment.
//
// The focus is expressed with the border colour and the icon, never with a
// second glyph: a field that changes its shape when it gains the focus
// makes the text jump.
type InputField struct {
	// Content is the pre-rendered value of the field.
	Content string
	// Placeholder is shown when there is no content.
	Placeholder string
	// Focused reports whether the field holds the keyboard.
	Focused bool
	// Icon is the leading glyph. It must be one cell wide.
	Icon string
	// Width is the outer width of the field: border and padding included.
	Width int
}

// Render renders the input field.
func (f InputField) Render() string {
	icon := f.Icon
	if icon == "" {
		icon = theme.Icon(theme.IconSearch)
	}

	iconRole := theme.Muted
	border := theme.Outline
	if f.Focused {
		iconRole = theme.Primary
		border = theme.Primary
	}

	body := f.Content
	if lipgloss.Width(body) == 0 {
		body = theme.Fg(lipgloss.NewStyle(), theme.Muted).Render(f.Placeholder)
	}

	style := theme.Bg(lipgloss.NewStyle().Padding(0, 1), theme.SurfaceLow).
		Border(theme.BorderRounded.Border(), true)
	style = theme.BorderFg(style, border)

	inner := f.Width - 4
	if f.Width > 0 {
		style = style.Width(f.Width)
		if inner < 1 {
			inner = 1
		}
	}

	content := theme.Fg(lipgloss.NewStyle(), iconRole).Render(icon) + " " + body
	if inner > 0 {
		content = lipgloss.NewStyle().Width(inner).Render(content)
	}
	return style.Render(content)
}
