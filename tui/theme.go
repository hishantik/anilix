package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Theme holds all color constants for the TUI.
var Theme = struct {
	Primary   color.Color
	Secondary color.Color
	Border    color.Color
	Text      color.Color
	Faint     color.Color
	Success   color.Color
	Warning   color.Color
	Error     color.Color
	Gradient  []color.Color
}{
	Primary:   lipgloss.Color("#00d4ff"),
	Secondary: lipgloss.Color("#9d4edd"),
	Border:    lipgloss.Color("#444444"),
	Text:      lipgloss.Color("#e4e4e7"),
	Faint:     lipgloss.Color("#666666"),
	Success:   lipgloss.Color("#10b981"),
	Warning:   lipgloss.Color("#f59e0b"),
	Error:     lipgloss.Color("#ef4444"),
	Gradient: []color.Color{
		lipgloss.Color("#00d4ff"), // cyan
		lipgloss.Color("#00bfff"), // sky blue
		lipgloss.Color("#6366f1"), // indigo
		lipgloss.Color("#a855f7"), // violet
		lipgloss.Color("#d946ef"), // fuchsia
		lipgloss.Color("#f43f5e"), // rose
	},
}

func gradientTitle(text string) string {
	if len(text) == 0 || len(Theme.Gradient) == 0 {
		return text
	}
	colors := lipgloss.Blend1D(len(text), Theme.Gradient...)
	var result strings.Builder
	for i, ch := range text {
		result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colors[i]).Render(string(ch)))
	}
	return result.String()
}

// gradientLine renders a horizontal line with gradient colors.
func gradientLine(width int) string {
	if width <= 0 {
		return ""
	}
	colors := lipgloss.Blend1D(width, Theme.Gradient...)
	var result strings.Builder
	for i := 0; i < width; i++ {
		result.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Render("\u2500"))
	}
	return result.String()
}

// gradientPopupBox returns a gradient-bordered box using box-drawing chars.
// Both focused and unfocused cards MUST call this (with different borderColors)
// so geometry is always identical.
func gradientPopupBox(content string, width, paddingH int) string {
	innerWidth := width - 2 // subtract left + right border
	if innerWidth < 10 {
		innerWidth = 10
	}
	return cardBox(content, width, paddingH, Theme.Gradient)
}

// cardBox renders a rounded-border box with exact outer width = width.
// borderColors is the palette used for top/bottom gradient and side colors.
// Every call produces the same outer dimensions regardless of content.
func cardBox(content string, width, paddingH int, borderColors []color.Color) string {
	innerWidth := width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Build per-cell gradient for top/bottom bars
	cells := lipgloss.Blend1D(innerWidth+2, borderColors...)

	// Top border: ╭ ─ ╮
	var top strings.Builder
	top.WriteString(lipgloss.NewStyle().Foreground(cells[0]).Render("\u256d"))
	for i := 1; i <= innerWidth; i++ {
		top.WriteString(lipgloss.NewStyle().Foreground(cells[i]).Render("\u2500"))
	}
	top.WriteString(lipgloss.NewStyle().Foreground(cells[innerWidth+1]).Render("\u256e"))

	// Bottom border: ╰ ─ ╯
	var bottom strings.Builder
	bottom.WriteString(lipgloss.NewStyle().Foreground(cells[0]).Render("\u2570"))
	for i := 1; i <= innerWidth; i++ {
		bottom.WriteString(lipgloss.NewStyle().Foreground(cells[i]).Render("\u2500"))
	}
	bottom.WriteString(lipgloss.NewStyle().Foreground(cells[innerWidth+1]).Render("\u256f"))

	// Side borders (solid color — leftmost palette for left, rightmost for right)
	leftBorder := lipgloss.NewStyle().Foreground(borderColors[0]).Render("\u2502")
	rightBorder := lipgloss.NewStyle().Foreground(borderColors[len(borderColors)-1]).Render("\u2502")
	blankRow := leftBorder + strings.Repeat(" ", innerWidth) + rightBorder

	// Pad each content line to innerWidth so the right border aligns
	pad := strings.Repeat(" ", paddingH)
	contentLines := strings.Split(content, "\n")
	var rows []string
	rows = append(rows, top.String())
	rows = append(rows, blankRow) // top padding row
	available := innerWidth - paddingH
	if available < 0 {
		available = 0
	}
	for _, line := range contentLines {
		line = lipgloss.NewStyle().Width(available).MaxWidth(available).Render(line)
		rows = append(rows, leftBorder+pad+line+rightBorder)
	}
	rows = append(rows, blankRow) // bottom padding row
	rows = append(rows, bottom.String())

	return strings.Join(rows, "\n")
}

