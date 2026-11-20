package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// viewThemeShowcase renders the design system showcase.
//
// It is the reference implementation of every component style: screens are
// built from exactly these parts, so the showcase doubles as living
// documentation of the visual language.
func (m *SearchModel) viewThemeShowcase() string {
	lines := themeShowcaseLines()

	// Clip to the viewport and apply the scroll offset.
	height := m.contentHeight - 1
	if height < 5 {
		height = 5
	}
	start := m.themeScroll
	if start > len(lines)-1 {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}

	view := strings.Join(lines[start:end], "\n")
	footer := m.themeShowcaseFooter(len(lines))
	return m.clampWidth(view) + "\n" + footer
}

// clampWidth truncates the rendered view to the terminal width.
//
// The showcase is wider than a phone screen, and a wrapped line would break
// the one-line-per-entry rhythm it is built on.
func (m *SearchModel) clampWidth(s string) string {
	if m.contentWidth <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(m.contentWidth).Render(line)
	}
	return strings.Join(lines, "\n")
}

// Showcase renders the whole design system showcase as plain text.
//
// It is exported so the showcase can be printed outside the TUI (see the
// "anilix theme" command) - useful for reviewing the design system in a
// pull request or piping it to a file.
func Showcase() string {
	return strings.Join(themeShowcaseLines(), "\n")
}

// themeShowcaseFooter renders the hint line under the showcase.
func (m *SearchModel) themeShowcaseFooter(total int) string {
	pos := fmt.Sprintf("%d/%d", minInt(m.themeScroll+1, total), total)
	return theme.Caption.Render("↑/↓ scroll  ") +
		theme.Caption.Render("esc close  ") +
		theme.Caption.Render(pos)
}

