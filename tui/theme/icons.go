package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Icon conventions.
//
// Terminals render one cell per glyph, so the icon set sticks to characters
// that are exactly one cell wide and monospaced-safe. Every symbol has an
// ASCII fallback for terminals without unicode support, and emoji are
// avoided entirely (they are two cells wide and break every alignment).
//
// Icons are named by meaning, not by shape, so a screen never hardcodes a
// glyph.
const (
	// IconMenu is the hamburger / navigation drawer toggle.
	IconMenu = "☰"
	// IconMenuItem is the bullet in front of a menu row.
	IconMenuItem = "•"
	// IconDiscover marks the discovery destination.
	IconDiscover = "✦"
	// IconHistory marks the search history destination.
	IconHistory = "↺"
	// IconWatchlist marks the watchlist destination.
	IconWatchlist = "⚑"
	// IconAniList marks the AniList account destination.
	IconAniList = "◈"
	// IconSearch is the search field icon. The focused state is expressed
	// with weight and colour rather than a second glyph, so the field keeps
	// a stable icon.
	IconSearch = "⌕"
	// IconChevronRight points right, used for disclosure.
	IconChevronRight = "›"
	// IconChevronDown points down, used for expanded disclosure.
	IconChevronDown = "˅"
	// IconCheck marks a completed or enabled state.
	IconCheck = "✓"
	// IconClose marks a dismiss or cancel action.
	IconClose = "✕"
	// IconSettings is the settings glyph.
	IconSettings = "⚙"
	// IconPlay starts playback.
	IconPlay = "▶"
	// IconPause pauses playback.
	IconPause = "❙❙"
	// IconEpisode marks an episode row.
	IconEpisode = "▸"
	// IconStar is the score marker.
	IconStar = "★"
	// IconDot is the small status dot.
	IconDot = "●"
	// IconWarning marks a caution.
	IconWarning = "▲"
	// IconInfo marks an informational note.
	IconInfo = "ℹ"
	// IconExternal marks a link that leaves the app.
	IconExternal = "↗"
	// IconLock marks a locked or unavailable item.
	IconLock = "⏻"
	// IconSelectionBar is the accent bar marking the selected row.
	IconSelectionBar = "▌"
	// IconPoster marks a cover that could not be rendered: the placeholder
	// stands in for the artwork, it never replaces it.
	IconPoster = "▣"
	// IconDivider is the horizontal rule character.
	IconDivider = "─"
	// IconTabActive marks the active tab.
	IconTabActive = "▎"
	// IconSpinner0..3 are the animation frames of the loading indicator.
	IconSpinner0 = "⠋"
	IconSpinner1 = "⠙"
	IconSpinner2 = "⠹"
	IconSpinner3 = "⠸"
)

// asciiFallbacks maps every icon to a plain ASCII equivalent, used when the
// terminal cannot render unicode.
var asciiFallbacks = map[string]string{
	IconMenu:         "=",
	IconMenuItem:     "*",
	IconDiscover:     "+",
	IconHistory:      "h",
	IconWatchlist:    "W",
	IconAniList:      "A",
	IconSearch:       "?",
	IconChevronRight: ">",
	IconChevronDown:  "v",
	IconCheck:        "+",
	IconClose:        "x",
	IconSettings:     "*",
	IconPlay:         ">",
	IconPause:        "||",
	IconEpisode:      ">",
	IconStar:         "*",
	IconDot:          "*",
	IconWarning:      "!",
	IconInfo:         "i",
	IconExternal:     "^",
	IconLock:         "o",
	IconSelectionBar: "|",
	IconPoster:       "@",
	IconDivider:      "-",
	IconTabActive:    "|",
	IconSpinner0:     "|",
	IconSpinner1:     "|",
	IconSpinner2:     "|",
	IconSpinner3:     "|",
}

// wideIcons are the icons that occupy two cells. Every other icon is
// exactly one cell wide so it can be used inside a single line without
// breaking the column alignment.
var wideIcons = map[string]int{
	IconPause: 2,
}

// IconWidth returns the number of cells an icon occupies.
func IconWidth(icon string) int {
	if w, ok := wideIcons[icon]; ok {
		return w
	}
	return 1
}

// Icon returns the icon, or its ASCII fallback when the terminal lacks
// unicode support.
func Icon(icon string) string {
	if HasUnicode() {
		return icon
	}
	if fallback, ok := asciiFallbacks[icon]; ok {
		return fallback
	}
	return "?"
}

// SpinnerFrame returns the frame of the loading spinner for a tick counter.
// The four braille frames animate smoothly in most terminals.
func SpinnerFrame(tick int) string {
	frames := []string{IconSpinner0, IconSpinner1, IconSpinner2, IconSpinner3}
	frame := frames[tick%len(frames)]
	if !HasUnicode() {
		return "/-\\|"[tick%4 : tick%4+1]
	}
	return frame
}

// Swatch renders a colour swatch: a filled block of the given role.
//
// It is used by the theme showcase and by any screen that needs to show a
// colour choice to the user.
func Swatch(role Role, width int) string {
	if width <= 0 {
		width = 4
	}
	return Bg(lipgloss.NewStyle(), role).Render(strings.Repeat(" ", width))
}

// SwatchText renders text on a coloured background, used to verify the
// on-* colour pairs of the palette.
func SwatchText(text string, fgRole, bgRole Role) string {
	style := Fg(Bg(lipgloss.NewStyle(), bgRole), fgRole)
	return style.Render(text)
}
