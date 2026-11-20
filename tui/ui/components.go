// Package ui implements Anilix's component library: the reusable pieces the
// TUI screens are assembled from.
//
// Every component is a pure function of its data and a width. It never
// touches the model, never reads anything but the theme tokens, and always
// renders to a deterministic number of cells. That keeps the components
// testable in isolation and reusable across layouts.
//
// The styling rules themselves - colour roles, typography, spacing, shape -
// live in tui/theme. This package only composes them.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
)

// ButtonVariant is the visual weight of a button, mirroring Material 3's
// emphasis levels.
type ButtonVariant int

const (
	// ButtonFilled is the highest emphasis: a solid accent fill. Use it for
	// the single primary action of a screen.
	ButtonFilled ButtonVariant = iota
	// ButtonTonal is a muted accent fill. Use it for secondary actions.
	ButtonTonal
	// ButtonOutlined is a bordered, transparent button. Use it for
	// lower-emphasis actions that still need a hit target.
	ButtonOutlined
	// ButtonText is a label-only button. Use it for tertiary actions and
	// for destructive actions that must not look inviting.
	ButtonText
)

// buttonVariantName gives every variant a name for the showcase and the docs.
var buttonVariantName = map[ButtonVariant]string{
	ButtonFilled:   "filled",
	ButtonTonal:    "tonal",
	ButtonOutlined: "outlined",
	ButtonText:     "text",
}

// String returns the variant's name.
func (v ButtonVariant) String() string {
	if n, ok := buttonVariantName[v]; ok {
		return n
	}
	return "unknown"
}

// AllButtonVariants returns every variant in order.
func AllButtonVariants() []ButtonVariant {
	return []ButtonVariant{ButtonFilled, ButtonTonal, ButtonOutlined, ButtonText}
}

// Button is a clickable action.
//
// Buttons are always a single row tall: a terminal has one font size, so a
// bordered button uses left and right rules only and never grows a second
// row for its border.
type Button struct {
	// Label is the button text.
	Label string
	// Variant is the emphasis level.
	Variant ButtonVariant
	// State is the interaction state.
	State theme.State
	// Icon is an optional leading glyph. It must be one cell wide.
	Icon string
	// Shortcut is an optional trailing key hint, e.g. "ctrl+s".
	Shortcut string
	// Width, when set, makes the button exactly that many cells wide so
	// buttons in a row stay aligned.
	Width int
}

// Render renders the button.
//
// A focused button carries the selection bar in front of it. The bar is a
// shape, not a tint: a filled button is already the accent colour, so a
// focus expressed with colour alone would be invisible next to its hover
// state, and a text button's focus would vanish on terminals that quantise
// the two accent shades to the same palette entry. The bar is added outside
// the button width, so the label itself never moves.
func (b Button) Render() string {
	label := b.Label
	if b.Icon != "" {
		label = b.Icon + " " + label
	}

	var out string
	switch b.Variant {
	case ButtonFilled:
		out = b.renderFilled(label)
	case ButtonTonal:
		out = b.renderTonal(label)
	case ButtonOutlined:
		out = b.renderOutlined(label)
	default:
		out = b.renderText(label)
	}

	if b.State == theme.StateFocused {
		out = theme.Fg(lipgloss.NewStyle(), theme.Primary).
			Render(theme.Icon(theme.IconSelectionBar)) + " " + out
	}
	return out
}

// renderFilled renders the highest emphasis button: a solid accent fill.
func (b Button) renderFilled(label string) string {
	bg, fg := theme.Primary, theme.OnPrimary
	switch b.State {
	case theme.StateHover:
		bg = theme.PrimaryHover
	case theme.StatePressed:
		bg = theme.PrimaryPressed
	case theme.StateFocused:
		bg = theme.PrimaryHover
	case theme.StateDisabled:
		bg, fg = theme.SurfaceVariant, theme.Disabled
	}

	style := theme.Bg(theme.Fg(lipgloss.NewStyle().Bold(true), fg), bg).
		Padding(0, 1)
	if b.Width > 0 {
		style = style.Width(b.Width)
	}
	return b.withShortcut(style, label)
}

