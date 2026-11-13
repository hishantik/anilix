package theme

import (
	"charm.land/lipgloss/v2"
)

// Space is a step on the 4dp spacing scale, measured in terminal cells.
//
// Terminals have no fractional spacing, so the scale is rounded to whole
// cells that still follow a 4dp rhythm: 1, 2, 4, 8, 12, 16, 24, 32.
type Space int

const (
	// Space0 is no spacing.
	Space0 Space = iota
	// Space1 is 1 cell - inside a single line.
	Space1
	// Space2 is 2 cells - between related lines.
	Space2
	// Space3 is 4 cells - between blocks.
	Space3
	// Space4 is 8 cells - between sections.
	Space4
	// Space5 is 12 cells - between major groups.
	Space5
	// Space6 is 16 cells - screen padding.
	Space6
	// Space7 is 24 cells - generous separation.
	Space7
	// Space8 is 32 cells - hero spacing.
	Space8
)

// spaceSize holds the cell count of every step.
var spaceSize = map[Space]int{
	Space0: 0,
	Space1: 1,
	Space2: 2,
	Space3: 4,
	Space4: 8,
	Space5: 12,
	Space6: 16,
	Space7: 24,
	Space8: 32,
}

// Cells returns the number of cells a spacing step occupies.
func (s Space) Cells() int {
	if n, ok := spaceSize[s]; ok {
		return n
	}
	return 0
}

// String returns the spacing step as a token, e.g. "space4".
func (s Space) String() string {
	switch s {
	case Space0:
		return "space0"
	case Space1:
		return "space1"
	case Space2:
		return "space2"
	case Space3:
		return "space3"
	case Space4:
		return "space4"
	case Space5:
		return "space5"
	case Space6:
		return "space6"
	case Space7:
		return "space7"
	case Space8:
		return "space8"
	}
	return "space?"
}

// AllSpaces returns every spacing step in order.
func AllSpaces() []Space {
	return []Space{Space0, Space1, Space2, Space3, Space4, Space5, Space6, Space7, Space8}
}

// Padding returns the horizontal padding for a spacing step.
func (s Space) Padding() (int, int) {
	n := s.Cells()
	return n, n
}

// Margin returns the top and bottom margin for a spacing step.
func (s Space) Margin() (int, int) {
	n := s.Cells()
	return n, n
}

// Gap returns a run of n spaces, used between inline elements.
func Gap(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = ' '
	}
	return string(out)
}

// Elevation is the surface tier a container sits on.
//
// A terminal has no shadows, so elevation is expressed as a surface tint
// plus a stronger border: higher tiers are lighter and more separated.
type Elevation int

const (
	// Elevation0 is flush with the background.
	Elevation0 Elevation = iota
	// Elevation1 is a card on the background.
	Elevation1
	// Elevation2 is a selected row or raised control.
	Elevation2
	// Elevation3 is a dialog, drawer or menu.
	Elevation3
)

// Surface returns the surface role for an elevation level.
func (e Elevation) Surface() Role {
	switch e {
	case Elevation1:
		return Surface
	case Elevation2:
		return SurfaceVariant
	case Elevation3:
		return SurfaceHigh
	}
	return Background
}

// Border returns the border role for an elevation level.
func (e Elevation) Border() Role {
	switch e {
	case Elevation1:
		return OutlineVariant
	case Elevation2:
		return Outline
	case Elevation3:
		return Outline
	}
	return OutlineVariant
}

// Size is a component sizing rule.
type Size int

const (
	// SizeCompact is the dense size for list rows and chips.
	SizeCompact Size = iota
	// SizeMedium is the default size for controls.
	SizeMedium
	// SizeExpanded is the comfortable size for dialogs and panes.
	SizeExpanded
)

// sizeName gives every size a name for the showcase and the docs.
var sizeName = map[Size]string{
	SizeCompact:  "compact",
	SizeMedium:   "medium",
	SizeExpanded: "expanded",
}

// String returns the size's name.
func (s Size) String() string {
	if n, ok := sizeName[s]; ok {
		return n
	}
	return "unknown"
}

// AllSizes returns every size in order.
func AllSizes() []Size {
	return []Size{SizeCompact, SizeMedium, SizeExpanded}
}

// ControlHeight returns the rendered height in cells of a control size.
func (s Size) ControlHeight() int {
	switch s {
	case SizeCompact:
		return 1
	case SizeMedium:
		return 3
	case SizeExpanded:
		return 5
	}
	return 3
}

// BorderStyle is the shape treatment of a container.
//
// A terminal has no corner radius, so the "shape scale" is expressed with
// the box drawing characters a container is drawn with.
type BorderStyle int

const (
	// BorderNone draws no border.
	BorderNone BorderStyle = iota
	// BorderThin is a square box (─ │ ┌ ┐ └ ┘).
	BorderThin
	// BorderRounded is a rounded box (╭ ─ ╮).
	BorderRounded
	// BorderFocus is a left accent bar, used to mark the focused row.
	BorderFocus
)

// borderName gives every border style a name for the showcase and the docs.
var borderName = map[BorderStyle]string{
	BorderNone:    "none",
	BorderThin:    "thin",
	BorderRounded: "rounded",
	BorderFocus:   "focus-bar",
}

// String returns the border style's name.
func (b BorderStyle) String() string {
	if n, ok := borderName[b]; ok {
		return n
	}
	return "unknown"
}

// AllBorderStyles returns every border style in order.
func AllBorderStyles() []BorderStyle {
	return []BorderStyle{BorderNone, BorderThin, BorderRounded, BorderFocus}
}

// Border returns the lipgloss border for the style, degrading to
// ASCII when the terminal cannot render box drawing characters.
func (b BorderStyle) Border() lipgloss.Border {
	switch b {
	case BorderThin:
		if HasUnicode() {
			return lipgloss.NormalBorder()
		}
		return lipgloss.Border{
			Top: "-", Bottom: "-", Left: "|", Right: "|",
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		}
	case BorderRounded:
		if HasUnicode() {
			return lipgloss.RoundedBorder()
		}
		return lipgloss.Border{
			Top: "-", Bottom: "-", Left: "|", Right: "|",
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		}
	case BorderFocus:
		// A single left bar; handled by the caller, kept for completeness.
		return lipgloss.Border{}
	}
	return lipgloss.Border{}
}
