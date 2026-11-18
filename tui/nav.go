package tui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// navID identifies a destination of the navigation drawer.
type navID int

const (
	// navNone is the absence of a destination.
	navNone navID = -1
	// navDiscover is the discovery destination.
	navDiscover navID = iota
	// navSearch is the search destination.
	navSearch
	// navHistory is the search history destination.
	navHistory
	// navWatchlist is the watchlist destination.
	navWatchlist
	// navEpisodes is the episode list of the selected anime.
	navEpisodes
	// navAniList is the AniList account section.
	navAniList
	// navSettings is the settings sheet.
	navSettings
)

// navDestination describes one entry of the navigation drawer.
type navDestination struct {
	id    navID
	label string
	icon  string
	// hint is the secondary text shown on the right of the row.
	hint string
	// supported reports whether the running application can actually open
	// the destination.
	//
	// A destination that is not supported stays in the drawer, dimmed and
	// unreachable: the navigation never changes shape under the user, and
	// no screen is invented for it.
	supported bool
}

// navDestinations is the content of the drawer, in the order it is drawn.
//
// The list covers the destinations the design calls for. "Discover" has
// no counterpart in the application yet - there is no seasonal
// catalogue - so it is listed as unavailable instead of being faked with
// an empty screen. "Watchlist" is the saved list of the AniList account.
var navDestinations = []navDestination{
	{id: navDiscover, label: "Discover", icon: theme.IconDiscover, hint: "n/a", supported: false},
	{id: navSearch, label: "Search", icon: theme.IconSearch, supported: true},
	{id: navHistory, label: "History", icon: theme.IconHistory, supported: true},
	{id: navWatchlist, label: "Watchlist", icon: theme.IconWatchlist, supported: true},
	{id: navEpisodes, label: "Episodes", icon: theme.IconEpisode, supported: true},
	{id: navAniList, label: "AniList", icon: theme.IconAniList, supported: true},
	{id: navSettings, label: "Settings", icon: theme.IconSettings, supported: true},
}

// navState holds the state of the navigation drawer.
type navState struct {
	// expanded is the user's preference for the persistent sidebar. It is
	// only honoured on terminals that are wide enough for one.
	expanded bool
	// overlay is the modal drawer. It is only used on terminals that are
	// too narrow for a persistent sidebar.
	overlay bool
	// focused reports whether the drawer holds the keyboard.
	focused bool
	// cursor is the index of the highlighted destination.
	cursor int
	// contentInputFocus remembers whether the search field held the focus
	// when the drawer was opened, so it can be restored afterwards.
	contentInputFocus bool
}

// navPersistent reports whether the terminal is wide enough for a persistent
// navigation sidebar.
func (m *SearchModel) navPersistent() bool {
	return ui.SupportsPersistentDrawer(m.width)
}

// navVisible reports whether the drawer is on screen.
func (m *SearchModel) navVisible() bool {
	if m.navPersistent() {
		return m.nav.expanded
	}
	return m.nav.overlay
}

// navFocusable reports whether the drawer may hold the keyboard right now.
//
// Modal screens (the settings sheet, the quit prompt, the design system
// showcase) take the keyboard away from the drawer.
func (m *SearchModel) navFocusable() bool {
	switch m.state {
	case searchState, animeState, detailState, historyState, watchlistState:
		return true
	}
	return false
}

// navHasFocus reports whether the drawer holds the keyboard.
func (m *SearchModel) navHasFocus() bool {
	return m.navVisible() && m.nav.focused && m.navFocusable()
}

