package tui

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/hianime"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/provider/kitsu"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/update"
	"github.com/hishantik/anilix/version"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

type SearchModel struct {
	state tuiState

	searchState  *SearchState
	episodeState *EpisodeState

	textInput textinput.Model
	help      help.Model
	keymap    keymap
	loading   spinner.Model
	progress  progress.Model

	searchList  list.Model
	episodeList list.Model

	// searchDelegate and episodeDelegate draw the rows of the two lists.
	// They are kept as pointers because they carry the focus state of the
	// list they belong to.
	searchDelegate  *resultDelegate
	episodeDelegate *resultDelegate

	jikanClient     *jikan.JikanClient
	anilistClient   *anilist.Client
	kitsuClient     *kitsu.Client
	hianimeProvider *hianime.Provider

	prevState     tuiState
	confirmSelect int

	// backStack holds the screens behind the current one.
	//
	// The escape key always returns to the view the user came from: the
	// results, the details of an entry and its episodes are three steps of
	// the same walk.
	backStack []tuiState

	settingsState *SettingsState

	selectedResult *SelectionResult
	lastQuery      string

	width  int
	height int

	// contentWidth and contentHeight are the dimensions of the content
	// region inside the application shell. They are smaller than the
	// terminal when the navigation drawer takes part of the screen.
	contentWidth  int
	contentHeight int

	// Navigation drawer
	nav navState

	progressPercent float64

	prevSearchListIndex  int
	metadataCache        map[int]*MetadataPanel
	kitsuAnimeCache      map[int]*kitsu.Anime
	metadataStale        bool
	episodeTitlesCache   map[int][]string
	episodeTitlesCacheMu sync.Mutex
	episodeMetadataCache map[string]*jikan.Episode
	prevEpisodeListIndex int

	// Recent searches
	recentSearches []string
	recentCursor   int

	// Watch history
	//
	// watchHistoryCache is the list the history screen shows; it is read
	// when the screen opens. pendingShowID steers the results of a
	// history resume back to the watched entry: the provider searches by
	// title, and the first result with the remembered id wins.
	watchHistoryCache []historyEntry
	pendingShowID     string

	// Watchlist
	//
	// watchlist holds the saved list of the account while the screen is
	// open. watchlistCover* cache the rendered artwork of the selected
	// entry, and watchlistCoverURL remembers which picture they belong
	// to: a download that comes back after the cursor moved is dropped.
	watchlist         *WatchlistState
	watchlistCover    string
	watchlistCoverW   int
	watchlistCoverR   int
	watchlistCoverURL string

	// AniList account screen
	anilistCursor int // 0 = tracking toggle, 1 = login/logout

	// animeScroll is the scroll offset of the details screen.
	//
	// A compact terminal cannot show the whole entry at once, and the
	// details screen has no list to scroll: the arrow keys move the text.
	animeScroll int

	progressTarget float64 // where the animation should stop

	// AniList tracking
	trackingEnabled bool
	anilistToken    string
	anilistUsername string

	// Kitty graphics
	kittyImageID uint32 // current image ID in terminal memory, for cleanup

	// Design system showcase
	themeScroll int

	// Pill switch (SUB / DUB) in the app bar: the character range so a
	// mouse click on either segment toggles the selected translation.
	psX1 int // first character column of the filled pill (active option)
	psX2 int // last character column + 1 (exclusive end)
}

func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = placeholderSearch
	// The search field is not focused on start: the application opens on
	// the navigation, and typing a character focuses the field. That also
	// keeps the menu key ("m") free for the navigation drawer.
	//
	// The prompt is empty: the field draws its own icon, so the widget only
	// renders the value.
	ti.Prompt = ""
	ts := textinput.DefaultStyles(true)
	ts.Focused.Prompt = theme.Fg(lipgloss.NewStyle(), theme.Primary)
	ts.Focused.Text = theme.Fg(lipgloss.NewStyle(), theme.Text)
	ts.Focused.Placeholder = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	ts.Blurred.Prompt = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	ts.Blurred.Text = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	ts.Blurred.Placeholder = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	ti.SetStyles(ts)

	translationType := config.GetString("translation_type")
	if translationType == "" {
		translationType = "sub"
	}
	hianimeProvider := hianime.NewProvider()
	hianimeProvider.SetTranslation(translationType)

	h := help.New()
	hs := help.DefaultStyles(true)
	hs.ShortKey = theme.Fg(lipgloss.NewStyle().Bold(true), theme.Primary)
	hs.ShortDesc = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	hs.FullKey = theme.Fg(lipgloss.NewStyle().Bold(true), theme.Primary)
	hs.FullDesc = theme.Fg(lipgloss.NewStyle(), theme.Muted)
	h.Styles = hs

	km := newKeymap()

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = theme.Fg(lipgloss.NewStyle(), theme.Primary)

	searchDelegate := &resultDelegate{focused: true}
	episodeDelegate := &resultDelegate{focused: true}

	return &SearchModel{
		state:        searchState,
		searchState:  NewSearchState(),
		episodeState: NewEpisodeState(),
		settingsState: &SettingsState{
			Quality: func() string {
				q := config.GetString("quality")
				if q == "" {
					return "auto"
				}
				return q
			}(),
			AniskipEnabled: config.GetBool("aniskip.enabled"),
		},
		textInput:            ti,
		help:                 h,
		keymap:               km,
		loading:              s,
		progress:             newProgress(),
		searchList:           makeSearchList(km, searchDelegate),
		episodeList:          makeEpisodeList(km, episodeDelegate),
		searchDelegate:       searchDelegate,
		episodeDelegate:      episodeDelegate,
		jikanClient:          jikan.NewClient("https://api.jikan.moe/v4"),
		anilistClient:        anilist.NewClient(),
		kitsuClient:          kitsu.NewClient(),
		hianimeProvider:      hianimeProvider,
		lastQuery:            "",
		metadataCache:        make(map[int]*MetadataPanel),
		kitsuAnimeCache:      make(map[int]*kitsu.Anime),
		episodeTitlesCache:   make(map[int][]string),
		episodeMetadataCache: make(map[string]*jikan.Episode),
		recentSearches:       loadRecentSearches(),
		watchlist:            &WatchlistState{},
		trackingEnabled:      config.GetBool("anilist.tracking.enabled") && config.GetString("anilist.token") != "",
		anilistToken:         config.GetString("anilist.token"),
		anilistUsername:      config.GetString("anilist.username"),
		nav:                  navState{expanded: config.GetString("ui.sidebar") != "collapsed", cursor: navIndexOf(navSearch)},
	}
}