// themeShowcaseLines builds every line of the showcase.
func themeShowcaseLines() []string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	blank := func() { lines = append(lines, "") }

	add(theme.Headline.Render("Anilix Design System"))
	add(theme.Label.Render("Material Design 3 · dark baseline · keyboard-driven terminal"))
	blank()

	// --- Capabilities -------------------------------------------------
	add(theme.Overline.RenderUpper("Terminal capabilities"))
	add(fmt.Sprintf("  profile %s · unicode %s · color %s",
		theme.Profile(),
		boolWord(theme.HasUnicode()),
		boolWord(theme.HasColor())))
	blank()

	// --- Palette ------------------------------------------------------
	add(theme.Overline.RenderUpper("Color tokens"))
	for _, group := range []struct {
		title string
		roles []theme.Role
	}{
		{"surfaces", []theme.Role{theme.Background, theme.Surface, theme.SurfaceVariant, theme.SurfaceHigh, theme.SurfaceLow}},
		{"accents", []theme.Role{theme.Primary, theme.Secondary, theme.Tertiary, theme.Error, theme.Success, theme.Warning}},
		{"text", []theme.Role{theme.Text, theme.Muted, theme.Disabled}},
		{"lines", []theme.Role{theme.Outline, theme.OutlineVariant}},
	} {
		add("  " + theme.LabelStrong.Render(group.title))
		for _, r := range group.roles {
			add(paletteRow(r))
		}
	}
	blank()

	add(theme.Overline.RenderUpper("On-colour pairs"))
	for _, pair := range []struct{ fg, bg theme.Role }{
		{theme.OnPrimary, theme.Primary},
		{theme.OnPrimaryContainer, theme.PrimaryContainer},
		{theme.OnSecondaryContainer, theme.SecondaryContainer},
		{theme.OnTertiaryContainer, theme.TertiaryContainer},
		{theme.OnErrorContainer, theme.ErrorContainer},
	} {
		add(fmt.Sprintf("  %s %s",
			theme.SwatchText(" sample ", pair.fg, pair.bg),
			theme.Caption.Render(pair.fg.String()+" on "+pair.bg.String())))
	}
	blank()

	// --- Typography ---------------------------------------------------
	add(theme.Overline.RenderUpper("Typography"))
	for _, t := range theme.AllTypes() {
		add(fmt.Sprintf("  %-12s %s", theme.Caption.Render(t.String()), t.Render("The quick brown fox")))
	}
	add("  " + theme.Overline.RenderUpper("overlined eyebrow"))
	blank()

	// --- Spacing and shape ---------------------------------------------
	add(theme.Overline.RenderUpper("Spacing (cells)"))
	add("  " + theme.Caption.Render(strings.Join(spacingSamples(), "  ")))
	add(theme.Overline.RenderUpper("Elevation and shape"))
	add(fmt.Sprintf("  %s  %s  %s  %s",
		theme.Caption.Render("elevation0 background"),
		theme.Caption.Render("elevation1 surface"),
		theme.Caption.Render("elevation2 surface-variant"),
		theme.Caption.Render("elevation3 surface-high")))
	add(fmt.Sprintf("  %s  %s  %s",
		theme.Caption.Render("size compact"),
		theme.Caption.Render("size medium"),
		theme.Caption.Render("size expanded")))
	blank()

	// --- Breakpoints --------------------------------------------------
	add(theme.Overline.RenderUpper("Layout breakpoints"))
	for _, bp := range ui.AllBreakpoints() {
		add(fmt.Sprintf("  %-10s %s",
			theme.LabelStrong.Render(bp.String()),
			theme.Caption.Render(breakpointNote(bp))))
	}
	blank()

	// --- Components ---------------------------------------------------
	add(theme.Overline.RenderUpper("Buttons"))
	for _, variant := range ui.AllButtonVariants() {
		add("  " + strings.Join([]string{
			ui.Button{Label: variant.String(), Variant: variant}.Render(),
			ui.Button{Label: "hover", Variant: variant, State: theme.StateHover}.Render(),
			ui.Button{Label: "focus", Variant: variant, State: theme.StateFocused}.Render(),
			ui.Button{Label: "disabled", Variant: variant, State: theme.StateDisabled}.Render(),
		}, "  "))
	}
	blank()

	add(theme.Overline.RenderUpper("Chips and badges"))
	add("  " + strings.Join([]string{
		ui.Chip{Label: "TV"}.Render(),
		ui.Chip{Label: "Action", Container: theme.SecondaryContainer}.Render(),
		ui.Chip{Label: "Score", Container: theme.TertiaryContainer}.Render(),
		ui.Chip{Label: "Error", Container: theme.ErrorContainer}.Render(),
		ui.Chip{Label: "Disabled", State: theme.StateDisabled}.Render(),
		ui.Badge{Label: "12"}.Render(),
	}, " "))
	blank()

	add(theme.Overline.RenderUpper("Tabs"))
	add("  " + ui.TabBar([]ui.Tab{
		{Label: "Overview"},
		{Label: "Episodes", Selected: true},
		{Label: "Related", Badge: "3"},
	}))
	blank()

	add(theme.Overline.RenderUpper("List items"))
	add(ui.ListItem{
		Title:       "One Piece",
		Description: "TV · 1999 · 1180 eps",
		Trailing:    ui.Rating(8.73),
	}.Render(46))
	add(ui.ListItem{
		Title:       "Frieren: Beyond Journey's End",
		Description: "TV · 2023 · 28 eps",
		Trailing:    ui.Rating(9.03),
		Selected:    true,
		Focused:     true,
	}.Render(46))
	add(ui.ListItem{
		Title:       "Blue Box Season 2",
		Description: "TV · 2026 · not yet aired",
		Selected:    true,
	}.Render(46))
	add(ui.ListItem{
		Title:       "Blue Box Season 2",
		Description: "TV · 2026 · not yet aired",
		Disabled:    true,
	}.Render(46))
	blank()

	add(theme.Overline.RenderUpper("Menu items"))
	add(ui.MenuItem{Label: "Search", Icon: theme.Icon(theme.IconSearch), Selected: true, Focused: true}.Render(40))
	add(ui.MenuItem{Label: "Recent searches", Hint: "10"}.Render(40))
	add(ui.MenuItem{Label: "Settings", Icon: theme.Icon(theme.IconSettings)}.Render(40))
	add(ui.MenuItem{Label: "Unavailable", Disabled: true}.Render(40))
	blank()

	// --- Application shell --------------------------------------------
	add(theme.Overline.RenderUpper("Application shell"))
	add("  " + theme.Caption.Render("App bar: navigation control, brand, screen title, status."))
	add("  " + ui.AppBar{
		Brand: theme.LabelStrong.Render("ANILIX"),
		Title: "Search",
		Menu:  true,
		Right: []string{
			theme.Fg(lipgloss.NewStyle(), theme.Primary).Render("1080p"),
			theme.Fg(lipgloss.NewStyle(), theme.Success).Render(theme.Icon(theme.IconDot) + " user"),
		},
		Width: 50,
	}.Render())
	add("  " + theme.Caption.Render("Navigation drawer: persistent sidebar with rule."))
	drawer := ui.Drawer{
		Title: "Menu",
		Items: []ui.NavItem{
			{Label: "Discover", Icon: theme.Icon(theme.IconDiscover), Hint: "n/a", Enabled: false},
			{Label: "Search", Icon: theme.Icon(theme.IconSearch), Active: true, Selected: true, Enabled: true},
			{Label: "History", Icon: theme.Icon(theme.IconHistory), Enabled: true},
			{Label: "Episodes", Icon: theme.Icon(theme.IconEpisode), Hint: "no anime", Enabled: false},
			{Label: "AniList", Icon: theme.Icon(theme.IconAniList), Enabled: true},
			{Label: "Settings", Icon: theme.Icon(theme.IconSettings), Enabled: true},
		},
		Footer:  "↑↓ move · esc close",
		Width:   22,
		Height:  10,
		Focused: true,
		Fill:    theme.Surface,
		Rule:    true,
	}.Render()
	for _, line := range strings.Split(drawer, "\n") {
		add(line)
	}
	add("  " + theme.Caption.Render("The drawer is a modal overlay under 60 columns."))
	blank()

	add(theme.Overline.RenderUpper("Card"))
	add(ui.Card{
		Title:     "Demon Slayer",
		Subtitle:  "TV · 2019 · 26 episodes",
		Body:      "Tanjiro and his allies fight demons to save his sister.",
		Elevation: theme.Elevation1,
		Width:     44,
	}.Render())
	blank()

	add(theme.Overline.RenderUpper("Dialog"))
	add(ui.Dialog{
		Title: "Quit Anilix?",
		Body:  "Are you sure you want to quit?",
		Actions: []string{
			ui.Button{Label: "Yes", Variant: ui.ButtonFilled}.Render(),
			ui.Button{Label: "No", Variant: ui.ButtonText}.Render(),
		},
		Width:     44,
		Elevation: theme.Elevation3,
	}.Render())
	blank()

	add(theme.Overline.RenderUpper("Empty and error states"))
	add(ui.EmptyState{
		Icon:  theme.Icon(theme.IconSearch),
		Title: "No results",
		Hint:  "Try a different title or check the spelling.",
	}.Render(46))
	add(ui.ErrorState{
		Title:   "Search failed",
		Message: "provider unreachable",
		Hint:    "ctrl+r to retry",
	}.Render(46))
	add(ui.LoadingIndicator("Searching...", 2))
	add(ui.ProgressBar(62, 40))
	blank()

	add(theme.Overline.RenderUpper("Dividers and headers"))
	add(ui.Divider(false, 40))
	add(ui.Divider(true, 40))
	add(ui.SectionHeader("Section", "3 items", 40))
	blank()

	add(theme.Overline.RenderUpper("Icons"))
	add("  " + strings.Join(iconSamples(), " "))
	blank()

	add(theme.Overline.RenderUpper("Search field"))
	add(ui.InputField{Placeholder: "Search anime...", Width: 46}.Render())
	add(ui.InputField{Content: "one piece", Focused: true, Width: 46}.Render())
	blank()

	add(theme.Overline.RenderUpper("Metadata rows"))
	add(ui.InfoRow("Type", "TV"))
	add(ui.InfoRow("Episodes", "28"))
	add(ui.InfoRow("Status", "Finished Airing"))
	add("  " + strings.Join(ui.ChipRow([]string{"Action", "Adventure", "Drama", "Fantasy"}, 46, theme.SecondaryContainer), " "))
	blank()

	add(theme.Overline.RenderUpper("Key hints"))
	add("  " + ui.KeyHint("enter", "play") + "   " + ui.KeyHint("esc", "back") + "   " + ui.KeyHint("tab", "sub/dub"))
	add(theme.Caption.Render("  Renders exactly like the application screens."))

	return lines
}

