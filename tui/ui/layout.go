package ui

// Breakpoint is the layout tier a screen renders at.
//
// The tiers come from the width of the terminal, and they are the only
// place in the codebase that decides how wide a screen is: every layout
// asks for a breakpoint instead of comparing raw column counts.
type Breakpoint int

const (
	// Compact is a single column layout for narrow terminals, typically a
	// phone in Termux.
	Compact Breakpoint = iota
	// Medium is a two column layout with a narrow preview.
	Medium
	// Expanded is a two column layout with a wide preview.
	Expanded
)

// breakpointName gives every tier a name for the showcase and the docs.
var breakpointName = map[Breakpoint]string{
	Compact:  "compact",
	Medium:   "medium",
	Expanded: "expanded",
}

// String returns the tier's name.
func (b Breakpoint) String() string {
	if n, ok := breakpointName[b]; ok {
		return n
	}
	return "unknown"
}

// AllBreakpoints returns every tier in order.
func AllBreakpoints() []Breakpoint {
	return []Breakpoint{Compact, Medium, Expanded}
}

// BreakpointOf returns the layout tier for a terminal width.
//
// The thresholds follow the Phase 0 audit: below 60 columns a two column
// layout is unusable, and below 100 columns the preview panel has to stay
// narrow.
func BreakpointOf(width int) Breakpoint {
	switch {
	case width < 60:
		return Compact
	case width < 100:
		return Medium
	default:
		return Expanded
	}
}

// IsCompact reports whether the tier renders a single column.
func (b Breakpoint) IsCompact() bool {
	return b == Compact
}

// IsTwoColumn reports whether the tier renders a list and a side panel.
func (b Breakpoint) IsTwoColumn() bool {
	return b != Compact
}

// PreviewWidth returns the width of the side panel for a terminal width.
//
// Compact terminals have no side panel at all and get zero.
func PreviewWidth(total int) int {
	switch BreakpointOf(total) {
	case Compact:
		return 0
	case Medium:
		w := total * 40 / 100
		if w < 28 {
			w = 28
		}
		return w
	default:
		w := total * 45 / 100
		if w < 36 {
			w = 36
		}
		if w > 80 {
			w = 80
		}
		return w
	}
}

// ListWidth returns the width of the result list for a terminal width.
//
// The list always keeps at least 30 cells; when the terminal is too narrow
// for a side panel the list takes the whole width.
func ListWidth(total int) int {
	preview := PreviewWidth(total)
	if preview == 0 {
		return clamp(total, 20, total)
	}
	width := total - preview - PanelGutter()
	if width < 30 {
		return clamp(total, 20, total)
	}
	return width
}

// PanelGutter returns the number of cells between two panels.
//
// The gutter carries the vertical rule that separates the results from the
// preview: a space, the rule and a space. A screen never decides the gap
// between two columns on its own.
func PanelGutter() int {
	return 3
}

// OverlayWidth returns the width of a modal for a terminal width.
//
// Overlays are clamped to the terminal so a dialog never wraps on a narrow
// Termux screen: the classic 44 column dialog becomes 36 on a phone.
func OverlayWidth(desired, total int) int {
	limit := total - 4
	if limit < 20 {
		limit = 20
	}
	if desired > limit {
		desired = limit
	}
	if desired < 20 {
		desired = 20
	}
	return desired
}

// ContentWidth returns the width available for content inside a terminal,
// after the screen padding is removed.
func ContentWidth(total int) int {
	return clamp(total-2*spaceScreen, 20, total)
}

// SupportsPersistentDrawer reports whether the terminal is wide enough for a
// persistent navigation sidebar.
//
// The rule follows the breakpoint table: a compact terminal is a phone
// screen, and a phone screen has no room for a sidebar. Every such terminal
// gets the drawer as a modal overlay instead.
func SupportsPersistentDrawer(total int) bool {
	return BreakpointOf(total) != Compact
}

// DrawerWidth returns the width of the persistent navigation sidebar.
//
// It returns zero when the terminal is too narrow for a sidebar, which is
// the signal for the shell to render the drawer as a modal overlay. The
// sidebar never squeezes the content below 36 columns: a content area that
// narrow cannot show a title and a metadata line at the same time.
func DrawerWidth(total int) int {
	if !SupportsPersistentDrawer(total) {
		return 0
	}
	w := 22
	if total < 80 {
		w = 20
	}
	if max := total - minContentWidth; w > max {
		w = max
	}
	if w < minDrawerWidth {
		w = minDrawerWidth
	}
	return w
}

// OverlayDrawerWidth returns the width of the modal navigation drawer.
//
// The drawer is clamped to the terminal so it never wraps, not even on a
// phone screen.
func OverlayDrawerWidth(total int) int {
	return OverlayWidth(30, total)
}

// minContentWidth is the narrowest content area the shell allows next to a
// persistent sidebar.
const minContentWidth = 36

// minDrawerWidth is the narrowest usable navigation sidebar.
const minDrawerWidth = 18

// spaceScreen is the horizontal screen padding, in cells, on each side.
const spaceScreen = 4

// clamp restricts a value to the [lo, hi] range.
func clamp(value, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}