// newProgress builds the progress bar of the shell.
//
// The bar is drawn in the accent colours of the design system: the filled
// part is the primary colour, the empty part the secondary one.
func newProgress() progress.Model {
	p := progress.New(
		progress.WithColors(roleColor(theme.Primary), roleColor(theme.Secondary)),
		progress.WithScaled(true),
	)
	p.SetWidth(40)
	return p
}

func (m *SearchModel) Init() tea.Cmd {
	// Ask the terminal about its capabilities; the answer is recorded by
	// HandlePrimaryDeviceAttributes. A terminal that does not answer
	// simply keeps the renderers the environment already proves.
	return tea.Raw(ansi.RequestPrimaryDeviceAttributes)
}

func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case uv.PrimaryDeviceAttributesEvent:
		// The terminal answered the capability query. Device attributes
		// are no longer used to pick a renderer, so the answer is ignored.
		// When it would change the chain, the artwork on screen would be
		// redrawn with the better protocol.
		if HandlePrimaryDeviceAttributes(msg) {
			return m, m.refreshCoverForProtocolChange()
		}
		return m, nil

	case tea.WindowSizeMsg:
		// Crossing the width threshold switches the drawer between its
		// persistent and its modal form. The modal drawer is closed on the
		// way: a drawer that opens by itself covers the screen the user
		// just resized.
		wasPersistent := m.navPersistent()
		m.resize(msg.Width, msg.Height)
		if wasPersistent != m.navPersistent() {
			m.nav.overlay = false
			m.nav.focused = false
			m.nav.contentInputFocus = false
		}
		// The cover was rendered for the size of the panel it lives in: a
		// resize asks for a new one. The ask is free when the size did not
		// change - ensureCover renders only a missing or mis-sized cover.
		return m, m.coverCmd()

	case tea.MouseMsg:
		// A click on the SUB/DUB pill switch toggles the translation.
		if msg, ok := msg.(tea.MouseClickMsg); ok && msg.Y == 0 {
			if m.pillSwitchClicked(msg.X) {
				m.toggleTranslation()
			}
			return m, nil
		}
		// A click on a recent search row re-runs that query. recentSearchAt
		// maps the content row (the app bar is row 0) onto the list.
		if msg, ok := msg.(tea.MouseClickMsg); ok {
			if click := m.recentSearchAt(msg.Y - 1); click >= 0 && click < len(m.recentSearches) {
				return m.selectRecentSearch(click)
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		cmds = append(cmds, cmd)

	case progressTickMsg:
		// Smoothly animate towards target
		if m.progressPercent < m.progressTarget {
			m.progressPercent += 0.08
			if m.progressPercent > m.progressTarget {
				m.progressPercent = m.progressTarget
			}
		}
		if m.searchState.Loading || m.episodeState.Loading || m.episodeState.Playing {
			cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
				return progressTickMsg{}
			}))
		}

	case tea.KeyMsg:
		// The design system showcase is reachable from every screen, even
		// while a text input has the focus: without this the key would be
		// typed into the search box.
		if m.state != themeState && key.Matches(msg, m.keymap.Theme) {
			m.prevState = m.state
			m.themeScroll = 0
			m.state = themeState
			return m, nil
		}

		// The navigation drawer owns the keyboard while it is open.
		if m.navHasFocus() {
			return m.handleNavKey(msg)
		}

		// The menu key. "m" is only a toggle when no text field holds the
		// focus - inside the search box it is a letter - so the drawer
		// stays reachable with ctrl+o.
		if key.Matches(msg, m.keymap.MenuAlt) {
			m.toggleDrawer()
			return m, nil
		}
		if key.Matches(msg, m.keymap.Menu) && !m.textInput.Focused() {
			m.toggleDrawer()
			return m, nil
		}

		if m.state == historyState {
			return m.handleHistoryKey(msg)
		}

		if m.state == watchlistState {
			return m.handleWatchlistKey(msg)
		}

		if m.state == searchState && m.textInput.Focused() {
			switch {
			case key.Matches(msg, m.keymap.Quit):
				m.prevState = m.state
				m.state = confirmQuitState
				return m, nil
			case key.Matches(msg, m.keymap.Back):
				if m.textInput.Value() == "" {
					m.textInput.Blur()
				} else {
					m.textInput.SetValue("")
				}
				return m, nil
			case key.Matches(msg, m.keymap.Select):
				// If recent searches are showing and cursor is on one, use it
				if m.textInput.Value() == "" && len(m.searchState.Results) == 0 && len(m.recentSearches) > 0 && m.recentCursor < len(m.recentSearches) {
					return m.selectRecentSearch(m.recentCursor)
				}
				query := m.textInput.Value()
				if len(query) >= 2 {
					m.lastQuery = query
					m.searchState.Query = query
					m.searchState.Loading = true
					m.progressPercent = 0
					m.progressTarget = 0.3
					m.textInput.Blur()
					cmds = append(cmds, m.doSearch(query))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
				return m, nil
			case msg.String() == "up", msg.String() == "down":
				m.textInput.Blur()
			case key.Matches(msg, m.keymap.Toggle):
				if m.searchState.TranslationType == "sub" {
					m.searchState.TranslationType = "dub"
				} else {
					m.searchState.TranslationType = "sub"
				}
				config.Set("translation_type", m.searchState.TranslationType)
				return m, nil
			case key.Matches(msg, m.keymap.Settings):
				m.prevState = m.state
				m.state = settingsState
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}

		if m.state == detailState && m.textInput.Focused() {
			switch {
			case key.Matches(msg, m.keymap.Quit):
				m.prevState = m.state
				m.state = confirmQuitState
				return m, nil
			case key.Matches(msg, m.keymap.Back):
				m.textInput.Blur()
				return m, nil
			case key.Matches(msg, m.keymap.Select):
				query := m.textInput.Value()
				if query != "" {
					m.selectEpisodeByNumber(query)
				}
				m.textInput.Blur()
				return m, nil
			case msg.String() == "up", msg.String() == "down":
				m.textInput.Blur()
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				m.filterEpisodesByNumber(m.textInput.Value())
				return m, tea.Batch(cmds...)
			}
		}

		if m.state == settingsState {
			switch msg.String() {
			case "up", "k":
				if m.settingsState.Cursor > 0 {
					m.settingsState.Cursor--
				}
			case "down", "j":
				if m.settingsState.Cursor < settingsRowCount-1 {
					m.settingsState.Cursor++
				}
			case "left", "h", "right", "l":
				m.settingsAdjust(msg.String() == "left" || msg.String() == "h")
			case "enter":
				switch m.settingsState.Cursor {
				case settingsRowAniList:
					m.state = anilistState
					m.anilistCursor = aniListStartCursor()
					return m, nil
				case settingsRowUpdate:
					if m.settingsState.UpdateStatus == "" || m.settingsState.UpdateStatus == "error" {
						m.settingsState.UpdateStatus = "checking"
						m.settingsState.UpdateError = nil
						return m, checkForUpdateCmd()
					} else if m.settingsState.UpdateStatus == "available" {
						m.settingsState.UpdateStatus = "updating"
						m.settingsState.UpdateError = nil
						return m, doSelfUpdateCmd(m.settingsState.UpdateVersion)
					}
				}
			case "esc":
				m.state = m.prevState
				return m, nil
			}
			return m, nil
		}

		if m.state == anilistState {
			switch {
			case key.Matches(msg, m.keymap.Quit):
				m.prevState = m.state
				m.confirmSelect = 1
				m.state = confirmQuitState
				return m, nil

			case key.Matches(msg, m.keymap.Back):
				m.state = m.prevState
				return m, nil

			case key.Matches(msg, m.keymap.Up), key.Matches(msg, m.keymap.Down):
				if !auth.IsLoggedIn() {
					// The tracking row exists only for a connected
					// account: the cursor never rests on a missing row.
					m.anilistCursor = 1
					return m, nil
				}
				if m.anilistCursor != 1 {
					m.anilistCursor = 1
				}
				return m, nil

			case key.Matches(msg, m.keymap.Select):
				switch m.anilistCursor {
				case 0:
					if !auth.IsLoggedIn() {
						m.anilistCursor = 1
						return m, nil
					}
					// The tracking switch is a real setting: it decides
					// whether a played episode is pushed to AniList.
					on := !config.GetBool("anilist.tracking.enabled")
					config.Set("anilist.tracking.enabled", on)
					m.trackingEnabled = on && m.anilistToken != ""
				case 1:
					if auth.IsLoggedIn() {
						m.prevState = m.state
						m.confirmSelect = 1
						m.state = confirmLogoutState
						return m, nil
					}
					m.state = anilistLoginState
					return m, doAniListLogin()
				}
				return m, nil
			}
			return m, nil
		}

		if m.state == confirmLogoutState {
			switch msg.String() {
			case "y", "enter":
				if m.confirmSelect == 0 {
					auth.Quiet = true
					err := auth.Logout()
					auth.Quiet = false
					if err == nil {
						m.trackingEnabled = false
						m.anilistToken = ""
						m.anilistUsername = ""
					}
				}
				m.state = anilistState
				m.anilistCursor = 1
				return m, nil
			case "n", "esc":
				m.state = anilistState
				m.anilistCursor = 1
				return m, nil
			case "left", "right", "h", "l", "up", "k", "down", "j", "tab":
				if m.confirmSelect == 0 {
					m.confirmSelect = 1
				} else {
					m.confirmSelect = 0
				}
				return m, nil
			}
			return m, nil
		}

		if m.state == anilistLoginState {
			switch msg.String() {
			case "esc":
				m.state = m.prevState
				return m, nil
			}
			return m, nil
		}

		if m.state == themeState {
			switch msg.String() {
			case "esc":
				m.state = m.prevState
				m.themeScroll = 0
				return m, nil
			case "up", "k":
				if m.themeScroll > 0 {
					m.themeScroll--
				}
				return m, nil
			case "down", "j":
				m.themeScroll++
				return m, nil
			case "home", "g":
				m.themeScroll = 0
				return m, nil
			case "end", "G":
				m.themeScroll = 1 << 20
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		if m.state == confirmQuitState {
			switch msg.String() {
			case "left", "h":
				m.confirmSelect = 0
			case "right", "l":
				m.confirmSelect = 1
			case "enter":
				if m.confirmSelect == 0 {
					return m, tea.Quit
				}
				m.state = m.prevState
				return m, nil
			case "y":
				return m, tea.Quit
			case "n", "esc":
				m.state = m.prevState
				return m, nil
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			m.prevState = m.state
			m.confirmSelect = 1
			m.state = confirmQuitState
			return m, nil

		case key.Matches(msg, m.keymap.Back):
			switch m.state {
			case animeState, detailState:
				return m, m.goBack()
			}

		case key.Matches(msg, m.keymap.Search):
			return m, m.gotoSearch()

		case key.Matches(msg, m.keymap.Toggle):
			if m.searchState.TranslationType == "sub" {
				m.searchState.TranslationType = "dub"
			} else {
				m.searchState.TranslationType = "sub"
			}
			config.Set("translation_type", m.searchState.TranslationType)

			if m.state == detailState && m.episodeState.AnimeID != "" {
				m.episodeState.Loading = true
				m.progressPercent = 0
				m.progressTarget = 0.4
				selectedAnime := m.searchState.Results[m.searchState.Selected]
				if selectedAnime != nil {
					cmds = append(cmds, m.fetchEpisodes(selectedAnime))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
				}
			} else if m.lastQuery != "" {
				m.searchState.Loading = true
				m.progressPercent = 0
				m.progressTarget = 0.3
				cmds = append(cmds, m.doSearch(m.lastQuery))
				cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
					return progressTickMsg{}
				}))
			}

		case key.Matches(msg, m.keymap.Retry):
			// The error states advertise ctrl+r: it re-runs the request
			// that failed instead of leaving the user to retype a query.
			switch m.state {
			case searchState:
				if m.lastQuery != "" {
					m.searchState.Loading = true
					m.searchState.Err = nil
					m.progressPercent = 0
					m.progressTarget = 0.3
					cmds = append(cmds, m.doSearch(m.lastQuery))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
				}
			case detailState:
				if m.searchState.Selected < len(m.searchState.Results) {
					selectedAnime := m.searchState.Results[m.searchState.Selected]
					if selectedAnime != nil && m.episodeState.AnimeID != "" {
						m.episodeState.Loading = true
						m.episodeState.Err = nil
						m.progressPercent = 0
						m.progressTarget = 0.4
						cmds = append(cmds, m.fetchEpisodes(selectedAnime))
						cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
							return progressTickMsg{}
						}))
					}
				}
			}

		case key.Matches(msg, m.keymap.Settings):
			m.prevState = m.state
			m.state = settingsState
			return m, nil

		case key.Matches(msg, m.keymap.Select):
			switch m.state {
			case searchState:
				return m, m.openSelectedAnime()

			case animeState:
				return m, m.openEpisodes()

			case detailState:
				m.episodeState.Selected = m.episodeList.Index()
				if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
					selectedAnime := m.searchState.Results[m.searchState.Selected]
					selectedEpisode := m.episodeState.Episodes[m.episodeState.Selected]
					// The history answers "what was I watching?": every
					// playback writes one row, newest first.
					recordWatch(selectedAnime.Name, selectedAnime.ShowID, selectedEpisode)
					m.episodeState.Playing = true
					m.progressPercent = 0
					m.progressTarget = 0.5
					cmds = append(cmds, m.playEpisode(selectedAnime, selectedEpisode))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
			}
		}

		// The details screen has no list: the arrow keys scroll the text.
		if m.state == animeState {
			switch msg.String() {
			case "up", "k":
				if m.animeScroll > 0 {
					m.animeScroll--
				}
				return m, nil
			case "down", "j":
				m.animeScroll++
				return m, nil
			case "home", "g":
				m.animeScroll = 0
				return m, nil
			}
		}

		// Typing printable characters (not list nav keys) refocuses search bar
		keyStr := msg.String()
		isListNav := keyStr == "g" || keyStr == "G" || keyStr == "j" || keyStr == "k"
		if m.state == searchState && !m.textInput.Focused() && len(keyStr) == 1 && !isListNav {
			m.textInput.Focus()
			m.textInput.SetValue("")
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		if m.state == searchState && !m.textInput.Focused() && len(m.searchState.Results) > 0 {
			var cmd tea.Cmd
			m.searchList, cmd = m.searchList.Update(msg)
			cmds = append(cmds, cmd)
			if m.searchList.Index() != m.prevSearchListIndex {
				m.prevSearchListIndex = m.searchList.Index()
				m.searchState.Selected = m.searchList.Index()

				// Look up cached metadata instantly — no network call needed
				if meta, ok := m.metadataCache[m.searchList.Index()]; ok {
					m.searchState.Metadata = meta
					m.searchState.MetadataLoading = false
					cmds = append(cmds, m.coverCmd())
				} else {
					// Not cached (AniList batch may not have covered this one) — fetch individually
					m.searchState.MetadataLoading = true
					cmds = append(cmds, m.fetchMetadata())
				}
			}
		} else if m.state == searchState && !m.textInput.Focused() && len(m.searchState.Results) == 0 && len(m.recentSearches) > 0 {
			// Navigate and select recent searches
			switch {
			case msg.String() == "up" || msg.String() == "k":
				if m.recentCursor > 0 {
					m.recentCursor--
				}
			case msg.String() == "down" || msg.String() == "j":
				if m.recentCursor < len(m.recentSearches)-1 {
					m.recentCursor++
				}
			case key.Matches(msg, m.keymap.Select):
				return m.selectRecentSearch(m.recentCursor)
			case key.Matches(msg, m.keymap.Back):
				// Esc from recent searches focuses the search bar
				m.textInput.Focus()
				return m, nil
			}
		} else if m.state == detailState && !m.textInput.Focused() && len(m.episodeState.Episodes) > 0 {
			if msg.String() >= "0" && msg.String() <= "9" {
				m.textInput.Focus()
				m.textInput.SetValue("")
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				m.filterEpisodesByNumber(m.textInput.Value())
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.episodeList, cmd = m.episodeList.Update(msg)
			cmds = append(cmds, cmd)
			if m.episodeList.Index() != m.prevEpisodeListIndex {
				m.prevEpisodeListIndex = m.episodeList.Index()
				m.episodeState.Selected = m.episodeList.Index()

				// Debounce episode metadata fetch via tea.Cmd
				idx := m.episodeList.Index()
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
					return episodeMetadataDebounceMsg{Index: idx}
				}))
			} else {
				m.episodeState.Selected = m.episodeList.Index()
			}
		}

	case SearchResultsMsg:
		m.searchState.Results = msg.Results
		m.searchState.Loading = false
		m.progressTarget = 0.6 // search done, metadata still loading
		m.searchState.Selected = 0
		m.textInput.Blur()
		m.episodeTitlesCacheMu.Lock()
		m.episodeTitlesCache = make(map[int][]string)
		m.episodeTitlesCacheMu.Unlock()
		m.metadataCache = make(map[int]*MetadataPanel)
		m.kitsuAnimeCache = make(map[int]*kitsu.Anime)
		m.metadataStale = false
		m.progress = newProgress()
		m.progress.SetWidth(m.width)
		// A resume from the history steers the results back to the watched
		// entry: the provider searched by title, and the first result with
		// the remembered streaming id wins.
		if m.pendingShowID != "" {
			for i, anime := range m.searchState.Results {
				if anime.ShowID == m.pendingShowID {
					m.searchState.Selected = i
					break
				}
			}
			m.pendingShowID = ""
		}
		// Resize list to two-column layout now that results are available
		m.resize(m.width, m.height)
		if len(msg.Results) > 0 {
			saveRecentSearch(m.lastQuery)
			m.recentSearches = loadRecentSearches()
			// Resolve AniList/MAL IDs in the background, then batch-fetch the
			// metadata for all results in a single API call. The Kitsu
			// resolver runs alongside and fills AniList/MAL IDs Kitsu knows
			// and AniList did not, enabling AniSkip even for titles AniList
			// lists under a different name.
			cmds = append(cmds, m.resolveIDs(msg.Results))
			cmds = append(cmds, m.resolveIDsViaKitsu(msg.Results))
		}
		m.updateSearchList()

	case IDsResolvedMsg:
		// IDs are filled in place; now the metadata batch can run.
		m.searchState.Results = msg.Results
		if len(msg.Results) > 0 {
			m.searchState.MetadataLoading = true
			m.loading = spinner.New()
			m.loading.Spinner = spinner.Points
			m.loading.Style = theme.Fg(lipgloss.NewStyle(), theme.Primary)
			cmds = append(cmds, m.loading.Tick)
			cmds = append(cmds, m.fetchMetadataBatch(msg.Results))
		}

	case MALIDsResolvedMsg:
		// Apply the Kitsu IDs on the UI thread, but only where the
		// AniList resolution did not already provide them. Kitsu returns
		// both AniList and MAL ids, so both fields are patched. The Kitsu
		// entries are kept as the metadata fallback for movies and OVAs
		// AniList does not know by id.
		for idx, ids := range msg.IDs {
			if idx < 0 || idx >= len(m.searchState.Results) {
				continue
			}
			if m.searchState.Results[idx].AniListID == 0 {
				m.searchState.Results[idx].AniListID = ids.AniListID
			}
			if m.searchState.Results[idx].MALID == 0 {
				m.searchState.Results[idx].MALID = ids.MALID
			}
		}
		for idx, anime := range msg.Animes {
			m.kitsuAnimeCache[idx] = anime
		}
		// The first batch ran with whatever ids AniList resolved; the ids
		// Kitsu just added may cover entries it skipped. Fetch those once
		// the running batch is done, so the two passes never overlap.
		if m.searchState.MetadataLoading {
			m.metadataStale = true
		} else if m.hasUncachedResults() {
			// The pass runs quietly: the list fills in when it returns,
			// no spinner is needed for the few entries it covers.
			cmds = append(cmds, m.fetchMetadataBatch(m.searchState.Results))
		}

	case MetadataBatchLoadedMsg:
		// Cache all metadata from the batch result; an entry already in
		// the cache was richer (AniList/Jikan/Kitsu) and is not downgraded
		// by the Kitsu or hianime fallback that a rerun may carry. The
		// hianime placeholder is provisional and is upgraded by any real
		// source.
		for idx, meta := range msg.MetadataMap {
			if meta == nil {
				continue
			}
			if m.hasRealMetadata(idx) {
				continue
			}
			m.metadataCache[idx] = meta
		}
		if m.metadataStale {
			// Kitsu IDs arrived while this batch was running: the ids
			// it added were not in the request. Run one more pass, which
			// skips everything already cached.
			m.metadataStale = false
			cmds = append(cmds, m.fetchMetadataBatch(m.searchState.Results))
		} else {
			m.searchState.MetadataLoading = false
		}
		// The provider pages are fetched after this batch: their panels
		// are provisional and must not delay the real ones.
		if msg.FollowUp != nil {
			cmds = append(cmds, msg.FollowUp)
		}
		m.progressTarget = 1.0
		// The rows carry the alternative title and the status of their
		// entry: the list is rebuilt with what the batch brought.
		m.updateSearchList()
		// Show metadata for the currently selected item
		if meta, ok := m.metadataCache[m.searchState.Selected]; ok {
			m.searchState.Metadata = meta
			cmds = append(cmds, m.coverCmd())
		}

	case metadataDebounceMsg:
		// Navigation changed — look up cached metadata (no network call needed)
		if msg.Index == m.searchState.Selected {
			if meta, ok := m.metadataCache[msg.Index]; ok {
				m.searchState.Metadata = meta
				m.searchState.MetadataLoading = false
				cmds = append(cmds, m.coverCmd())
			}
			// If not in cache, batch already failed or ID wasn't available
		}

	case MetadataLoadedMsg:
		// Fallback for individual fetch (Jikan-only items)
		if msg.Index == m.searchState.Selected {
			m.searchState.Metadata = msg.Metadata
			m.searchState.MetadataLoading = false
			m.progressTarget = 1.0
			m.updateSearchList()
			cmds = append(cmds, m.coverCmd())
		}

	case EpisodesLoadedMsg:
		m.episodeState.Episodes = msg.Episodes
		m.episodeState.EpisodeTitles = msg.EpisodeTitles
		m.episodeState.Loading = false
		m.progressTarget = 0.7 // episodes loaded, metadata still loading
		m.progress = newProgress()
		m.progress.SetWidth(m.width)
		if msg.Error != nil {
			m.episodeState.Err = msg.Error
		}
		m.updateEpisodeList()
		if len(msg.Episodes) > 0 {
			m.episodeState.MetadataLoading = true
			cmds = append(cmds, m.fetchEpisodeMetadata())
		}

	case episodeMetadataDebounceMsg:
		if msg.Index == m.episodeState.Selected {
			m.episodeState.MetadataLoading = true
			cmds = append(cmds, m.fetchEpisodeMetadata())
		}

	case EpisodeMetadataFetchTriggered:

	case EpisodeMetadataLoadedMsg:
		if msg.CacheKey != "" && msg.RawEpisode != nil {
			m.episodeMetadataCache[msg.CacheKey] = msg.RawEpisode
		}
		if msg.Index == m.episodeState.Selected {
			m.episodeState.EpisodeMetadata = msg.Metadata
			m.episodeState.MetadataLoading = false
		}

	case CoverImageLoadedMsg:
		if m.state == watchlistState {
			// The watchlist recognises its covers by the URL they were
			// downloaded from: a download that comes back after the
			// cursor moved belongs to an entry that is no longer shown.
			if entry := m.watchlistSelected(); entry != nil && msg.Rendered != "" && msg.URL == entry.Cover {
				m.watchlistCover = msg.Rendered
				m.watchlistCoverW = msg.Width
				m.watchlistCoverR = msg.Rows
			}
		}
		if msg.Rendered != "" {
			// Update the cached metadata with the rendered cover
			if meta, ok := m.metadataCache[msg.Index]; ok {
				meta.CoverImage = msg.Rendered
				meta.CoverWidth = msg.Width
				meta.CoverRows = msg.Rows
			}
			// Also update current metadata if it's the selected item
			if m.searchState.Metadata != nil && msg.Index == m.searchState.Selected {
				m.searchState.Metadata.CoverImage = msg.Rendered
				m.searchState.Metadata.CoverWidth = msg.Width
				m.searchState.Metadata.CoverRows = msg.Rows
			}
			// For Kitty protocol the image is transmitted outside the view
			// text: the ultraviolet renderer strips sequences it cannot read
			// (APC), so tea.Raw() sends them directly to the terminal. Delete
			// the old image first, then transmit the new.
			if msg.KittyTransmitSeq != "" {
				oldID := m.kittyImageID
				m.kittyImageID = msg.KittyImageID
				cmds = append(cmds,
					DeleteKittyImageCmd(oldID),
					tea.Raw(msg.KittyTransmitSeq),
				)
			}
		}

	case PlayStreamMsg:
		m.episodeState.Playing = false
		m.progressPercent = 1.0
		m.progressTarget = 1.0
		m.progress = newProgress()
		m.progress.SetWidth(m.width)
		// Fire tracking update
		if m.trackingEnabled && len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
			selectedAnime := m.searchState.Results[m.searchState.Selected]
			if selectedAnime.AniListID > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
				epNumStr := m.episodeState.Episodes[m.episodeState.Selected]
				epNum, _ := strconv.Atoi(epNumStr)
				if epNum > 0 {
					cmds = append(cmds, m.updateTrackingCmd(selectedAnime.AniListID, epNum, selectedAnime.EpisodeCount))
				}
			}
		}

	case TrackingStatusLoadedMsg:
		m.episodeState.TrackingStatus = msg.Status
		m.episodeState.TrackingProgress = msg.Progress
		// The rows carry the watched badges: they must be rebuilt once
		// the progress is known, the episodes may have loaded first.
		m.updateEpisodeList()

	case TrackingUpdateMsg:
		if msg.Err != nil {
			log.Printf("[anilix] tracking update failed: %v\n", msg.Err)
		} else {
			m.episodeState.TrackingStatus = msg.Status
			m.episodeState.TrackingProgress = msg.Progress
			// The episode just played moved the progress: the badge of
			// the next episode becomes the play glyph.
			m.updateEpisodeList()
		}

	case WatchlistLoadedMsg:
		if m.watchlist == nil {
			m.watchlist = &WatchlistState{}
		}
		m.watchlist.Loading = false
		m.watchlist.Err = msg.Err
		m.watchlist.Entries = msg.Entries
		m.watchlist.Cursor = 0
		m.watchlist.Scroll = 0
		cmds = append(cmds, m.watchlistCoverCmd())

	case AniListLoginMsg:
		m.state = settingsState
		if msg.Err != nil {
			log.Printf("[anilix] anilist login failed: %v\n", msg.Err)
		} else {
			// Login succeeded — update state
			m.anilistToken = config.GetString("anilist.token")
			m.anilistUsername = auth.GetUsername()
			m.trackingEnabled = m.anilistToken != ""
		}

	case UpdateCheckMsg:
		if msg.Err != nil {
			m.settingsState.UpdateStatus = "error"
			m.settingsState.UpdateError = msg.Err
		} else if update.IsUpdateAvailable(version.Version, msg.LatestVersion) {
			m.settingsState.UpdateStatus = "available"
			m.settingsState.UpdateVersion = msg.LatestVersion
		} else {
			m.settingsState.UpdateStatus = "up_to_date"
		}

	case UpdateCompleteMsg:
		if msg.Err != nil {
			m.settingsState.UpdateStatus = "error"
			m.settingsState.UpdateError = msg.Err
		} else {
			m.settingsState.UpdateStatus = "updated"
		}

	case TUIErrorMsg:
		m.episodeState.Playing = false
		m.progressPercent = 1.0
		m.progressTarget = 1.0
		m.progress = newProgress()
		m.progress.SetWidth(m.width)
		m.episodeState.Err = msg.Err
	}

	return m, tea.Batch(cmds...)
}

