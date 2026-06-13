package tui

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/history"
	allanime "github.com/hishantik/anilix/provider/allanime"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
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

	allanimeClient   *allanime.AllanimeClient
	jikanClient      *jikan.JikanClient
	anilistClient    *anilist.Client
	allanimeProvider *allanime.AllanimeProvider

	prevState     tuiState
	confirmSelect int

	settingsState *SettingsState

	selectedResult *SelectionResult
	lastQuery      string

	width  int
	height int

	progressPercent float64

	prevSearchListIndex  int
	metadataCache        map[int]*MetadataPanel
	episodeTitlesCache   map[int][]string
	episodeTitlesCacheMu sync.Mutex
	episodeMetadataCache map[string]*jikan.Episode
	prevEpisodeListIndex int

	// Recent searches
	recentSearches []string
	recentCursor   int

	progressTarget float64 // where the animation should stop

	// AniList tracking
	trackingEnabled bool
	anilistToken    string
	anilistUsername string

	// Kitty graphics
	kittyImageID uint32 // current image ID in terminal memory, for cleanup

	// Home screen
	homeScreen *HomeState
	hist       *history.History
}

func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search anime..."
	ti.Focus()
	ti.Prompt = "  "
	ts := textinput.DefaultStyles(true)
	ts.Focused.Prompt = lipgloss.NewStyle().Foreground(Theme.Primary).Padding(0, 1)
	ts.Focused.Text = lipgloss.NewStyle().Foreground(Theme.Text)
	ts.Focused.Placeholder = lipgloss.NewStyle().Foreground(Theme.Faint)
	ti.SetStyles(ts)

	translationType := config.GetString("translation_type")
	if translationType == "" {
		translationType = "sub"
	}
	allanimeProvider := allanime.NewAllanimeProvider()
	allanimeProvider.SetTranslation(translationType)

	h := help.New()
	hs := help.DefaultStyles(true)
	hs.ShortKey = lipgloss.NewStyle().Foreground(Theme.Primary)
	hs.ShortDesc = lipgloss.NewStyle().Foreground(Theme.Faint)
	hs.FullKey = lipgloss.NewStyle().Foreground(Theme.Primary)
	hs.FullDesc = lipgloss.NewStyle().Foreground(Theme.Faint)
	h.Styles = hs

	km := newKeymap()

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(Theme.Primary)

	p := progress.New(progress.WithColors(Theme.Primary, Theme.Secondary), progress.WithScaled(true))
	p.SetWidth(40)

	return &SearchModel{
		state:            homeState,
		searchState:      NewSearchState(),
		episodeState:     NewEpisodeState(),
		homeScreen:       NewHomeState(),
		hist:             history.New(),
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
		progress:             p,
		searchList:           makeSearchList(km),
		episodeList:          makeEpisodeList(km),
		allanimeClient:       allanime.NewAllanimeClient(),
		jikanClient:          jikan.NewClient("https://api.jikan.moe/v4"),
		anilistClient:        anilist.NewClient(),
		allanimeProvider:     allanimeProvider,
		lastQuery:            "",
		metadataCache:        make(map[int]*MetadataPanel),
		episodeTitlesCache:   make(map[int][]string),
		episodeMetadataCache: make(map[string]*jikan.Episode),
		recentSearches:       loadRecentSearches(),
		trackingEnabled:      config.GetBool("anilist.tracking.enabled") && config.GetString("anilist.token") != "",
		anilistToken:         config.GetString("anilist.token"),
		anilistUsername:      config.GetString("anilist.username"),
	}
}

func (m *SearchModel) Init() tea.Cmd {
	return tea.Batch(
		fetchHomeHistoryCmd(m.hist),
		fetchHomeTrendingCmd(m.anilistClient),
		fetchHomePopularCmd(m.jikanClient),
		fetchHomeGenreCmd(m.anilistClient, "Action"),
		fetchHomeGenreCmd(m.anilistClient, "Romance"),
		fetchHomeGenreCmd(m.anilistClient, "Comedy"),
		fetchHomeGenreCmd(m.anilistClient, "Fantasy"),
		m.loading.Tick,
	)
}

