// Package theme implements Anilix's visual design system: semantic color
// tokens, a typography scale, spacing and shape rules, and the reusable
// component styles the TUI renders with.
//
// The palette follows Material Design 3's dark theme role model (surface /
// onSurface, primary / onPrimary, ...) using the MD3 baseline colours, and
// is tuned for a dark terminal. Terminals have a single font size, so
// hierarchy is expressed with weight, colour, case and spacing rather than
// size.
//
// Colours degrade for terminals with less capability: the detected
// colorprofile is used to downsample truecolor values to 256 or 16 colours,
// and terminals without colour support fall back to typography alone.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Role identifies a semantic colour in the design system.
//
// Roles are intentionally named after their *purpose*, never their hue, so a
// screen never references "purple" but always "primary".
type Role int

const (
	// RoleUnset is the zero value: no role chosen.
	//
	// Components with an optional colour (chips, badges, list markers) use
	// it instead of relying on a "magic" colour, so the zero value of a
	// struct field always means "unset".
	RoleUnset Role = iota
	// Background is the application canvas behind every surface.
	Background
	// Surface is the base elevation for panels and cards.
	Surface
	// SurfaceVariant is the elevated tone used for selected rows and chips.
	SurfaceVariant
	// SurfaceHigh is the highest elevation: dialogs, drawers, menus.
	SurfaceHigh
	// SurfaceLow is a recessed tone, used for wells and input fields.
	SurfaceLow
	// Outline is the border colour of containers.
	Outline
	// OutlineVariant is the subtle divider colour between rows.
	OutlineVariant
	// Primary is the accent colour: focus, selection, primary actions.
	Primary
	// OnPrimary is the text colour placed on Primary.
	OnPrimary
	// PrimaryHover is the accent colour in its hovered state.
	PrimaryHover
	// PrimaryPressed is the accent colour in its pressed state.
	PrimaryPressed
	// PrimaryContainer is a muted primary fill for chips and badges.
	PrimaryContainer
	// OnPrimaryContainer is the text colour on PrimaryContainer.
	OnPrimaryContainer
	// Secondary is the supporting accent, used for less important accents.
	Secondary
	// OnSecondary is the text colour placed on Secondary.
	OnSecondary
	// SecondaryContainer is a muted secondary fill.
	SecondaryContainer
	// OnSecondaryContainer is the text colour on SecondaryContainer.
	OnSecondaryContainer
	// Tertiary is a third accent for balanced emphasis.
	Tertiary
	// OnTertiary is the text colour placed on Tertiary.
	OnTertiary
	// TertiaryContainer is a muted tertiary fill.
	TertiaryContainer
	// OnTertiaryContainer is the text colour on TertiaryContainer.
	OnTertiaryContainer
	// Error is used for failures and destructive actions.
	Error
	// OnError is the text colour placed on Error.
	OnError
	// ErrorContainer is a muted error fill.
	ErrorContainer
	// OnErrorContainer is the text colour on ErrorContainer.
	OnErrorContainer
	// Success is used for confirmations and connected states.
	Success
	// OnSuccess is the text colour placed on Success.
	OnSuccess
	// Warning is used for cautionary states.
	Warning
	// OnWarning is the text colour placed on Warning.
	OnWarning
	// Text is the primary text colour.
	Text
	// Muted is the secondary text colour.
	Muted
	// Disabled is the text colour for unavailable content.
	Disabled
	// Scrim dims the background behind a modal.
	Scrim
	// roleCount is the number of defined roles; keep last.
	roleCount
)