func (m *SearchModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.updateLayout()
}

// updateLayout recomputes the geometry of the application shell and resizes
// the components that live in the content region.
//
// The navigation drawer takes part of the screen on wide terminals, so the
// content region is smaller than the terminal. Everything that is laid out
// inside the content region has to be measured against the content region.
func (m *SearchModel) updateLayout() {
	regions := m.shellRegions()
	m.contentWidth = regions.contentWidth
	m.contentHeight = regions.contentHeight

	padX, padY := 4, 2
	styledWidth := m.contentWidth - padX
	styledHeight := m.contentHeight - padY

	if styledWidth < 20 {
		styledWidth = 20
	}
	if styledHeight < 1 {
		styledHeight = 1
	}

	usedHeight := 7
	listHeight := styledHeight - usedHeight
	if listHeight < 1 {
		listHeight = 1
	}

	// The search field draws its own frame: the border and the padding take
	// four cells, the icon and its space take two more. The text input only
	// renders the value, so it gets the rest.
	inputWidth := m.contentWidth - 6
	if inputWidth < 8 {
		inputWidth = 8
	}

	m.textInput.SetWidth(inputWidth)
	m.help.SetWidth(styledWidth)
	m.progress.SetWidth(m.contentWidth)

	// Size lists based on current state layout
	switch m.state {
	case searchState:
		if len(m.searchState.Results) > 0 {
			leftW, _, _, _ := searchLayout(m.contentWidth, m.contentHeight)
			m.searchList.SetSize(leftW, listHeight)
		} else {
			m.searchList.SetSize(styledWidth, listHeight)
		}
		m.episodeList.SetSize(styledWidth, listHeight)
	case detailState:
		_, rightW, _, _ := detailLayout(m.contentWidth, m.contentHeight)
		m.searchList.SetSize(styledWidth, listHeight)
		m.episodeList.SetSize(rightW, listHeight)
	default:
		m.searchList.SetSize(styledWidth, listHeight)
		m.episodeList.SetSize(styledWidth, listHeight)
	}
}

