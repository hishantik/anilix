package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// Artwork is the frame around the cover art of an entry.
//
// The image itself is rendered by the caller: the terminal image protocols
// produce *text* - half blocks for the ANSI fallback, Unicode placeholders
// for the kitty protocol - and not pixels. The component frames what it is
// given and never decides how the picture is made.
//
// When there is no image - no protocol, no cover on the provider, a failed
// download - the component draws a clean placeholder instead of leaving a
// hole in the layout. A screen never has to know whether the terminal can
// show a picture.
type Artwork struct {
	// Image is the pre-rendered cover. It may contain escape sequences.
	Image string
	// Title is the name of the entry. It is shown in the placeholder.
	Title string
	// Width is the outer width of the frame, the border included.
	Width int
	// Height is the outer height of the frame, the border included.
	Height int
}

// Render renders the artwork.
//
// The frame is exactly as wide as it was asked to be, on every row: a cover
// that is wider than its column would push the rest of the screen out of
// the terminal.
func (a Artwork) Render() string {
	width := a.Width
	if width < 4 {
		return ""
	}
	inner := width - 2

	rows := a.Height - 2
	if rows < 1 {
		rows = 1
	}

	var content []string
	if a.Image != "" {
		content = a.imageRows(inner, rows)
	} else {
		content = a.placeholderRows(inner, rows)
	}

	border := theme.BorderRounded.Border()
	frame := theme.Fg(lipgloss.NewStyle(), theme.Outline)

	top := frame.Render(border.TopLeft + strings.Repeat(border.Top, inner) + border.TopRight)
	bottom := frame.Render(border.BottomLeft + strings.Repeat(border.Bottom, inner) + border.BottomRight)

	lines := make([]string, 0, rows+2)
	lines = append(lines, top)
	for _, row := range content {
		lines = append(lines, frame.Render(border.Left)+row+frame.Render(border.Right))
	}
	lines = append(lines, bottom)

	return strings.Join(lines, "\n")
}

// imageRows returns the rows of the frame that carry the image.
//
// The image is clipped to the width of the frame and centred in it: the
// protocols render the cover at the size they were asked for, which is not
// necessarily the size of the column it ends up in.
func (a Artwork) imageRows(inner, rows int) []string {
	lines := strings.Split(a.Image, "\n")

	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		if i >= len(lines) {
			out = append(out, strings.Repeat(" ", inner))
			continue
		}
		line := lipgloss.NewStyle().MaxWidth(inner).Render(lines[i])
		out = append(out, centre(line, inner))
	}
	return out
}

// placeholderRows returns the rows of the frame that stand in for a missing
// cover.
//
// The placeholder is not an apology: it carries the name of the entry and
// the poster glyph, centred in the frame, so it still reads as a poster.
func (a Artwork) placeholderRows(inner, rows int) []string {
	fill := theme.SurfaceLow

	content := make([]string, 0, 3)
	content = append(content, centreText(theme.Icon(theme.IconPoster), inner, theme.Muted, fill))

	if rows > 1 {
		title := a.Title
		if title == "" {
			title = "No title"
		}
		content = append(content, centreText(title, inner, theme.Text, fill))
	}
	if rows > 2 {
		content = append(content, centreText("No cover art", inner, theme.Muted, fill))
	}
	if len(content) > rows {
		content = content[:rows]
	}

	// The block sits in the middle of the frame: a poster is not a caption
	// at the top of an empty box.
	pad := rows - len(content)
	top := pad / 2
	bottom := pad - top

	out := make([]string, 0, rows)
	for i := 0; i < top; i++ {
		out = append(out, filledLine(inner, fill))
	}
	out = append(out, content...)
	for i := 0; i < bottom; i++ {
		out = append(out, filledLine(inner, fill))
	}

	return out[:rows]
}

// centreText centres a text in the given width and styles it.
//
// The fill and the colour live on the same style: Lip Gloss closes a styled
// run with a reset, and a reset would clear the fill of the padding around
// the text.
func centreText(text string, width int, role, fill theme.Role) string {
	if lipgloss.Width(text) > width {
		text = truncate(text, width)
	}
	gap := width - lipgloss.Width(text)
	if gap < 0 {
		gap = 0
	}
	left := gap / 2
	right := gap - left

	style := theme.Fg(lipgloss.NewStyle(), role)
	if fill != theme.RoleUnset {
		style = theme.Bg(style, fill)
	}
	return style.Render(strings.Repeat(" ", left) + text + strings.Repeat(" ", right))
}

// centre centres a line in the given width.
//
// The line keeps its own escape sequences: an image brings its own colours
// and a reset in the middle of it would clear them.
func centre(line string, width int) string {
	w := lipgloss.Width(line)
	if w >= width {
		return line
	}
	gap := width - w
	left := gap / 2
	right := gap - left
	return strings.Repeat(" ", left) + line + strings.Repeat(" ", right)
}