// renderTonal renders the medium emphasis button: a muted accent fill.
func (b Button) renderTonal(label string) string {
	bg, fg := theme.SecondaryContainer, theme.OnSecondaryContainer
	switch b.State {
	case theme.StateHover, theme.StatePressed:
		bg, fg = theme.Secondary, theme.OnSecondary
	case theme.StateFocused:
		bg, fg = theme.Secondary, theme.OnSecondary
	case theme.StateDisabled:
		bg, fg = theme.SurfaceVariant, theme.Disabled
	}

	style := theme.Bg(theme.Fg(lipgloss.NewStyle().Bold(true), fg), bg).
		Padding(0, 1)
	if b.Width > 0 {
		style = style.Width(b.Width)
	}
	return b.withShortcut(style, label)
}

// renderOutlined renders a bordered button.
//
// The border is drawn on the left and right only: a full box would need
// three rows and would break the inline layout buttons are used in.
func (b Button) renderOutlined(label string) string {
	border, fg := theme.Outline, theme.Primary
	switch b.State {
	case theme.StateHover:
		border, fg = theme.PrimaryHover, theme.PrimaryHover
	case theme.StatePressed:
		border, fg = theme.PrimaryPressed, theme.PrimaryPressed
	case theme.StateFocused:
		// The focus ring: the border carries the accent colour.
		border, fg = theme.Primary, theme.Primary
	case theme.StateDisabled:
		border, fg = theme.OutlineVariant, theme.Disabled
	}

	style := theme.Fg(lipgloss.NewStyle().Bold(true), fg).
		Border(theme.BorderRounded.Border(), false, true, false, true)
	if c := border.Color(); c != nil {
		style = style.BorderForeground(c)
	}
	if b.Width > 0 {
		style = style.Width(b.Width)
	}
	return b.withShortcut(style, label)
}

// renderText renders a label-only button.
func (b Button) renderText(label string) string {
	fg := theme.Primary
	switch b.State {
	case theme.StateHover:
		fg = theme.PrimaryHover
	case theme.StatePressed:
		fg = theme.PrimaryPressed
	case theme.StateFocused:
		fg = theme.Primary
	case theme.StateDisabled:
		fg = theme.Disabled
	}

	style := theme.Fg(lipgloss.NewStyle(), fg).Padding(0, 1)
	if b.Width > 0 {
		style = style.Width(b.Width)
	}
	return b.withShortcut(style, label)
}

// withShortcut appends the key hint to the label, dimmed.
//
// The hint is rendered last so the reset it carries never cancels the
// container colour of the button.
func (b Button) withShortcut(style lipgloss.Style, label string) string {
	if b.Shortcut == "" {
		return style.Render(label)
	}
	hint := theme.Caption.Render(" " + b.Shortcut)
	return style.Render(label + hint)
}

// Chip is a small tag: a genre, a status, a quality marker.
type Chip struct {
	// Label is the chip text.
	Label string
	// Container is the fill role. Use theme.RoleUnset for the default.
	Container theme.Role
	// State is the interaction state.
	State theme.State
}

// Render renders the chip.
func (c Chip) Render() string {
	container := c.Container
	if container == theme.RoleUnset {
		container = theme.PrimaryContainer
	}
	if c.State == theme.StateDisabled {
		return lipgloss.NewStyle().Faint(true).Render(c.Label)
	}
	return filledLabel(c.Label, container, false)
}

// filledLabel renders text on a filled surface.
//
// The padding and the label are styled separately, each with the fill
// colour: Lip Gloss closes every styled run with a reset, and a reset would
// clear the fill of whatever follows it.
func filledLabel(text string, container theme.Role, bold bool) string {
	label := theme.Fg(lipgloss.NewStyle(), onFor(container))
	if bold {
		label = label.Bold(true)
	}
	fill := theme.Bg(lipgloss.NewStyle(), container)
	return fill.Render(" ") +
		theme.Bg(label, container).Render(text) +
		fill.Render(" ")
}