// toggleDrawer opens or closes the navigation drawer.
//
// On a terminal with room for a sidebar the drawer is persistent: the
// hamburger collapses it or expands it again. On a narrow terminal the
// drawer is a modal overlay that opens and closes in place.
func (m *SearchModel) toggleDrawer() {
	if m.navPersistent() {
		switch {
		case !m.nav.expanded:
			m.nav.expanded = true
			m.focusNav()
		case m.nav.focused:
			// The drawer is open and in use: collapse it.
			m.nav.expanded = false
			m.releaseNavFocus()
		default:
			// The sidebar is visible but the content has the focus: hand
			// the keyboard to the drawer.
			m.focusNav()
		}
		m.updateLayout()
		return
	}

	if m.nav.overlay {
		m.nav.overlay = false
		m.releaseNavFocus()
		return
	}
	m.nav.overlay = true
	m.focusNav()
}

// closeDrawer closes the drawer and gives the keyboard back to the content.
func (m *SearchModel) closeDrawer() {
	if m.navPersistent() {
		m.nav.expanded = false
	} else {
		m.nav.overlay = false
	}
	m.releaseNavFocus()
	m.updateLayout()
}

// focusNav moves the keyboard into the drawer.
//
// The cursor jumps to the destination of the current page, so the drawer
// always opens on the page the user is looking at.
func (m *SearchModel) focusNav() {
	m.nav.contentInputFocus = m.textInput.Focused()
	m.textInput.Blur()
	m.nav.focused = true
	if idx := navIndexOf(m.activeNav()); idx >= 0 {
		m.nav.cursor = idx
	}
}

// releaseNavFocus gives the keyboard back to the content.
func (m *SearchModel) releaseNavFocus() {
	m.nav.focused = false
	if m.nav.contentInputFocus {
		m.textInput.Focus()
		m.nav.contentInputFocus = false
	}
}

// moveNavCursor moves the cursor over the destinations.
//
// Unavailable destinations are skipped: the cursor never lands on a row
// that cannot be opened.
func (m *SearchModel) moveNavCursor(delta int) {
	if delta == 0 {
		return
	}
	items := m.navItems()
	if len(items) == 0 {
		return
	}

	idx := m.nav.cursor
	for step := 0; step < len(items); step++ {
		idx += delta
		if idx < 0 || idx >= len(items) {
			return
		}
		if items[idx].Enabled {
			m.nav.cursor = idx
			return
		}
	}
}

// navItems returns the drawer content for the current state.
func (m *SearchModel) navItems() []ui.NavItem {
	active := m.activeNav()
	items := make([]ui.NavItem, 0, len(navDestinations))

	for i, dest := range navDestinations {
		items = append(items, ui.NavItem{
			Label:    dest.label,
			Icon:     dest.icon,
			Hint:     m.navHint(dest),
			Active:   dest.id == active,
			Selected: i == m.nav.cursor,
			Enabled:  m.navAvailable(dest.id),
		})
	}
	return items
}

// navHint returns the secondary text of a destination.
func (m *SearchModel) navHint(dest navDestination) string {
	if dest.id == navEpisodes && !m.navAvailable(navEpisodes) {
		return "no anime"
	}
	return dest.hint
}

// navAvailable reports whether a destination can be opened right now.
//
// A destination the application does not implement is never available, and
// "Episodes" needs an anime: without a selection there is no episode list
// to show, so the destination stays out of reach.
func (m *SearchModel) navAvailable(id navID) bool {
	for _, dest := range navDestinations {
		if dest.id != id {
			continue
		}
		if !dest.supported {
			return false
		}
		if id == navEpisodes {
			anime := m.GetSelectedAnime()
			return anime != nil && anime.ShowID != ""
		}
		return true
	}
	return false
}

// activeNav returns the destination of the current page.
func (m *SearchModel) activeNav() navID {
	switch m.state {
	case searchState:
		return navSearch
	case animeState:
		return navSearch
	case historyState:
		return navHistory
	case watchlistState:
		return navWatchlist
	case detailState:
		return navEpisodes
	case settingsState:
		if m.settingsState.Cursor == settingsRowAniList {
			return navAniList
		}
		return navSettings
	case anilistState, confirmLogoutState:
		return navAniList
	case anilistLoginState:
		return navAniList
	case confirmQuitState:
		return navActiveOf(m.prevState)
	}
	return navNone
}

