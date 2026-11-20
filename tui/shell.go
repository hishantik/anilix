package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// The application shell.
//
// Every screen of Anilix is rendered inside the same frame:
//
//	┌──────────────────────────────────────────────────────────┐
//	│ ☰  ANILIX │ Search                       SUB  1080p  ●  │  app bar
//	├───────────┬──────────────────────────────────────────────┤
//	│  MENU     │                                              │
//	│ ▌ Search  │              content region                  │  drawer + content
//	│   History │                                              │
//	│   Episodes│                                              │
//	│   AniList │                                              │
//	│   Settings│                                              │
//	├───────────┴──────────────────────────────────────────────┤
//	│ m menu · enter open · ctrl+s settings · ctrl+c quit      │  footer
//	└──────────────────────────────────────────────────────────┘
//
// The shell owns the frame only: the screens themselves are still the
// ones from the pre-redesign application and are migrated one by one.
const (
	// appBarHeight is the height of the top application bar.
	appBarHeight = 1
	// footerHeight is the height of the status/footer region.
	footerHeight = 1
)

// shellRegions is the geometry of the application shell.
type shellRegions struct {
	// contentWidth is the width of the content region.
	contentWidth int
	// contentHeight is the height of the content region.
	contentHeight int
	// drawerWidth is the width of the persistent sidebar. It is zero when
	// there is no sidebar.
	drawerWidth int
	// progress reports whether the progress bar takes a row of its own.
	progress bool
}

// shellRegions computes the geometry of the shell for the current terminal
// size and drawer state.
func (m *SearchModel) shellRegions() shellRegions {
	r := shellRegions{contentWidth: m.width}

	if m.navPersistent() && m.nav.expanded {
		r.drawerWidth = ui.DrawerWidth(m.width)
		r.contentWidth = m.width - r.drawerWidth
	}
	if r.contentWidth < 1 {
		r.contentWidth = 1
	}

	r.progress = m.searchState.Loading || m.episodeState.Loading || m.episodeState.Playing

	height := m.height - appBarHeight - footerHeight
	if r.progress {
		height--
	}
	if height < 1 {
		height = 1
	}
	r.contentHeight = height

	return r
}

// renderShell renders the whole application: app bar, content region,
// optional progress bar and the footer.
func (m *SearchModel) renderShell(content string) string {
	regions := m.shellRegions()

	var b strings.Builder
	b.WriteString(m.renderAppBar())
	b.WriteString("\n")
	b.WriteString(m.renderBody(content, regions))
	if regions.progress {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).
			Render(m.progress.ViewAs(m.progressPercent)))
	}
	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// renderAppBar renders the top application bar.
func (m *SearchModel) renderAppBar() string {
	pill := renderSubDubSwitch(m.searchState.TranslationType)
	bar := ui.AppBar{
		Brand:    brandWordmark(),
		Title:    m.navTitle(),
		Menu:     true,
		MenuOpen: m.navVisible(),
		Right:    m.appBarStatus(),
		Width:    m.width,
	}
	out, rightStart := bar.RenderWithLayout()

	// The SUB/DUB pill switch is the first item of the right side: its
	// clickable range starts where the right side begins and spans the
	// width of the pill. The bounds describe the frame being drawn, so a
	// mouse click is checked against exactly what is on screen.
	m.psX1, m.psX2 = 0, 0
	if w := lipgloss.Width(pill); rightStart >= 0 && w > 0 {
		m.psX1 = rightStart
		m.psX2 = m.psX1 + w
	}
	return out
}

// appBarStatus returns the status items of the app bar, most important
// first: the least important ones are dropped when the bar is too narrow.
func (m *SearchModel) appBarStatus() []string {
	items := make([]string, 0, 3)

	items = append(items, renderSubDubSwitch(m.searchState.TranslationType))

	quality := m.settingsState.Quality
	if quality == "" {
		quality = "auto"
	}
	items = append(items, theme.Fg(lipgloss.NewStyle(), theme.Primary).Render(quality))

	if m.trackingEnabled && m.anilistUsername != "" {
		items = append(items, theme.Fg(lipgloss.NewStyle(), theme.Success).
			Render(theme.Icon(theme.IconDot)+" "+m.anilistUsername))
	}

	return items
}