// onFor returns the label role that belongs on a container role.
func onFor(container theme.Role) theme.Role {
	switch container {
	case theme.Primary:
		return theme.OnPrimary
	case theme.PrimaryContainer:
		return theme.OnPrimaryContainer
	case theme.Secondary:
		return theme.OnSecondary
	case theme.SecondaryContainer:
		return theme.OnSecondaryContainer
	case theme.Tertiary:
		return theme.OnTertiary
	case theme.TertiaryContainer:
		return theme.OnTertiaryContainer
	case theme.Error:
		return theme.OnError
	case theme.ErrorContainer:
		return theme.OnErrorContainer
	case theme.Success:
		return theme.OnSuccess
	case theme.Warning:
		return theme.OnWarning
	case theme.Surface, theme.SurfaceVariant, theme.SurfaceHigh, theme.SurfaceLow:
		return theme.Text
	}
	return theme.Text
}

// Badge is a count or status marker attached to a label.
type Badge struct {
	// Label is the badge text, usually a number.
	Label string
	// Container is the fill role. Use theme.RoleUnset for the default.
	Container theme.Role
}

// Render renders the badge.
func (b Badge) Render() string {
	container := b.Container
	if container == theme.RoleUnset {
		container = theme.SecondaryContainer
	}
	return filledLabel(b.Label, container, true)
}

// Card is a contained block of content.
type Card struct {
	// Title is the card headline.
	Title string
	// Subtitle is the secondary line under the title.
	Subtitle string
	// Body is the card content.
	Body string
	// Trailing is an optional trailing line, e.g. a timestamp.
	Trailing string
	// Elevation is the surface tier the card sits on.
	Elevation theme.Elevation
	// Width is the *outer* width: the border and the padding are subtracted
	// from the content box, so a card of width 44 renders exactly 44 cells.
	Width int
}

// Render renders the card.
func (c Card) Render() string {
	style := theme.Bg(lipgloss.NewStyle().
		Padding(0, 1).
		Border(theme.BorderRounded.Border(), true), c.Elevation.Surface())
	if col := c.Elevation.Border().Color(); col != nil {
		style = style.BorderForeground(col)
	}

	lines := make([]string, 0, 4)
	if c.Title != "" {
		lines = append(lines, theme.Title.Render(c.Title))
	}
	if c.Subtitle != "" {
		lines = append(lines, theme.Label.Render(c.Subtitle))
	}
	if c.Body != "" {
		lines = append(lines, theme.Body.Render(c.Body))
	}
	if c.Trailing != "" {
		lines = append(lines, theme.Caption.Render(c.Trailing))
	}

	content := strings.Join(lines, "\n")
	if c.Width > 0 {
		// The border counts towards the width, the padding does not: the
		// content box is the width minus the border and the padding.
		content = lipgloss.NewStyle().Width(c.Width - 4).Render(content)
		style = style.Width(c.Width)
	}
	return style.Render(content)
}

// MenuItem is a row in a menu, drawer or settings list.
type MenuItem struct {
	// Label is the row text.
	Label string
	// Hint is the right-aligned secondary text, e.g. a count or a shortcut.
	Hint string
	// Icon is the leading glyph. It must be one cell wide.
	Icon string
	// Selected marks the row as the current one.
	Selected bool
	// Focused reports whether the menu holds keyboard focus.
	Focused bool
	// Disabled marks the row as unavailable.
	Disabled bool
	// Active marks the row as the destination of the current page.
	//
	// It is different from Selected: Selected is the cursor row, Active is
	// the page the user is on. The active row keeps its accent tint even
	// when the menu does not hold the focus.
	Active bool
	// Fill is the surface the row sits on. Use theme.RoleUnset for the
	// terminal background.
	Fill theme.Role
}

