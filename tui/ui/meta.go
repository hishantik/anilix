package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// InfoRow renders a label and a value on one line.
//
// The label column has a fixed width, so a block of metadata reads as a
// table: the values line up under each other instead of drifting with the
// length of the labels.
func InfoRow(label, value string) string {
	const labelWidth = 12

	lbl := theme.Label.Render(fmt.Sprintf("%-*s", labelWidth, label))
	if value == "" {
		return lbl
	}
	return lbl + theme.BodyStrong.Render(value)
}

// ChipRow renders a list of chips, wrapped to the given width.
//
// It returns one string per row: the caller decides where the rows go.
func ChipRow(labels []string, width int, container theme.Role) []string {
	if len(labels) == 0 {
		return nil
	}

	rows := make([]string, 0, len(labels))
	current := ""

	for _, label := range labels {
		chip := Chip{Label: label, Container: container}.Render()

		candidate := chip
		if current != "" {
			candidate = current + " " + chip
		}
		if width > 0 && lipgloss.Width(candidate) > width && current != "" {
			rows = append(rows, current)
			current = chip
			continue
		}
		current = candidate
	}

	if current != "" {
		rows = append(rows, current)
	}
	return rows
}