// renderBody renders the content region, with the navigation drawer next to
// it or on top of it.
func (m *SearchModel) renderBody(content string, regions shellRegions) string {
	// On a terminal that is too narrow for a sidebar the drawer is a modal
	// overlay: it takes over the content region.
	if !m.navPersistent() && m.nav.overlay {
		return m.renderOverlayDrawer(regions)
	}

	body := clipLines(content, regions.contentWidth)
	body = padToHeight(body, regions.contentHeight)

	if regions.drawerWidth > 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(regions), body)
	}
	return body
}

// renderSidebar renders the persistent navigation sidebar.
func (m *SearchModel) renderSidebar(regions shellRegions) string {
	drawer := ui.Drawer{
		Title:   "Menu",
		Items:   m.navItems(),
		Footer:  m.navFooterHint(),
		Width:   regions.drawerWidth,
		Height:  regions.contentHeight,
		Focused: m.nav.focused,
		Fill:    theme.Surface,
		Rule:    true,
	}
	return drawer.Render()
}

// renderOverlayDrawer renders the modal navigation drawer.
//
// The drawer is clamped to the terminal: it never wraps and never exceeds
// the width of the screen. The rest of the row is covered by a scrim, so
// the drawer reads as a layer on top of the application.
func (m *SearchModel) renderOverlayDrawer(regions shellRegions) string {
	width := ui.OverlayDrawerWidth(m.width)
	inner := width - 2
	if inner < 8 {
		inner = 8
	}

	drawer := ui.Drawer{
		Title:   "Menu",
		Items:   m.navItems(),
		Footer:  m.navFooterHint(),
		Width:   inner,
		Height:  maxInt(regions.contentHeight-2, 1),
		Focused: true,
		Fill:    theme.SurfaceHigh,
	}

	style := lipgloss.NewStyle().Border(theme.BorderRounded.Border(), true)
	if col := theme.Outline.Color(); col != nil {
		style = style.BorderForeground(col)
	}

	return scrimLines(style.Render(drawer.Render()), m.width)
}

// renderFooter renders the status/footer region.
func (m *SearchModel) renderFooter() string {
	return clipLines(m.footerHints(), m.width)
}

// footerHints returns the key hints of the current screen.
//
// The hints are dropped from the end when the terminal is too narrow for
// them: the most important one always survives, so the footer never tells
// the user less than it can.
func (m *SearchModel) footerHints() string {
	hints := m.hintList()
	if len(hints) == 0 {
		return ""
	}

	const separator = "  ·  "

	joined := hints[0]
	for _, hint := range hints[1:] {
		candidate := joined + separator + hint
		if m.width > 0 && lipgloss.Width(candidate) > m.width {
			break
		}
		joined = candidate
	}
	return joined
}