func (m *SearchModel) updateSearchList() {
	items := make([]list.Item, 0, len(m.searchState.Results))
	for i, anime := range m.searchState.Results {
		// The row carries the metadata of its entry when it is already
		// known: the alternative title and the status are part of the row.
		items = append(items, animeItem{anime: anime, meta: m.metadataCache[i]})
	}
	cmd := m.searchList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.searchList.Select(m.searchState.Selected)
}

func (m *SearchModel) updateEpisodeList() {
	items := make([]list.Item, 0, len(m.episodeState.Episodes))
	for i, ep := range m.episodeState.Episodes {
		title := ""
		if len(m.episodeState.EpisodeTitles) > i {
			title = m.episodeState.EpisodeTitles[i]
		}
		watched, nextUp := m.episodeTracking(ep)
		items = append(items, episodeItem{number: ep, title: title, watched: watched, nextUp: nextUp})
	}
	cmd := m.episodeList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.episodeList.Select(m.episodeState.Selected)
}

// episodeTracking reports the watched and next-up state of one episode.
//
// The progress comes from AniList: every episode at or before the progress
// is watched, the one right after it is the one to play next. A model
// without tracking marks nothing, so the rows stay as they were.
func (m *SearchModel) episodeTracking(ep string) (watched, nextUp bool) {
	if !m.trackingEnabled || m.episodeState.TrackingStatus == "" {
		return false, false
	}
	progress := m.episodeState.TrackingProgress
	if progress <= 0 {
		return false, false
	}
	number, err := strconv.Atoi(ep)
	if err != nil {
		return false, false
	}
	if number <= progress {
		return true, false
	}
	return false, number == progress+1
}