// Render renders the menu item.
//
// The selected row carries an accent bar; when the menu does not hold
// keyboard focus the bar stays but the row is muted, so "here" and "active"
// never look the same.
func (m MenuItem) Render(width int) string {
	if width < 1 {
		width = 1
	}
	icon := m.Icon
	if icon == "" {
		icon = theme.Icon(theme.IconMenuItem)
	}

	// The marker, the icon and the space take the first four cells; the
	// label and the hint share the rest.
	const prefix = 4
	room := width - prefix
	if room < 1 {
		room = 1
	}

	label := m.Label
	if lipgloss.Width(label) > room {
		label = truncate(label, room)
	}

	if m.Disabled {
		return styledLine([]segment{
			{text: "  ", role: theme.RoleUnset, faint: true},
			{text: icon, role: theme.Disabled, faint: true},
			{text: " ", role: theme.RoleUnset, faint: true},
			{text: label, role: theme.Disabled, faint: true},
		}, width, m.Fill)
	}

	hint := m.Hint
	if hint != "" && lipgloss.Width(label)+lipgloss.Width(hint)+1 > room {
		// The hint is the first thing to go: the label matters more.
		hint = ""
	}

	markerRole := theme.RoleUnset
	labelRole := theme.Text
	iconRole := theme.RoleUnset
	marker := "  "
	if m.Active {
		// The current page keeps its accent tint at all times.
		iconRole = theme.Primary
		labelRole = theme.Primary
	}
	if m.Selected {
		marker = theme.Icon(theme.IconSelectionBar) + " "
		markerRole = theme.Primary
		if m.Focused {
			labelRole = theme.Primary
		} else if !m.Active {
			labelRole = theme.Muted
		}
	}

	segs := []segment{
		{text: marker, role: markerRole},
		{text: icon, role: iconRole},
		{text: " ", role: theme.RoleUnset},
		{text: label, role: labelRole},
	}
	if hint != "" {
		gap := width - segmentWidth(segs) - lipgloss.Width(hint)
		if gap < 1 {
			gap = 1
		}
		segs = append(segs,
			segment{text: strings.Repeat(" ", gap), role: theme.RoleUnset},
			segment{text: hint, role: theme.Disabled},
		)
	}
	return styledLine(segs, width, m.Fill)
}

// truncate shortens a string to the given number of cells.
//
// The ellipsis is plain ASCII so it survives a terminal without unicode.
func truncate(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return string([]rune(s)[:width])
	}
	return string([]rune(s)[:width-3]) + "..."
}

// Truncate shortens a string to the given number of cells.
//
// The screens use it before they hand a title to a component: a title that
// is longer than its column has to be cut, not wrapped.
func Truncate(s string, width int) string {
	return truncate(s, width)
}

// segmentWidth returns the rendered width of the segments.
func segmentWidth(segs []segment) int {
	width := 0
	for _, s := range segs {
		width += lipgloss.Width(s.text)
	}
	return width
}

// Tab is one entry of a tab row.
type Tab struct {
	// Label is the tab text.
	Label string
	// Selected marks the active tab.
	Selected bool
	// Badge is an optional count shown next to the label.
	Badge string
}

// Render renders the tab.
func (t Tab) Render() string {
	label := theme.Fg(lipgloss.NewStyle(), theme.Muted)
	marker := "  "
	if t.Selected {
		label = theme.Fg(lipgloss.NewStyle().Bold(true), theme.Primary)
		marker = theme.Icon(theme.IconTabActive) + " "
	}
	out := marker + label.Render(t.Label)
	if t.Badge != "" {
		out += " " + theme.Caption.Render("("+t.Badge+")")
	}
	return out
}

// TabBar renders a row of tabs.
func TabBar(tabs []Tab) string {
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		parts = append(parts, tab.Render())
	}
	return strings.Join(parts, "  ")
}

// Dialog is a modal container.
type Dialog struct {
	// Title is the dialog headline.
	Title string
	// Body is the dialog content.
	Body string
	// Actions are the already-rendered action buttons.
	Actions []string
	// Width is the *outer* width: border and padding are subtracted from
	// the content box, so the dialog never exceeds the width it was given.
	Width int
	// Elevation is the surface tier the dialog sits on.
	Elevation theme.Elevation
}