// solidCardBox renders a card with a solid-colored rounded border.
// Uses the same cardBox geometry so focused/unfocused dimensions match exactly.
func solidCardBox(content string, width, paddingH int, borderColor color.Color) string {
	return cardBox(content, width, paddingH, []color.Color{borderColor, borderColor, borderColor})
}

func renderSubDubSwitch(translationType string) string {
	isSub := translationType == "sub"

	onStyle := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true)
	offStyle := lipgloss.NewStyle().Foreground(Theme.Faint)

	// Gradient colors for the pill border
	gradColors := lipgloss.Blend1D(4, Theme.Gradient...)
	leftC := lipgloss.NewStyle().Foreground(gradColors[0])
	rightC := lipgloss.NewStyle().Foreground(gradColors[len(gradColors)-1])
	midC := lipgloss.NewStyle().Foreground(gradColors[len(gradColors)/2])

	// Single-line pill: ╭─ ● SUB ║ DUB ○ ─╮
	var body string
	if isSub {
		body = onStyle.Render("\u25CF") + " SUB " +
			midC.Render("\u2502") +
			" DUB " + offStyle.Render("\u25CB")
	} else {
		body = offStyle.Render("\u25CB") + " SUB " +
			midC.Render("\u2502") +
			" DUB " + onStyle.Render("\u25CF")
	}

	return leftC.Render("\u256D\u2500") + " " + body + " " + rightC.Render("\u2500\u256E")
}

// scoreBadge renders a score value in a compact gradient-bordered box.
func scoreBadge(score float64) string {
	content := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("\u2605 %.2f", score))
	return gradientPopupBox(content, 14, 2)
}

// infoBlock renders a label/value pair with uppercase faint label and primary value.
func infoBlock(label, value string) string {
	lbl := lipgloss.NewStyle().
		Foreground(Theme.Faint).
		Width(10).
		Render(strings.ToUpper(label))
	val := lipgloss.NewStyle().
		Foreground(Theme.Text).
		Render(value)
	return lbl + val
}

// genreTag renders a single genre as a compact colored badge.
func genreTag(genre string) string {
	return lipgloss.NewStyle().
		Foreground(Theme.Text).
		Background(Theme.Secondary).
		Padding(0, 1).
		Render(genre)
}

// coverPlaceholder renders a gradient-bordered frame with the anime title centered.
func coverPlaceholder(title string, width int) string {
	innerWidth := width - 4
	if innerWidth < 8 {
		innerWidth = 8
	}

	label := lipgloss.NewStyle().Faint(true).Align(lipgloss.Center).Width(innerWidth).Render("[ No Cover Art ]")
	nameLine := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Align(lipgloss.Center).
		Width(innerWidth).
		Render(truncateTitle(title, innerWidth))

	content := label + "\n\n" + nameLine
	return gradientPopupBox(content, width, 1)
}

// truncateTitle truncates a title to fit within maxLen characters.
func truncateTitle(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// progressBar renders a compact progress bar like ████░░░░ 67%.
func progressBar(ratio float64, width int) string {
	if width < 4 {
		width = 4
	}
	// Reserve 5 chars for " XX%"
	barWidth := width - 5
	if barWidth < 4 {
		barWidth = 4
	}
	filled := int(ratio * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	bar := lipgloss.NewStyle().Foreground(Theme.Primary).Render(strings.Repeat("\u2588", filled)) +
		lipgloss.NewStyle().Foreground(Theme.Border).Render(strings.Repeat("\u2591", empty))
	pct := fmt.Sprintf(" %3d%%", int(ratio*100))
	return bar + lipgloss.NewStyle().Foreground(Theme.Faint).Render(pct)
}

// statLine renders a label: value pair with consistent alignment for the sidebar.
func statLine(label, value string) string {
	lbl := lipgloss.NewStyle().Foreground(Theme.Faint).Render(label + ": ")
	val := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(value)
	return lbl + val
}