func (m *SearchModel) filterEpisodesByNumber(query string) {
	if query == "" {
		m.updateEpisodeList()
		return
	}
	items := make([]list.Item, 0)
	for i, ep := range m.episodeState.Episodes {
		if strings.Contains(ep, query) {
			title := ""
			if len(m.episodeState.EpisodeTitles) > i {
				title = m.episodeState.EpisodeTitles[i]
			}
			watched, nextUp := m.episodeTracking(ep)
			items = append(items, episodeItem{number: ep, title: title, watched: watched, nextUp: nextUp})
		}
	}
	cmd := m.episodeList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	if len(items) > 0 {
		m.episodeList.Select(0)
	}
}

func (m *SearchModel) selectEpisodeByNumber(query string) {
	for i, ep := range m.episodeState.Episodes {
		if ep == query || strings.Contains(ep, query) {
			m.episodeState.Selected = i
			m.episodeList.Select(i)
			return
		}
	}
}

func (m *SearchModel) GetSelectedAnime() *source.Anime {
	if m.searchState.Selected < len(m.searchState.Results) {
		return m.searchState.Results[m.searchState.Selected]
	}
	return nil
}

// pillSwitchClicked reports whether a mouse click at column x hit the
// SUB/DUB pill switch in the app bar. The bounds are set by renderAppBar
// and describe the frame the click lands on.
func (m *SearchModel) pillSwitchClicked(x int) bool {
	return m.psX1 < m.psX2 && x >= m.psX1 && x < m.psX2
}