func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

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
					query := m.recentSearches[m.recentCursor]
					m.textInput.SetValue(query)
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
				if m.settingsState.Cursor < 3 {
					m.settingsState.Cursor++
				}
			case "left", "h", "right", "l":
				if m.settingsState.Cursor == 0 {
					qualities := []string{"best", "1080p", "720p", "480p", "360p", "auto"}
					idx := 0
					for i, q := range qualities {
						if q == m.settingsState.Quality {
							idx = i
							break
						}
					}
					if msg.String() == "left" || msg.String() == "h" {
						idx = (idx - 1 + len(qualities)) % len(qualities)
					} else {
						idx = (idx + 1) % len(qualities)
					}
					m.settingsState.Quality = qualities[idx]
					config.Set("quality", m.settingsState.Quality)
				} else if m.settingsState.Cursor == 1 {
					m.settingsState.AniskipEnabled = !m.settingsState.AniskipEnabled
					config.Set("aniskip.enabled", m.settingsState.AniskipEnabled)
				}
			case "enter":
				if m.settingsState.Cursor == 2 {
					if auth.IsLoggedIn() {
						// Logout immediately
						auth.Quiet = true
						err := auth.Logout()
						auth.Quiet = false
						if err == nil {
							m.trackingEnabled = false
							m.anilistToken = ""
							m.anilistUsername = ""
						}
					} else {
						// Start OAuth login flow
						m.state = anilistLoginState
						return m, doAniListLogin()
					}
				} else if m.settingsState.Cursor == 3 {
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

		if m.state == anilistLoginState {
			switch msg.String() {
			case "esc":
				m.state = settingsState
				return m, nil
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

		// Home screen navigation
		if m.state == homeState {
			switch msg.String() {
			case "up", "k":
				if m.homeScreen.ActiveSection > 0 {
					m.homeScreen.ActiveSection--
				}
			case "down", "j":
				if m.homeScreen.ActiveSection < len(m.homeScreen.Sections)-1 {
					m.homeScreen.ActiveSection++
				}
			case "left", "h":
				sec := m.homeScreen.Sections[m.homeScreen.ActiveSection]
				if sec.Selected > 0 {
					sec.Selected--
				}
			case "right", "l":
				sec := m.homeScreen.Sections[m.homeScreen.ActiveSection]
				if sec.Selected < len(sec.Items)-1 {
					sec.Selected++
				}
			case "enter":
				return m, m.selectHomeItem()
			case "/":
				m.prevState = homeState
				m.state = searchState
				m.textInput.Focus()
				m.textInput.SetValue("")
				return m, nil
			case "ctrl+s":
				m.prevState = m.state
				m.state = settingsState
				return m, nil
			case "ctrl+t":
				if m.searchState.TranslationType == "sub" {
					m.searchState.TranslationType = "dub"
				} else {
					m.searchState.TranslationType = "sub"
				}
				config.Set("translation_type", m.searchState.TranslationType)
				return m, nil
			case "ctrl+c":
				m.prevState = m.state
				m.confirmSelect = 1
				m.state = confirmQuitState
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
			if m.state == detailState {
				// Return to home if we came from home, otherwise search
				if m.prevState == homeState {
					m.state = homeState
				} else {
					m.state = searchState
				}
				m.episodeState = NewEpisodeState()
				m.textInput.Placeholder = "Type to search..."
				if m.kittyImageID != 0 {
					id := m.kittyImageID
					m.kittyImageID = 0
					return m, DeleteKittyImageCmd(id)
				}
				return m, nil
			}

		case key.Matches(msg, m.keymap.Search):
			if m.state == detailState {
				m.state = searchState
				m.episodeState = NewEpisodeState()
				if m.kittyImageID != 0 {
					id := m.kittyImageID
					m.kittyImageID = 0
					return m, tea.Batch(DeleteKittyImageCmd(id), nil)
				}
			}
			m.textInput.Focus()
			m.textInput.SetValue(m.lastQuery)
			return m, nil

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
					cmds = append(cmds, m.fetchEpisodes(m.episodeState.AnimeID, selectedAnime.MALID))
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

		case key.Matches(msg, m.keymap.Settings):
			m.prevState = m.state
			m.state = settingsState
			return m, nil

		case key.Matches(msg, m.keymap.Select):
			if m.state == searchState {
				m.searchState.Selected = m.searchList.Index()
				if len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
					anime := m.searchState.Results[m.searchState.Selected]
					if anime.AllAnimeID != "" {
						m.state = detailState
						m.textInput.Placeholder = "Search episode..."
						m.episodeState.AnimeID = anime.AllAnimeID
						m.episodeState.Loading = true
						m.progressPercent = 0
						m.progressTarget = 0.4
						// Compute resume episode from history (AniList progress overrides later)
						m.computeResumeEpisode()
						m.episodeState.ResumeFocus = m.episodeState.ResumeEpisode > 0
						cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
						cmds = append(cmds, m.fetchTrackingStatusCmd(anime.AniListID))
						cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
							return progressTickMsg{}
						}))
					}
				}
			} else if m.state == detailState {
				// When resume card has focus, Enter is handled below in the episode list block
				if m.episodeState.ResumeFocus && m.episodeState.ResumeEpisode > 0 {
					break // skip episode list selection — handled in episode list block below
				}
				m.episodeState.Selected = m.episodeList.Index()
				if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
					selectedAnime := m.searchState.Results[m.searchState.Selected]
					selectedEpisode := m.episodeState.Episodes[m.episodeState.Selected]
					m.episodeState.Playing = true
					m.progressPercent = 0
					m.progressTarget = 0.5
					cmds = append(cmds, m.playEpisode(selectedAnime.AllAnimeID, selectedEpisode, selectedAnime.Name, selectedAnime.MALID))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
			}
		}

		// Typing printable characters (not list nav keys) refocuses search bar
		keyStr := msg.String()
		isListNav := keyStr == "g" || keyStr == "G" || keyStr == "j" || keyStr == "k"
		if m.state == searchState && !m.textInput.Focused() && len(m.searchState.Results) > 0 && len(keyStr) == 1 && !isListNav {
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
					// Trigger cover download if not already loaded
					if meta.Cover != "" && meta.CoverImage == "" {
						leftW, _, _, _ := detailLayout(m.width, m.height)
						cmds = append(cmds, downloadCoverCmd(meta.Cover, leftW, m.searchList.Index(), coverMaxRows(m.height)))
					}
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
				if m.recentCursor < len(m.recentSearches) {
					query := m.recentSearches[m.recentCursor]
					m.textInput.SetValue(query)
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
			case key.Matches(msg, m.keymap.Back):
				// Esc from recent searches focuses the search bar
				m.textInput.Focus()
				return m, nil
			}
		} else if m.state == detailState && !m.textInput.Focused() && len(m.episodeState.Episodes) > 0 {
			// Resume card: Enter or r plays the resume episode
			if m.episodeState.ResumeFocus && m.episodeState.ResumeEpisode > 0 {
				if key.Matches(msg, m.keymap.Select) || key.Matches(msg, m.keymap.Resume) {
					epStr := strconv.Itoa(m.episodeState.ResumeEpisode)
					// Find the episode index in the list
					idx := -1
					for i, ep := range m.episodeState.Episodes {
						if ep == epStr {
							idx = i
							break
						}
					}
					if idx >= 0 {
						m.episodeState.Selected = idx
						m.episodeList.Select(idx)
						selectedAnime := m.searchState.Results[m.searchState.Selected]
						m.episodeState.Playing = true
						m.progressPercent = 0
						m.progressTarget = 0.5
						var cmds []tea.Cmd
						cmds = append(cmds, m.playEpisode(selectedAnime.AllAnimeID, epStr, selectedAnime.Name, selectedAnime.MALID))
						cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
							return progressTickMsg{}
						}))
						return m, tea.Batch(cmds...)
					}
				}
				// j/Tab/down moves focus to episode list
				if msg.String() == "tab" || msg.String() == "j" || msg.String() == "down" {
					m.episodeState.ResumeFocus = false
					m.episodeState.FocusRegion = RegionEpisodes
					return m, nil
				}
				// Shift+Tab from resume goes back to episodes; cycling further wraps to Recs
				// (handled by the up/k branch only since Shift+Tab has a special string form).
				if msg.String() == "shift+tab" {
					m.episodeState.ResumeFocus = false
					m.episodeState.FocusRegion = RegionEpisodes
					return m, nil
				}
				// Consume up/k while resume card is focused (top item, nowhere to go up)
				if msg.String() == "up" || msg.String() == "k" {
					return m, nil
				}
				// Consume other keys while resume card is focused (don't pass to episode list)
				return m, nil
			}
			// Recommendations focus: h/l/j/k navigate, Tab/Shift+Tab cycle regions.
			if m.episodeState.FocusRegion == RegionRecs && len(m.episodeState.RecItems) > 0 {
				cpr := recsPerRow(m.width)
				switch msg.String() {
				case "h", "left":
					if m.episodeState.RecSelected > 0 {
						m.episodeState.RecSelected--
					}
					return m, nil
				case "l", "right":
					if m.episodeState.RecSelected < len(m.episodeState.RecItems)-1 {
						m.episodeState.RecSelected++
					}
					return m, nil
				case "k", "up":
					if m.episodeState.RecSelected-cpr >= 0 {
						m.episodeState.RecSelected -= cpr
					} else {
						// Top row — hand off to episode list (keep list cursor where it is).
						m.episodeState.FocusRegion = RegionEpisodes
						m.episodeState.ResumeFocus = false
					}
					return m, nil
				case "j", "down":
					if m.episodeState.RecSelected+cpr < len(m.episodeState.RecItems) {
						m.episodeState.RecSelected += cpr
					}
					return m, nil
				case "tab":
					// Recs → Resume (if available) → Episodes → Recs
					if m.episodeState.ResumeEpisode > 0 {
						m.episodeState.FocusRegion = RegionResume
						m.episodeState.ResumeFocus = true
					} else {
						m.episodeState.FocusRegion = RegionEpisodes
						m.episodeState.ResumeFocus = false
					}
					return m, nil
				case "shift+tab":
					m.episodeState.FocusRegion = RegionEpisodes
					m.episodeState.ResumeFocus = false
					return m, nil
				case "enter":
					return m, openRecItem(m)
				}
				// Consume other keys while recs are focused
				return m, nil
			}
			if msg.String() >= "0" && msg.String() <= "9" {
				m.textInput.Focus()
				m.textInput.SetValue("")
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				m.filterEpisodesByNumber(m.textInput.Value())
				return m, tea.Batch(cmds...)
			}
			// Tab from the episode list cycles forward to Recs (skip Resume if absent)
			if msg.String() == "tab" {
				m.episodeState.FocusRegion = RegionRecs
				m.episodeState.ResumeFocus = false
				return m, nil
			}
			// Shift+Tab from episode list goes back to Resume if available
			if msg.String() == "shift+tab" {
				if m.episodeState.ResumeEpisode > 0 {
					m.episodeState.FocusRegion = RegionResume
					m.episodeState.ResumeFocus = true
				}
				return m, nil
			}
			// Navigate back up to resume card when at top of episode list
			if (msg.String() == "up" || msg.String() == "k") && m.episodeList.Index() == 0 && m.episodeState.ResumeEpisode > 0 {
				m.episodeState.ResumeFocus = true
				m.episodeState.FocusRegion = RegionResume
				return m, nil
			}
			prevIdx := m.episodeList.Index()
			var cmd tea.Cmd
			m.episodeList, cmd = m.episodeList.Update(msg)
			cmds = append(cmds, cmd)
			// Bottom-of-list handoff: when the user moves down past the last visible item
			// (or already at the end and presses down/j), hand focus to Recs.
			if (msg.String() == "down" || msg.String() == "j") &&
				m.episodeList.Index() == prevIdx &&
				prevIdx == len(m.episodeState.Episodes)-1 &&
				len(m.episodeState.RecItems) > 0 {
				m.episodeState.FocusRegion = RegionRecs
				m.episodeState.ResumeFocus = false
				m.episodeState.RecSelected = 0
				return m, tea.Batch(cmds...)
			}
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
		m.progress = progress.New(progress.WithColors(Theme.Primary, Theme.Secondary), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		// Resize list to two-column layout now that results are available
		m.resize(m.width, m.height)
		if len(msg.Results) > 0 {
			saveRecentSearch(m.lastQuery)
			m.recentSearches = loadRecentSearches()
			// Batch-fetch ALL metadata in one API call
			m.searchState.MetadataLoading = true
			m.loading = spinner.New()
			m.loading.Spinner = spinner.Points
			m.loading.Style = lipgloss.NewStyle().Foreground(Theme.Primary)
			cmds = append(cmds, m.loading.Tick)
			cmds = append(cmds, m.fetchMetadataBatch(msg.Results))
		}
		m.updateSearchList()

	case MetadataBatchLoadedMsg:
		// Cache all metadata from the batch result
		for idx, meta := range msg.MetadataMap {
			m.metadataCache[idx] = meta
		}
		m.searchState.MetadataLoading = false
		m.progressTarget = 1.0
		// Show metadata for the currently selected item
		if meta, ok := m.metadataCache[m.searchState.Selected]; ok {
			m.searchState.Metadata = meta
			// Trigger cover image download for the selected item
			if meta.Cover != "" && meta.CoverImage == "" {
				leftW, _, _, _ := detailLayout(m.width, m.height)
				cmds = append(cmds, downloadCoverCmd(meta.Cover, leftW, m.searchState.Selected, coverMaxRows(m.height)))
			}
		}

	case metadataDebounceMsg:
		// Navigation changed — look up cached metadata (no network call needed)
		if msg.Index == m.searchState.Selected {
			if meta, ok := m.metadataCache[msg.Index]; ok {
				m.searchState.Metadata = meta
				m.searchState.MetadataLoading = false
				// Trigger cover download if not already loaded
				if meta.Cover != "" && meta.CoverImage == "" {
					leftW, _, _, _ := detailLayout(m.width, m.height)
					cmds = append(cmds, downloadCoverCmd(meta.Cover, leftW, msg.Index, coverMaxRows(m.height)))
				}
			}
			// If not in cache, batch already failed or ID wasn't available
		}

	case MetadataLoadedMsg:
		// Fallback for individual fetch (Jikan-only items)
		if msg.Index == m.searchState.Selected {
			m.searchState.Metadata = msg.Metadata
			m.searchState.MetadataLoading = false
			m.progressTarget = 1.0
			// Trigger cover image download
			if msg.Metadata != nil && msg.Metadata.Cover != "" && msg.Metadata.CoverImage == "" {
				leftW, _, _, _ := detailLayout(m.width, m.height)
				cmds = append(cmds, downloadCoverCmd(msg.Metadata.Cover, leftW, msg.Index, coverMaxRows(m.height)))
			}
		}

	case EpisodesLoadedMsg:
		m.episodeState.Episodes = msg.Episodes
		m.episodeState.EpisodeTitles = msg.EpisodeTitles
		m.episodeState.Loading = false
		m.progressTarget = 0.7 // episodes loaded, metadata still loading
		m.progress = progress.New(progress.WithColors(Theme.Primary, Theme.Secondary), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		if msg.Error != nil {
			m.episodeState.Err = msg.Error
		}
		m.updateEpisodeList()
		if len(msg.Episodes) > 0 {
			m.episodeState.MetadataLoading = true
			cmds = append(cmds, m.fetchEpisodeMetadata())
		}
		// Fetch recommendations after episodes load (lazy)
		if len(m.searchState.Results) > 0 && m.searchState.Results[m.searchState.Selected].AniListID > 0 {
			cmds = append(cmds, fetchRecommendationsCmd(m.searchState.Results[m.searchState.Selected].AniListID, m.anilistClient))
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
		if msg.Rendered != "" {
			// Update the cached metadata with the rendered cover
			if meta, ok := m.metadataCache[msg.Index]; ok {
				meta.CoverImage = msg.Rendered
			}
			// Also update current metadata if it's the selected item
			if m.searchState.Metadata != nil && msg.Index == m.searchState.Selected {
				m.searchState.Metadata.CoverImage = msg.Rendered
			}
			// For Kitty protocol, transmit the image to terminal memory
			// via tea.Raw() (bypasses the ultraviolet renderer).
			// Delete old image first, then transmit the new one.
			if msg.KittyTransmitSeq != "" {
				oldID := m.kittyImageID
				m.kittyImageID = msg.KittyImageID
				return m, tea.Batch(
					DeleteKittyImageCmd(oldID),
					tea.Raw(msg.KittyTransmitSeq),
				)
			}
		}

	case TrackingStatusLoadedMsg:
		m.episodeState.TrackingStatus = msg.Status
		m.episodeState.TrackingProgress = msg.Progress
		// Override resume episode with AniList progress (next unwatched)
		if msg.Progress > 0 {
			m.episodeState.ResumeEpisode = msg.Progress + 1
		} else {
			m.episodeState.ResumeEpisode = 1
		}
		m.episodeState.ResumeFocus = m.episodeState.ResumeEpisode > 0
		// Cap at total episodes — if all watched, hide resume card
		if m.searchState.Metadata != nil && m.searchState.Metadata.Episodes > 0 {
			if m.episodeState.ResumeEpisode > m.searchState.Metadata.Episodes {
				m.episodeState.ResumeEpisode = 0
				m.episodeState.ResumeFocus = false
			}
		}
		if len(m.episodeState.Episodes) > 0 {
			m.updateEpisodeList()
		}

	case TrackingUpdateMsg:
		if msg.Err != nil {
			log.Printf("[anilix] tracking update failed: %v\n", msg.Err)
		} else {
			m.episodeState.TrackingStatus = msg.Status
			m.episodeState.TrackingProgress = msg.Progress
			// Override resume episode with AniList progress (next unwatched)
			if msg.Progress > 0 {
				m.episodeState.ResumeEpisode = msg.Progress + 1
			} else {
				m.episodeState.ResumeEpisode = 1
			}
			m.episodeState.ResumeFocus = m.episodeState.ResumeEpisode > 0
			// Cap at total episodes — if all watched, hide resume card
			if m.searchState.Metadata != nil && m.searchState.Metadata.Episodes > 0 {
				if m.episodeState.ResumeEpisode > m.searchState.Metadata.Episodes {
					m.episodeState.ResumeEpisode = 0
					m.episodeState.ResumeFocus = false
				}
			}
			if len(m.episodeState.Episodes) > 0 {
				m.updateEpisodeList()
			}
		}

	case RecommendationsLoadedMsg:
		m.episodeState.RecItems = msg.Items
		m.episodeState.RecLoading = false
		m.episodeState.RecErr = msg.Err
		// Clamp RecSelected to a valid index when recs arrive; don't yank focus
		// region itself — the user may be navigating elsewhere at this moment.
		if m.episodeState.RecSelected >= len(msg.Items) {
			m.episodeState.RecSelected = 0
		}

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
		m.progress = progress.New(progress.WithColors(Theme.Primary, Theme.Secondary), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		m.episodeState.Err = msg.Err

	case HomeHistoryLoadedMsg:
		if len(m.homeScreen.Sections) > 0 {
			m.homeScreen.Sections[0].Items = msg.Items
			m.homeScreen.Sections[0].Loading = false
		}

	case HomeTrendingLoadedMsg:
		if msg.Err != nil {
			if len(m.homeScreen.Sections) > 1 {
				m.homeScreen.Sections[1].Err = msg.Err
				m.homeScreen.Sections[1].Loading = false
			}
		} else if len(m.homeScreen.Sections) > 1 {
			m.homeScreen.Sections[1].Items = msg.Items
			m.homeScreen.Sections[1].Loading = false
		}

	case HomePopularLoadedMsg:
		if msg.Err != nil {
			if len(m.homeScreen.Sections) > 2 {
				m.homeScreen.Sections[2].Err = msg.Err
				m.homeScreen.Sections[2].Loading = false
			}
		} else if len(m.homeScreen.Sections) > 2 {
			m.homeScreen.Sections[2].Items = msg.Items
			m.homeScreen.Sections[2].Loading = false
		}

	case HomeGenreLoadedMsg:
		genreTitle := msg.Genre + " Anime"
		for _, sec := range m.homeScreen.Sections {
			if sec.Title == genreTitle {
				if msg.Err != nil {
					sec.Err = msg.Err
				} else {
					sec.Items = msg.Items
				}
				sec.Loading = false
				break
			}
		}

	case HomeItemResolvedMsg:
		cmds = append(cmds, m.enterDetail(msg.Anime)...)

	case PlayStreamMsg:
		m.episodeState.Playing = false
		m.progressPercent = 1.0
		m.progressTarget = 1.0
		m.progress = progress.New(progress.WithColors(Theme.Primary, Theme.Secondary), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		// Record to history
		if len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
			selectedAnime := m.searchState.Results[m.searchState.Selected]
			epNum := ""
			if m.episodeState.Selected < len(m.episodeState.Episodes) {
				epNum = m.episodeState.Episodes[m.episodeState.Selected]
			}
			m.hist.Add(history.Entry{
				AnimeName:    selectedAnime.Name,
				MALID:        selectedAnime.MALID,
				AniListID:    selectedAnime.AniListID,
				AllAnimeID:   selectedAnime.AllAnimeID,
				Episode:      epNum,
				CoverURL:     selectedAnime.Cover,
				Genres:       selectedAnime.Genres,
				Score:        selectedAnime.Score,
				Type:         selectedAnime.Type,
				Year:         selectedAnime.Year,
				EpisodeCount: selectedAnime.EpisodeCount,
			})
			_ = m.hist.Save()
		}
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
	}

	return m, tea.Batch(cmds...)
}

func (m *SearchModel) resize(width, height int) {
	m.width = width
	m.height = height

	padX, padY := 4, 2
	styledWidth := width - padX
	styledHeight := height - padY

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

	m.textInput.SetWidth(styledWidth)
	m.help.SetWidth(styledWidth)
	m.progress.SetWidth(width)

	// Size lists based on current state layout
	switch m.state {
	case searchState:
		if len(m.searchState.Results) > 0 {
			leftW, _, _, _ := searchLayout(width, height)
			m.searchList.SetSize(leftW, listHeight)
		} else {
			m.searchList.SetSize(styledWidth, listHeight)
		}
		m.episodeList.SetSize(styledWidth, listHeight)
	case detailState:
		_, rightW, _, _ := detailLayout(width, height)
		m.searchList.SetSize(styledWidth, listHeight)
		m.episodeList.SetSize(rightW, listHeight)
	default:
		m.searchList.SetSize(styledWidth, listHeight)
		m.episodeList.SetSize(styledWidth, listHeight)
	}
}

func (m *SearchModel) updateSearchList() {
	items := make([]list.Item, 0, len(m.searchState.Results))
	for _, anime := range m.searchState.Results {
		items = append(items, animeItem{anime: anime})
	}
	cmd := m.searchList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.searchList.Select(m.searchState.Selected)
}

func (m *SearchModel) updateEpisodeList() {
	trackingProgress := m.episodeState.TrackingProgress
	items := make([]list.Item, 0, len(m.episodeState.Episodes))
	for i, ep := range m.episodeState.Episodes {
		title := ""
		if len(m.episodeState.EpisodeTitles) > i {
			title = m.episodeState.EpisodeTitles[i]
		}
		epNum := parseEpisodeNum(ep)
		prog := episodeUnwatched
		if trackingProgress > 0 && epNum > 0 {
			if epNum <= trackingProgress {
				prog = episodeWatched
			} else if epNum == trackingProgress+1 {
				prog = episodeCurrent
			}
		}
		items = append(items, episodeItem{number: ep, title: title, progress: prog})
	}
	cmd := m.episodeList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.episodeList.Select(m.episodeState.Selected)
}

func (m *SearchModel) filterEpisodesByNumber(query string) {
	if query == "" {
		m.updateEpisodeList()
		return
	}
	trackingProgress := m.episodeState.TrackingProgress
	items := make([]list.Item, 0)
	for i, ep := range m.episodeState.Episodes {
		if strings.Contains(ep, query) {
			title := ""
			if len(m.episodeState.EpisodeTitles) > i {
				title = m.episodeState.EpisodeTitles[i]
			}
			epNum := parseEpisodeNum(ep)
			prog := episodeUnwatched
			if trackingProgress > 0 && epNum > 0 {
				if epNum <= trackingProgress {
					prog = episodeWatched
				} else if epNum == trackingProgress+1 {
					prog = episodeCurrent
				}
			}
			items = append(items, episodeItem{number: ep, title: title, progress: prog})
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

// computeResumeEpisode sets ResumeEpisode from local history.
// AniList progress overrides this when TrackingStatusLoadedMsg arrives.
func (m *SearchModel) computeResumeEpisode() {
	m.episodeState.ResumeEpisode = 0
	if m.hist == nil {
		return
	}
	anime := m.searchState.Results[m.searchState.Selected]
	if anime == nil {
		return
	}
	entries := m.hist.RecentUnique(50)
	for _, e := range entries {
		if e.MALID == anime.MALID {
			epNum, _ := strconv.Atoi(e.Episode)
			if epNum > 0 {
				m.episodeState.ResumeEpisode = epNum + 1
			} else {
				m.episodeState.ResumeEpisode = 1
			}
			return
		}
	}
	// No history: start from episode 1
	m.episodeState.ResumeEpisode = 1
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

// enterDetail transitions the UI from a home/rec-card selection into the
// detail state for the given anime. It mirrors the work originally done by
// the HomeItemResolvedMsg case so the same commands fire whether the entry
// point was the home grid or a recommendation card on the detail page.
func (m *SearchModel) enterDetail(anime *source.Anime) []tea.Cmd {
	m.searchState.Results = []*source.Anime{anime}
	m.searchState.Selected = 0
	m.prevState = homeState
	m.state = detailState
	m.textInput.Placeholder = "Search episode..."
	m.episodeState.Loading = true
	m.episodeState.AnimeID = anime.AllAnimeID
	m.episodeState.FocusRegion = RegionEpisodes // reset focus to episode list
	m.progressPercent = 0
	m.progressTarget = 0.4
	// Compute resume episode from history (AniList progress overrides later)
	m.computeResumeEpisode()
	m.episodeState.ResumeFocus = m.episodeState.ResumeEpisode > 0

	var cmds []tea.Cmd
	cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
	if m.searchState.Metadata == nil && anime.AniListID > 0 {
		cmds = append(cmds, m.fetchMetadata())
	}
	if anime.AniListID > 0 {
		cmds = append(cmds, m.fetchTrackingStatusCmd(anime.AniListID))
	}
	cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return progressTickMsg{}
	}))
	return cmds
}

// openRecItem activates the currently-focused recommendation card, mirroring the
// home-grid selection flow. If the rec already has an AllAnimeID, it dispatches
// straight into the detail state. Otherwise it kicks off a name-search to
// resolve the AllAnimeID asynchronously and falls back into HomeItemResolvedMsg.
func openRecItem(m *SearchModel) tea.Cmd {
	if m.episodeState.RecSelected < 0 || m.episodeState.RecSelected >= len(m.episodeState.RecItems) {
		return nil
	}
	rec := m.episodeState.RecItems[m.episodeState.RecSelected]

	// Synthesize a *source.Anime from the rec card. MALID is unknown until we
	// resolve through AllAnime; if the rec payload already carries one, forward it.
	anime := &source.Anime{
		Name:       rec.Name,
		MALID:      rec.MALID,
		AniListID:  rec.AniListID,
		AllAnimeID: rec.AllAnimeID,
		Cover:      rec.Cover,
		Genres:     rec.Genres,
		Type:       rec.Type,
		Year:       rec.Year,
	}

	// Mirror selectHomeItem's logic: AllAnimeID present → enterDetail directly,
	// otherwise resolve via AllAnime search after the user explicitly picked the card.
	if anime.AllAnimeID != "" {
		cmds := m.enterDetail(anime)
		return tea.Batch(cmds...)
	}
	return resolveHomeItemAllAnimeID(anime, m.allanimeClient, m.searchState.TranslationType)
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