// Render renders the dialog.
func (d Dialog) Render() string {
	style := theme.Bg(lipgloss.NewStyle().
		Padding(1, 2).
		Border(theme.BorderRounded.Border(), true), d.Elevation.Surface())
	if col := d.Elevation.Border().Color(); col != nil {
		style = style.BorderForeground(col)
	}

	lines := make([]string, 0, 3)
	if d.Title != "" {
		lines = append(lines, theme.Headline.Render(d.Title))
	}
	if d.Body != "" {
		lines = append(lines, theme.Body.Render(d.Body))
	}
	if len(d.Actions) > 0 {
		// The actions are already-rendered buttons: restyling them here
		// would strip the fill of the focused one.
		lines = append(lines, strings.Join(d.Actions, "   "))
	}

	inner := d.Width - 6 // borders (2) + padding (4)
	if inner < 10 {
		inner = 10
	}
	content := lipgloss.NewStyle().Width(inner).Render(strings.Join(lines, "\n"))
	if d.Width > 0 {
		style = style.Width(d.Width)
	}
	return style.Render(content)
}

// ListItem is a row in a result list.
type ListItem struct {
	// Title is the row headline.
	Title string
	// Subtitle is the alternative title, shown next to the headline on the
	// same line. It is dropped when the row is too narrow for it: the
	// headline always wins.
	Subtitle string
	// Description is the secondary line under the title.
	Description string
	// Trailing is the right-aligned metadata, e.g. a score.
	Trailing string
	// TrailingRole is the colour of the trailing value. Use theme.RoleUnset
	// for the default.
	TrailingRole theme.Role
	// Selected marks the row as the cursor row.
	Selected bool
	// Focused reports whether the list holds keyboard focus.
	Focused bool
	// Disabled marks the row as unavailable.
	Disabled bool
}

// Render renders the list item.
//
// Selection is marked by a leading accent bar and the primary colour.
// Focus adds the surface fill; when the list loses focus the row keeps its
// bar but is muted, which matters because the list and the preview panel
// have separate focus.
func (l ListItem) Render(width int) string {
	if width < 1 {
		width = 1
	}
	if l.Disabled {
		return lipgloss.NewStyle().Faint(true).Width(width).
			Render("  " + l.Title)
	}

	marker := "  "
	markerRole := theme.RoleUnset
	titleRole := theme.Text
	subtitleRole := theme.Muted
	trailingRole := theme.Muted
	fill := theme.RoleUnset

	switch {
	case l.Selected && l.Focused:
		marker = theme.Icon(theme.IconSelectionBar) + " "
		markerRole = theme.Primary
		titleRole = theme.Primary
		subtitleRole = theme.Muted
		trailingRole = theme.Primary
		fill = theme.SurfaceVariant
	case l.Selected:
		marker = theme.Icon(theme.IconSelectionBar) + " "
		markerRole = theme.Muted
		titleRole = theme.Muted
		subtitleRole = theme.Muted
		trailingRole = theme.Muted
		fill = theme.Surface
	default:
		if l.TrailingRole != theme.RoleUnset {
			trailingRole = l.TrailingRole
		}
	}

	title, subtitle := l.fitTitle(width)

	rows := make([]string, 0, 2)
	head := []segment{
		{text: marker, role: markerRole},
		{text: title, role: titleRole, bold: l.Selected},
	}
	if subtitle != "" {
		head = append(head,
			segment{text: "  ", role: theme.RoleUnset},
			segment{text: subtitle, role: subtitleRole},
		)
	}
	if l.Trailing != "" {
		gap := width - segmentWidth(head) - lipgloss.Width(l.Trailing)
		if gap < 1 {
			gap = 1
		}
		head = append(head,
			segment{text: strings.Repeat(" ", gap), role: theme.RoleUnset},
			segment{text: l.Trailing, role: trailingRole},
		)
	}
	rows = append(rows, styledLine(head, width, fill))

	if l.Description != "" {
		rows = append(rows, styledLine([]segment{
			{text: "  ", role: theme.RoleUnset},
			{text: truncate(l.Description, width-2), role: theme.Muted},
		}, width, fill))
	}

	return strings.Join(rows, "\n")
}