// recentSearchAt maps a content-region row (0 = the row under the app bar)
// to an index into the recent searches, or -1 when the row does not hold a
// recent search entry.
//
// The recent searches are on screen only on a fresh search screen. The
// bordered search field takes three rows, then a blank line, the section
// header and another blank line sit between it and the entries: the first
// entry is content row 6, and the entries follow one per row.
func (m *SearchModel) recentSearchAt(y int) int {
	if m.state != searchState || m.textInput.Value() != "" ||
		m.searchState.Loading || m.searchState.Err != nil ||
		len(m.searchState.Results) > 0 || len(m.recentSearches) == 0 {
		return -1
	}
	// The modal drawer covers the content region: the recent searches are
	// not on screen, so a click must not reach them through the drawer.
	if !m.navPersistent() && m.nav.overlay {
		return -1
	}
	index := y - recentSearchesTop
	if index < 0 || index >= len(m.recentSearches) {
		return -1
	}
	return index
}

// recentSearchesTop is the content-region row of the first recent search
// entry: the three rows of the bordered field, a blank line, the section
// header and the blank line under it.
const recentSearchesTop = 6

// selectRecentSearch starts a search for the recent query at the index,
// exactly as the enter key does on the same screen.
func (m *SearchModel) selectRecentSearch(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.recentSearches) {
		return m, nil
	}
	m.recentCursor = index
	query := m.recentSearches[index]
	m.textInput.SetValue(query)
	m.lastQuery = query
	m.searchState.Query = query
	m.searchState.Loading = true
	m.progressPercent = 0
	m.progressTarget = 0.3
	m.textInput.Blur()
	return m, tea.Batch(
		m.doSearch(query),
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return progressTickMsg{}
		}),
	)
}

// toggleTranslation flips the selected translation between sub and dub,
// wiring the change through the state, the configuration and the provider
// exactly as the ctrl+t key does.
func (m *SearchModel) toggleTranslation() {
	s := &m.searchState.TranslationType
	if *s != "sub" {
		*s = "sub"
	} else {
		*s = "dub"
	}
	m.hianimeProvider.SetTranslation(*s)
	config.Set("translation_type", *s)
}

func RunSearch() (*SelectionResult, error) {
	model := NewSearchModel()
	p := tea.NewProgram(model)

	_, err := p.Run()
	if err != nil {
		return nil, err
	}

	return model.selectedResult, nil
}