// paletteRow renders one colour token: swatch, role name and hex value.
func paletteRow(r theme.Role) string {
	return fmt.Sprintf("  %s %-22s %s",
		theme.Swatch(r, 6),
		theme.Label.Render(r.String()),
		theme.Caption.Render(r.Hex()))
}

// breakpointNote describes what a layout tier does.
func breakpointNote(bp ui.Breakpoint) string {
	switch bp {
	case ui.Compact:
		return "single column, modal drawer, < 60 cols"
	case ui.Medium:
		return "list + narrow preview, persistent drawer"
	default:
		return "list + wide preview, persistent drawer"
	}
}

// iconSamples returns every icon of the set.
func iconSamples() []string {
	icons := []string{
		theme.IconMenu, theme.IconMenuItem, theme.IconSearch, theme.IconChevronRight,
		theme.IconChevronDown, theme.IconCheck, theme.IconClose, theme.IconSettings,
		theme.IconPlay, theme.IconPause, theme.IconEpisode, theme.IconStar,
		theme.IconDot, theme.IconWarning, theme.IconInfo, theme.IconExternal,
		theme.IconLock, theme.IconSelectionBar, theme.IconTabActive,
		theme.IconDiscover, theme.IconHistory, theme.IconWatchlist, theme.IconAniList,
	}
	out := make([]string, 0, len(icons))
	for _, i := range icons {
		out = append(out, theme.Label.Render(i))
	}
	return out
}

// spacingSamples renders the spacing scale as visual bars.
func spacingSamples() []string {
	out := make([]string, 0, 8)
	for _, s := range theme.AllSpaces() {
		out = append(out, fmt.Sprintf("%s%d", theme.Caption.Render(s.String()+"="), s.Cells()))
	}
	return out
}

// boolWord renders a boolean as a readable word.
func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensure the showcase keeps using lipgloss for measurement helpers.
var _ = lipgloss.Width