// hintList returns the key hints of the current screen, most important
// first.
func (m *SearchModel) hintList() []string {
	if m.navHasFocus() {
		return []string{
			ui.KeyHint("↑↓/jk", "move"),
			ui.KeyHint("enter", "open"),
			ui.KeyHint("esc", "close"),
			ui.KeyHint("ctrl+c", "quit"),
		}
	}

	switch m.state {
	case confirmQuitState:
		return []string{
			ui.KeyHint("←→", "choose"),
			ui.KeyHint("enter", "confirm"),
			ui.KeyHint("y", "yes"),
			ui.KeyHint("n/esc", "no"),
		}

	case confirmLogoutState:
		return []string{
			ui.KeyHint("←→", "choose"),
			ui.KeyHint("enter", "confirm"),
			ui.KeyHint("y", "yes"),
			ui.KeyHint("n/esc", "no"),
		}

	case anilistState:
		hints := make([]string, 0, 4)
		if aniListLoggedIn() {
			hints = append(hints, ui.KeyHint("↑↓", "move"))
		}
		return append(hints,
			ui.KeyHint("enter", "select"),
			ui.KeyHint("esc", "back"),
			ui.KeyHint("ctrl+c", "quit"),
		)

	case anilistLoginState:
		// The browser owns the flow until it answers: the only key the
		// screen honours is the one that leaves it.
		return []string{
			ui.KeyHint("esc", "cancel"),
		}

	case settingsState:
		hints := []string{ui.KeyHint("↑↓", "move")}
		switch {
		case m.settingsState.Cursor <= settingsRowPlayer:
			// The rows up to the player are values changed in place.
			hints = append(hints, ui.KeyHint("←→", "change"))
		case m.settingsState.Cursor == settingsRowVersion:
			// The version is read only.
		default:
			hints = append(hints, ui.KeyHint("enter", "select"))
		}
		return append(hints, ui.KeyHint("esc", "close"))

	case themeState:
		return []string{
			ui.KeyHint("↑↓", "scroll"),
			ui.KeyHint("esc", "close"),
		}

	case historyState:
		return []string{
			ui.KeyHint("↑↓", "browse"),
			ui.KeyHint("enter", "search"),
			ui.KeyHint("esc", "back"),
			ui.KeyHint("ctrl+c", "quit"),
		}

	case watchlistState:
		return []string{
			ui.KeyHint("↑↓", "browse"),
			ui.KeyHint("enter", "search"),
			ui.KeyHint("esc", "back"),
			ui.KeyHint("ctrl+c", "quit"),
		}

	case detailState:
		if m.episodeState.Err != nil {
			return []string{
				ui.KeyHint("ctrl+r", "retry"),
				ui.KeyHint("esc", "back"),
				ui.KeyHint("ctrl+c", "quit"),
			}
		}
		return []string{
			ui.KeyHint("esc", "back"),
			ui.KeyHint("enter", "play"),
			ui.KeyHint("tab", "sub/dub"),
			ui.KeyHint("ctrl+c", "quit"),
		}

	case animeState:
		return []string{
			ui.KeyHint("↑↓", "scroll"),
			ui.KeyHint("enter", "episodes"),
			ui.KeyHint("esc", "back"),
			ui.KeyHint("ctrl+c", "quit"),
		}

	default:
		if m.searchState.Err != nil {
			// The failed search has one useful key, and it is not the
			// list navigation: advertise the retry.
			return []string{
				ui.KeyHint("ctrl+r", "retry"),
				ui.KeyHint("/", "new search"),
				ui.KeyHint("ctrl+c", "quit"),
			}
		}
		if m.textInput.Focused() {
			// The search field has the keyboard: "m" would be typed into
			// it, so the menu is reached with the escape key first.
			return []string{
				ui.KeyHint("enter", "search"),
				ui.KeyHint("esc", "leave field"),
				ui.KeyHint("ctrl+s", "settings"),
				ui.KeyHint("ctrl+c", "quit"),
			}
		}
		return []string{
			ui.KeyHint("↑↓/jk", "move"),
			ui.KeyHint("enter", "open"),
			ui.KeyHint("/", "search"),
			ui.KeyHint("m", "menu"),
			ui.KeyHint("ctrl+s", "settings"),
			ui.KeyHint("ctrl+c", "quit"),
		}
	}
}

// clipLines truncates every line of a block to the given width.
//
// The screens are written for the width of the terminal. When the sidebar
// takes part of it, the content has to be cut instead of wrapped.
func clipLines(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return strings.Join(lines, "\n")
}

// padToHeight appends empty rows until the block is height rows tall.
func padToHeight(s string, height int) string {
	rows := strings.Count(s, "\n") + 1
	if rows >= height {
		return s
	}
	return s + strings.Repeat("\n", height-rows)
}

// scrimLines pads every line of a block with the scrim colour, so a modal
// layer covers the rest of the row.
func scrimLines(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		pad := width - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		lines[i] = line + theme.Bg(lipgloss.NewStyle(), theme.Scrim).Render(strings.Repeat(" ", pad))
	}
	return strings.Join(lines, "\n")
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