// navActiveOf returns the destination of a screen.
func navActiveOf(state tuiState) navID {
	switch state {
	case searchState:
		return navSearch
	case animeState:
		return navSearch
	case historyState:
		return navHistory
	case watchlistState:
		return navWatchlist
	case detailState:
		return navEpisodes
	case settingsState:
		return navSettings
	case anilistState, anilistLoginState:
		return navAniList
	}
	return navNone
}

// navLabel returns the name of a destination.
func navLabel(id navID) string {
	for _, dest := range navDestinations {
		if dest.id == id {
			return dest.label
		}
	}
	return ""
}

// navIndexOf returns the index of a destination in the drawer.
func navIndexOf(id navID) int {
	for i, dest := range navDestinations {
		if dest.id == id {
			return i
		}
	}
	return -1
}

// navFooterHint returns the hint line at the bottom of the drawer.
//
// It is plain text on purpose: the drawer is narrow, and a styled hint
// would not fit into a phone sized column.
func (m *SearchModel) navFooterHint() string {
	// The hint describes the keys of the drawer: showing it while the
	// drawer does not hold the keyboard would advertise keys that move
	// the list instead.
	if !m.navHasFocus() {
		return ""
	}
	return "↑↓ move · esc close"
}

// handleNavKey handles a key while the drawer holds the keyboard.
func (m *SearchModel) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Quit):
		m.prevState = m.state
		m.confirmSelect = 1
		m.state = confirmQuitState
		return m, nil

	case key.Matches(msg, m.keymap.Menu), key.Matches(msg, m.keymap.MenuAlt):
		m.toggleDrawer()
		return m, nil

	case key.Matches(msg, m.keymap.Back):
		m.closeDrawer()
		return m, nil

	case key.Matches(msg, m.keymap.Up):
		m.moveNavCursor(-1)
		return m, nil

	case key.Matches(msg, m.keymap.Down):
		m.moveNavCursor(1)
		return m, nil

	case key.Matches(msg, m.keymap.Select):
		return m, m.activateNav()
	}

	return m, nil
}

// handleHistoryKey handles a key on the watch history screen.
//
// The history is a list of the episodes that were played: the store the
// player feeds, exposed as a destination of its own. Enter resumes the
// entry: the anime is searched again and the matching result is picked,
// so the user lands on the episode list of what they watched.
func (m *SearchModel) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := m.watchHistory()

	switch {
	case key.Matches(msg, m.keymap.Quit):
		m.prevState = m.state
		m.confirmSelect = 1
		m.state = confirmQuitState
		return m, nil

	case key.Matches(msg, m.keymap.Back):
		m.state = searchState
		return m, nil

	case key.Matches(msg, m.keymap.Up):
		if m.recentCursor > 0 {
			m.recentCursor--
		}
		return m, nil

	case key.Matches(msg, m.keymap.Down):
		if m.recentCursor < len(entries)-1 {
			m.recentCursor++
		}
		return m, nil

	case key.Matches(msg, m.keymap.Select):
		if m.recentCursor < 0 || m.recentCursor >= len(entries) {
			return m, nil
		}
		entry := entries[m.recentCursor]
		// The provider searches by title: the streaming id is remembered
		// so the results can be steered back to the watched entry.
		m.pendingShowID = entry.ShowID
		m.textInput.SetValue(entry.Name)
		m.lastQuery = entry.Name
		m.searchState.Query = entry.Name
		m.searchState.Loading = true
		m.searchState.Results = nil
		m.searchState.Err = nil
		m.progressPercent = 0
		m.progressTarget = 0.3
		m.textInput.Blur()
		m.state = searchState
		return m, tea.Batch(
			m.doSearch(entry.Name),
			tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
				return progressTickMsg{}
			}),
			m.loading.Tick,
		)
	}

	return m, nil
}