// fitTitle returns the title and the subtitle that fit on the headline line.
//
// The marker and the trailing value take their share of the row first; what
// is left belongs to the headline. The alternative title is the first thing
// to go when the row is narrow: the headline is what identifies the entry.
func (l ListItem) fitTitle(width int) (title, subtitle string) {
	const prefix = 2 // the marker and the space after it

	room := width - prefix
	if room < 1 {
		room = 1
	}
	if l.Trailing != "" {
		room -= lipgloss.Width(l.Trailing) + 1
	}
	if room < 1 {
		room = 1
	}

	title = l.Title
	subtitle = l.Subtitle

	if subtitle != "" {
		needed := lipgloss.Width(subtitle) + 2
		if room-needed < 8 {
			// There is no room for both: the headline wins.
			subtitle = ""
		} else {
			title = truncate(title, room-needed)
		}
	}
	if subtitle == "" {
		title = truncate(title, room)
	}

	return title, subtitle
}

// Divider renders a horizontal rule.
//
// Two weights: subtle (outline-variant) between groups, strong (outline)
// between major sections.
func Divider(strong bool, width int) string {
	if width <= 0 {
		return ""
	}
	role := theme.OutlineVariant
	if strong {
		role = theme.Outline
	}
	line := theme.Icon(theme.IconDivider)
	return theme.Fg(lipgloss.NewStyle(), role).Render(strings.Repeat(line, width))
}

// SectionHeader renders an eyebrow label with an optional trailing value.
//
// The trailing value is aligned to the right edge of the given width; pass
// zero to let it follow the label directly.
func SectionHeader(label, trailing string, width int) string {
	out := theme.Overline.RenderUpper(label)
	if trailing == "" {
		return out
	}
	if width <= 0 {
		return out + " " + theme.Caption.Render(trailing)
	}
	pad := width - lipgloss.Width(out) - lipgloss.Width(trailing)
	if pad < 1 {
		pad = 1
	}
	return out + strings.Repeat(" ", pad) + theme.Caption.Render(trailing)
}

// KeyHint renders a keyboard hint, e.g. "enter play".
func KeyHint(key, action string) string {
	return theme.Fg(lipgloss.NewStyle().Bold(true), theme.Muted).Render(key) + " " +
		theme.Caption.Render(action)
}

// Row joins inline elements with a fixed gap.
//
// It is the one place the horizontal rhythm of a screen is decided, so rows
// of buttons or chips always share the same spacing.
func Row(items []string, gap theme.Space) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, theme.Gap(gap.Cells()))
}

// segment is a run of text with a single foreground role.
type segment struct {
	text  string
	role  theme.Role
	bold  bool
	faint bool
}

// styledLine renders the segments of one line on a filled background and pads
// the line to exactly width cells.
//
// Segments are styled one by one on purpose. Lip Gloss closes every styled
// run with a reset, and a reset also clears the background colour: nesting a
// coloured run inside a filled block would leave a hole in the fill. Styling
// each segment with the fill as well keeps the surface continuous.
func styledLine(segs []segment, width int, fill theme.Role) string {
	used := 0
	for _, s := range segs {
		used += lipgloss.Width(s.text)
	}
	pad := width - used
	if pad < 0 {
		pad = 0
	}

	parts := make([]string, 0, len(segs)+1)
	for _, s := range segs {
		parts = append(parts, styleSegment(s.text, s.role, fill, s.bold, s.faint))
	}
	if pad > 0 {
		parts = append(parts, styleSegment(strings.Repeat(" ", pad), theme.RoleUnset, fill, false, false))
	}
	return strings.Join(parts, "")
}

// styleSegment renders text with a foreground role on an optional fill.
func styleSegment(text string, role, fill theme.Role, bold, faint bool) string {
	style := lipgloss.NewStyle()
	if bold {
		style = style.Bold(true)
	}
	if faint {
		style = style.Faint(true)
	}
	if role != theme.RoleUnset {
		style = theme.Fg(style, role)
	}
	if fill != theme.RoleUnset {
		style = theme.Bg(style, fill)
	}
	return style.Render(text)
}

// filledLine renders an empty row on a filled surface.
//
// It keeps the fill of a column continuous: a blank line without the fill
// would show the terminal background as a gap in the surface.
func filledLine(width int, fill theme.Role) string {
	if width < 1 {
		width = 1
	}
	return styleSegment(strings.Repeat(" ", width), theme.RoleUnset, fill, false, false)
}
