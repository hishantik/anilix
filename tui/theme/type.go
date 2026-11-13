package theme

import (
	"charm.land/lipgloss/v2"
)

// Type is a step on the typography scale.
//
// Terminals render a single font size, so the scale expresses hierarchy with
// weight, colour, case and spacing instead of point size. Every screen uses
// these steps so that "a title" always looks the same.
type Type int

const (
	// Display is the brand wordmark in the app bar.
	Display Type = iota
	// Headline is a screen-level title.
	Headline
	// Title is a section or item title.
	Title
	// BodyStrong is emphasised body text.
	BodyStrong
	// Body is default reading text.
	Body
	// Label is metadata text paired with a value.
	Label
	// LabelStrong is an emphasised metadata label.
	LabelStrong
	// Caption is the smallest text: hints, timestamps, counts.
	Caption
	// Overline is an uppercase section eyebrow.
	Overline
	// Code is monospaced technical text (URLs, ids).
	Code
)

// typeName gives every step a name for the showcase and the docs.
var typeName = map[Type]string{
	Display:     "display",
	Headline:    "headline",
	Title:       "title",
	BodyStrong:  "body-strong",
	Body:        "body",
	Label:       "label",
	LabelStrong: "label-strong",
	Caption:     "caption",
	Overline:    "overline",
	Code:        "code",
}

// String returns the step's name.
func (t Type) String() string {
	if n, ok := typeName[t]; ok {
		return n
	}
	return "unknown"
}

// AllTypes returns every step of the scale in order.
func AllTypes() []Type {
	return []Type{Display, Headline, Title, BodyStrong, Body, Label, LabelStrong, Caption, Overline, Code}
}

// Style returns the lipgloss style for a typography step.
//
//	Display     bold, primary      - brand wordmark
//	Headline    bold, text         - screen titles
//	Title       bold, text         - item and section titles
//	BodyStrong  bold, text         - emphasised body
//	Body        text               - default reading text
//	Label       muted              - metadata labels
//	LabelStrong bold, muted        - emphasised labels
//	Caption     faint, disabled    - hints, timestamps
//	Overline    bold, primary, caps- section eyebrows
//	Code        muted, no wrap      - technical values
func (t Type) Style() lipgloss.Style {
	switch t {
	case Display:
		return Fg(lipgloss.NewStyle().Bold(true), Primary)
	case Headline:
		return Fg(lipgloss.NewStyle().Bold(true), Text)
	case Title:
		return Fg(lipgloss.NewStyle().Bold(true), Text)
	case BodyStrong:
		return Fg(lipgloss.NewStyle().Bold(true), Text)
	case Body:
		return Fg(lipgloss.NewStyle(), Text)
	case Label:
		return Fg(lipgloss.NewStyle(), Muted)
	case LabelStrong:
		return Fg(lipgloss.NewStyle().Bold(true), Muted)
	case Caption:
		return Fg(lipgloss.NewStyle().Faint(true), Disabled)
	case Overline:
		return Fg(lipgloss.NewStyle().Bold(true), Primary)
	case Code:
		return Fg(lipgloss.NewStyle(), Muted)
	}
	return lipgloss.NewStyle()
}

// Render renders text at a step of the scale.
func (t Type) Render(s string) string {
	return t.Style().Render(s)
}

// In returns the style of a step of the scale with the foreground replaced
// by the given role.
//
// It is used when a component has to keep the weight and case of a type step
// but needs a different colour - a title inside a selected row, for example.
func (t Type) In(role Role) lipgloss.Style {
	if role == RoleUnset {
		return t.Style()
	}
	return Fg(t.Style(), role)
}

// RenderIn renders text at a step of the scale in the given colour.
func (t Type) RenderIn(s string, role Role) string {
	return t.In(role).Render(s)
}

// RenderUpper renders text at a step of the scale in upper case, for the
// eyebrow and label treatments.
func (t Type) RenderUpper(s string) string {
	return t.Style().Render(upper(s))
}

// upper upper-cases ASCII without allocating for already-upper strings.
func upper(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		out = append(out, r)
	}
	return string(out)
}