// watchHistory returns the entries of the history screen.
//
// The list is read once when the screen is opened; the cursor walks over
// the copy.
func (m *SearchModel) watchHistory() []historyEntry {
	return m.watchHistoryCache
}

// activateNav opens the destination under the cursor.
func (m *SearchModel) activateNav() tea.Cmd {
	if m.nav.cursor < 0 || m.nav.cursor >= len(navDestinations) {
		return nil
	}
	dest := navDestinations[m.nav.cursor]
	if !dest.supported || !m.navAvailable(dest.id) {
		return nil
	}

	var cmd tea.Cmd
	switch dest.id {
	case navSearch:
		cmd = m.gotoSearch()
	case navHistory:
		cmd = m.gotoHistory()
	case navWatchlist:
		cmd = m.gotoWatchlist()
	case navEpisodes:
		cmd = m.gotoEpisodes()
	case navAniList:
		cmd = m.gotoAniList()
	case navSettings:
		cmd = m.gotoSettings(0)
	}

	// The destination is open: the drawer gives the keyboard back.
	if m.navPersistent() {
		m.releaseNavFocus()
	} else {
		m.nav.overlay = false
		m.releaseNavFocus()
	}
	return cmd
}

// gotoSearch opens the search screen and focuses the search field.
func (m *SearchModel) gotoSearch() tea.Cmd {
	if m.state != searchState {
		m.state = searchState
		m.backStack = nil
		m.episodeState = NewEpisodeState()
		m.textInput.Placeholder = placeholderSearch
	}
	m.textInput.Focus()
	m.textInput.SetValue(m.lastQuery)
	return nil
}

// gotoHistory opens the watch history.
func (m *SearchModel) gotoHistory() tea.Cmd {
	m.state = historyState
	m.watchHistoryCache = loadWatchHistory()
	m.recentCursor = 0
	m.textInput.Blur()
	return nil
}

// gotoAniList opens the AniList account screen.
func (m *SearchModel) gotoAniList() tea.Cmd {
	m.prevState = m.state
	m.state = anilistState
	m.anilistCursor = aniListStartCursor()
	return nil
}

// gotoEpisodes opens the episode list of the selected anime.
func (m *SearchModel) gotoEpisodes() tea.Cmd {
	anime := m.GetSelectedAnime()
	if anime == nil || anime.ShowID == "" {
		return nil
	}

	m.pushState(detailState)
	m.textInput.Placeholder = placeholderEpisode
	m.episodeState.AnimeID = anime.ShowID
	m.episodeState.Loading = true
	m.progressPercent = 0
	m.progressTarget = 0.4

	return tea.Batch(
		m.fetchEpisodes(anime),
		m.fetchTrackingStatusCmd(anime.AniListID),
		tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
			return progressTickMsg{}
		}),
		m.loading.Tick,
	)
}

// gotoSettings opens the settings sheet with the cursor on a row.
//
// The rows are shared with the settings screen, so the AniList destination
// lands on the account row and not on a second, parallel implementation.
func (m *SearchModel) gotoSettings(cursor int) tea.Cmd {
	m.prevState = m.state
	m.state = settingsState
	m.settingsState.Cursor = cursor
	return nil
}

// navTitle returns the title of the active screen for the app bar.
func (m *SearchModel) navTitle() string {
	switch m.state {
	case themeState:
		return "Design System"
	case confirmQuitState:
		return "Quit Anilix?"
	case confirmLogoutState:
		return "Log out of AniList?"
	case anilistState:
		return "AniList"
	case anilistLoginState:
		return "AniList Login"
	case animeState:
		return m.animeTitle()
	}
	if id := m.activeNav(); id != navNone {
		return navLabel(id)
	}
	return ""
}

// animeTitle returns the name of the selected anime.
//
// The details screen is about one entry, and the app bar names it.
func (m *SearchModel) animeTitle() string {
	anime := m.GetSelectedAnime()
	if anime == nil {
		return "Details"
	}
	return anime.Name
}