// hex holds the canonical (truecolor) value of every role.
//
// The base values are the reference palette; the derived ones are computed
// from it so the system stays internally consistent when the base moves.
var hex = map[Role]string{
	// Base surfaces — Material Design 3 dark theme
	Background:     "#111318",
	Surface:        "#1C2028",
	SurfaceVariant: "#20242C",
	SurfaceHigh:    "#292D36",
	SurfaceLow:     "#171B21",

	// Lines — spec Outline on dark surfaces
	Outline:        "#938F99",
	OutlineVariant: "#262B33",

	// Primary accent — spec baseline (#D0BCFF), tonal container computed
	Primary:            "#D0BCFF",
	OnPrimary:          "#381E72",
	PrimaryHover:       "#DCC7FF",
	PrimaryPressed:     "#B69EFF",
	PrimaryContainer:   "#4F378B",
	OnPrimaryContainer: "#EADDFF",

	// Secondary accent — spec baseline (#CCC2DC)
	Secondary:            "#CCC2DC",
	OnSecondary:          "#332D41",
	SecondaryContainer:   "#33304A",
	OnSecondaryContainer: "#E8DEF8",

	// Tertiary accent — spec baseline (#EFB8C8)
	Tertiary:            "#EFB8C8",
	OnTertiary:          "#492A43",
	TertiaryContainer:   "#633B48",
	OnTertiaryContainer: "#FFD8E4",

	// Status
	Error:            "#F2B8B5",
	OnError:          "#601410",
	ErrorContainer:   "#8C1D18",
	OnErrorContainer: "#F9DEDC",
	Success:          "#86D9A4",
	OnSuccess:        "#003919",
	Warning:          "#F5C26B",
	OnWarning:        "#3A2B10",

	// Text
	Text:     "#E6E1E5",
	Muted:    "#CAC4D0",
	Disabled: "#6A6E7A",

	// Overlay. Pure black: the app background is already near-black, so a
	// near-black scrim would be invisible and the modal drawer would not
	// read as a layer on top of the application.
	Scrim: "#000000",
}

// names gives every role a stable, human-readable name. Used by the
// showcase and by error messages.
var names = map[Role]string{
	Background:           "background",
	Surface:              "surface",
	SurfaceVariant:       "surface-variant",
	SurfaceHigh:          "surface-high",
	SurfaceLow:           "surface-low",
	Outline:              "outline",
	OutlineVariant:       "outline-variant",
	Primary:              "primary",
	OnPrimary:            "on-primary",
	PrimaryHover:         "primary-hover",
	PrimaryPressed:       "primary-pressed",
	PrimaryContainer:     "primary-container",
	OnPrimaryContainer:   "on-primary-container",
	Secondary:            "secondary",
	OnSecondary:          "on-secondary",
	SecondaryContainer:   "secondary-container",
	OnSecondaryContainer: "on-secondary-container",
	Tertiary:             "tertiary",
	OnTertiary:           "on-tertiary",
	TertiaryContainer:    "tertiary-container",
	OnTertiaryContainer:  "on-tertiary-container",
	Error:                "error",
	OnError:              "on-error",
	ErrorContainer:       "error-container",
	OnErrorContainer:     "on-error-container",
	Success:              "success",
	OnSuccess:            "on-success",
	Warning:              "warning",
	OnWarning:            "on-warning",
	Text:                 "text",
	Muted:                "muted",
	Disabled:             "disabled",
	Scrim:                "scrim",
}

// String returns the role's name.
func (r Role) String() string {
	if r == RoleUnset {
		return "unset"
	}
	if n, ok := names[r]; ok {
		return n
	}
	return "unknown"
}

// Hex returns the canonical truecolor value of a role as "#rrggbb".
//
// The unset role has no colour and returns an empty string.
func (r Role) Hex() string {
	if r == RoleUnset {
		return ""
	}
	if h, ok := hex[r]; ok {
		return h
	}
	return "#000000"
}

// AllRoles returns every defined role in a stable order. The showcase and the
// tests iterate over it.
func AllRoles() []Role {
	roles := make([]Role, 0, roleCount-1)
	for r := Background; r < roleCount; r++ {
		roles = append(roles, r)
	}
	return roles
}

// Color returns the role's colour adapted to the terminal's capabilities.
//
// It returns nil when the terminal cannot display colour at all; callers
// must treat nil as "no colour" rather than rendering a broken escape
// sequence.
func (r Role) Color() color.Color {
	if r == RoleUnset {
		return nil
	}
	return colorFor(r.Hex())
}

// Lipgloss returns the role as a lipgloss colour. It is nil-safe: when the
// terminal has no colour support the returned colour is nil and the caller
// should skip the colour entirely.
func (r Role) Lipgloss() color.Color {
	return r.Color()
}

// Fg applies a foreground role to a style, skipping it when the terminal has
// no colour support.
func Fg(s lipgloss.Style, r Role) lipgloss.Style {
	c := r.Color()
	if c == nil {
		return s
	}
	return s.Foreground(c)
}

// Bg applies a background role to a style, skipping it when the terminal has
// no colour support.
func Bg(s lipgloss.Style, r Role) lipgloss.Style {
	c := r.Color()
	if c == nil {
		return s
	}
	return s.Background(c)
}

// BorderFg applies a border role to a style, skipping it when the terminal
// has no colour support.
func BorderFg(s lipgloss.Style, r Role) lipgloss.Style {
	c := r.Color()
	if c == nil {
		return s
	}
	return s.BorderForeground(c)
}
