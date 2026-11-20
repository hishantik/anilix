package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/tui/theme"
)

// The responsive checks render every screen at every supported terminal
// size and fail on the defects that only show up on a phone or a very wide
// terminal: a row wider than the terminal, a screen taller than the content
// region, a colour escape on a terminal without colour, or a layout that
// breaks after a resize.
//
// The profiles are the manual matrix of docs/design-system.md, turned into
// a test.

// TestEveryScreenRendersAtEveryProfile renders every screen and every
// meaningful state at the five profiles.
func TestEveryScreenRendersAtEveryProfile(t *testing.T) {
	isolateScreenConfig(t)
	theme.SetProfile(colorprofile.TrueColor)
	t.Cleanup(func() { theme.SetProfile(colorprofile.TrueColor) })

	profiles := []struct {
		name   string
		width  int
		height int
	}{
		{"very-small", 40, 15},
		{"compact", 60, 20},
		{"standard", 80, 24},
		{"large", 120, 35},
		{"wide", 160, 45},
	}

	for _, p := range profiles {
		for _, st := range screenStates() {
			resetScreenConfig()
			name := p.name + "/" + st.name
			m := NewSearchModel()
			m.resize(p.width, p.height)
			m.textInput.Blur()
			st.stage(m)

			content := renderScreenContent(m)
			view := shellStrip(m.renderShell(content))
			checkRenderedBlock(t, name+" content", content, m.contentWidth, m.contentHeight)
			checkRenderedBlock(t, name+" shell", view, p.width, p.height)
		}
	}
}

// TestScreensSurviveResizes renders the populated screens after the
// terminal changes size in both directions: a list that kept the size of
// the old terminal overflows the new one.
func TestScreensSurviveResizes(t *testing.T) {
	isolateScreenConfig(t)
	theme.SetProfile(colorprofile.TrueColor)
	t.Cleanup(func() { theme.SetProfile(colorprofile.TrueColor) })

	sizes := [][2]int{{160, 45}, {40, 15}, {80, 24}, {120, 35}, {60, 20}, {40, 15}}

	for _, st := range screenStates() {
		if st.name == "drawer-open" {
			continue
		}
		resetScreenConfig()
		m := NewSearchModel()
		m.resize(sizes[0][0], sizes[0][1])
		m.textInput.Blur()
		st.stage(m)

		for _, size := range sizes {
			m.resize(size[0], size[1])
			content := renderScreenContent(m)
			view := shellStrip(m.renderShell(content))
			name := fmt.Sprintf("resize-%dx%d/%s", size[0], size[1], st.name)
			checkRenderedBlock(t, name+" content", content, m.contentWidth, m.contentHeight)
			checkRenderedBlock(t, name+" shell", view, size[0], size[1])
		}
	}
}

// TestScreensRenderWithoutColourOrUnicode checks the degraded modes: a
// terminal without colour must not receive colour escapes, and a terminal
// without unicode must still get rows that fit the screen.
func TestScreensRenderWithoutColourOrUnicode(t *testing.T) {
	isolateScreenConfig(t)

	for _, mode := range []struct {
		name    string
		profile colorprofile.Profile
		unicode bool
	}{
		{"no-colour", colorprofile.ASCII, true},
		{"no-unicode", colorprofile.TrueColor, false},
		{"plain", colorprofile.ASCII, false},
	} {
		t.Run(mode.name, func(t *testing.T) {
			theme.SetProfile(mode.profile)
			theme.SetUnicode(mode.unicode)
			t.Cleanup(func() {
				theme.SetProfile(colorprofile.TrueColor)
				theme.SetUnicode(true)
			})

			for _, st := range screenStates() {
				resetScreenConfig()
				m := NewSearchModel()
				m.resize(80, 24)
				m.textInput.Blur()
				st.stage(m)

				view := shellStrip(m.renderShell(renderScreenContent(m)))
				checkRenderedBlock(t, mode.name+"/"+st.name, view, 80, 24)

				if mode.profile == colorprofile.ASCII {
					if strings.Contains(view, "\x1b[38;") || strings.Contains(view, "\x1b[48;") {
						t.Errorf("%s/%s emitted a colour escape on a terminal without colour", mode.name, st.name)
					}
				}
			}
		})
	}
}

// resetScreenConfig restores the config keys the staged states read and
// write, so a state that signs in does not leak its token into the next
// state or the next test.
func resetScreenConfig() {
	config.Set("anilist.token", "")
	config.Set("anilist.username", "")
	config.Set("anilist.tracking.enabled", false)
	config.Set("history.enabled", true)
	config.Set("artwork.renderer", "auto")
	config.Set("translation_type", "sub")
}

// isolateScreenConfig points HOME at a temporary directory and resets the
// config on the way out.
func isolateScreenConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	resetScreenConfig()
	t.Cleanup(resetScreenConfig)
}

type screenState struct {
	name  string
	stage func(m *SearchModel)
}

// screenStates is every screen and every meaningful state of a screen,
// populated with data that stresses the layout: long titles, long
// descriptions, long errors and long lists.
func screenStates() []screenState {
	longTitle := "Sousou no Frieren: Beyond Journey's End And A Very Long Title That Keeps Going"
	longEpisode := "The Endless Journey Of The Longest Episode Title Ever Written Down And Then Some"

	meta := func() *MetadataPanel {
		return &MetadataPanel{
			Title:        longTitle,
			TitleEnglish: "Frieren: Beyond Journey's End",
			Synopsis:     strings.Repeat("A long synopsis of an adventure that keeps going. ", 30),
			Genres:       []string{"Action", "Adventure", "Drama", "Fantasy"},
			Year:         2023,
			Type:         "TV",
			Episodes:     28,
			Status:       "Finished",
			Score:        9.12,
			Rank:         3,
			Popularity:   42,
			Source:       "Manga",
		}
	}

	return []screenState{
		{"search-empty", func(m *SearchModel) {
			m.searchState.Results = nil
			m.searchState.Err = nil
			m.updateSearchList()
		}},
		{"search-loading", func(m *SearchModel) {
			m.searchState.Loading = true
			m.updateSearchList()
		}},
		{"search-error", func(m *SearchModel) {
			m.searchState.Err = errors.New("the provider could not be reached while searching for the title")
			m.updateSearchList()
		}},
		{"search-results", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{
				{Name: longTitle, ShowID: "21", Score: 9.0},
				{Name: "One Piece", ShowID: "22", Score: 8.7},
				{Name: "Dandadan", ShowID: "23", Score: 8.4},
			}
			m.metadataCache = map[int]*MetadataPanel{0: meta()}
			m.syncDelegates()
			m.updateSearchList()
			m.updateEpisodeList()
		}},
		{"anime-details", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: longTitle, ShowID: "21"}}
			m.searchState.Metadata = meta()
			m.pushState(animeState)
			m.updateEpisodeList()
		}},
		{"anime-details-empty", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: longTitle, ShowID: "21"}}
			m.searchState.Metadata = nil
			m.pushState(animeState)
		}},
		{"detail-episodes", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: "Frieren", ShowID: "21"}}
			m.pushState(detailState)
			m.episodeState.Episodes = []string{"1", "2", "3", "4", "5", "6"}
			m.episodeState.EpisodeTitles = []string{longEpisode, "Ende", "Fass", "", "", ""}
			m.updateEpisodeList()
		}},
		{"detail-episodes-meta", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: longTitle, ShowID: "21"}}
			m.searchState.Metadata = meta()
			m.pushState(detailState)
			m.episodeState.Episodes = []string{"1", "2", "3", "4", "5", "6"}
			m.episodeState.EpisodeTitles = []string{longEpisode, "Ende", "Fass", "", "", ""}
			m.updateEpisodeList()
		}},
		{"detail-loading", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: "Frieren", ShowID: "21"}}
			m.pushState(detailState)
			m.episodeState.Loading = true
		}},
		{"detail-error", func(m *SearchModel) {
			m.searchState.Results = []*source.Anime{{Name: "Frieren", ShowID: "21"}}
			m.pushState(detailState)
			m.episodeState.Err = errors.New("the episode list of the series could not be fetched from the provider")
		}},
		{"history", func(m *SearchModel) {
			now := time.Now()
			m.state = historyState
			m.watchHistoryCache = []historyEntry{
				{Name: longTitle, ShowID: "21", Episode: "3", At: now.Add(-2 * time.Hour)},
				{Name: "One Piece", ShowID: "22", Episode: "1100", At: now.Add(-30 * time.Hour)},
				{Name: "Dandadan", ShowID: "23", Episode: "1", At: now.Add(-48 * time.Hour)},
			}
		}},
		{"history-empty", func(m *SearchModel) {
			m.state = historyState
			m.watchHistoryCache = nil
		}},
		{"watchlist", func(m *SearchModel) {
			config.Set("anilist.token", "tok")
			m.state = watchlistState
			m.watchlist = &WatchlistState{Entries: []*anilist.WatchlistEntry{
				{AniListID: 21, Title: longTitle, Status: "CURRENT", Progress: 5, Total: 28, Year: 2023, Score: 9.0, Format: "TV"},
				{AniListID: 1, Title: "One Piece", Status: "PLANNING", Year: 1999, Score: 8.7, Format: "TV"},
			}}
		}},
		{"watchlist-loading", func(m *SearchModel) {
			m.state = watchlistState
			m.watchlist = &WatchlistState{Loading: true}
		}},
		{"watchlist-error", func(m *SearchModel) {
			m.state = watchlistState
			m.watchlist = &WatchlistState{Err: errors.New("network unreachable while fetching the list of the account")}
		}},
		{"watchlist-disconnected", func(m *SearchModel) {
			m.state = watchlistState
			m.watchlist = &WatchlistState{}
		}},
		{"anilist-out", func(m *SearchModel) {
			m.state = anilistState
			m.anilistCursor = aniListStartCursor()
		}},
		{"anilist-in", func(m *SearchModel) {
			config.Set("anilist.token", "tok")
			config.Set("anilist.username", "pirate-king")
			m.trackingEnabled = true
			m.state = anilistState
			m.anilistCursor = 1
		}},
		{"anilist-login", func(m *SearchModel) {
			m.prevState = anilistState
			m.state = anilistLoginState
		}},
		{"settings", func(m *SearchModel) {
			m.state = settingsState
		}},
		{"settings-bottom", func(m *SearchModel) {
			m.state = settingsState
			m.settingsState.Cursor = settingsRowUpdate
		}},
		{"theme", func(m *SearchModel) { m.state = themeState }},
		{"dialog-quit", func(m *SearchModel) { m.state = confirmQuitState }},
		{"dialog-logout", func(m *SearchModel) { m.state = confirmLogoutState }},
		{"drawer-open", func(m *SearchModel) {
			m.nav.overlay = true
			m.nav.focused = true
			m.nav.cursor = navIndexOf(navWatchlist)
		}},
	}
}

// renderScreenContent renders the body of the current screen, without the
// application shell.
func renderScreenContent(m *SearchModel) string {
	switch m.state {
	case searchState:
		return m.viewSearchState()
	case animeState:
		return m.viewAnimeState()
	case detailState:
		return m.viewDetailState()
	case historyState:
		return m.viewHistoryState()
	case watchlistState:
		return m.viewWatchlistState()
	case settingsState:
		return m.viewSettings()
	case anilistState:
		return m.viewAniListState()
	case anilistLoginState:
		return m.viewAniListLogin()
	case confirmQuitState:
		return m.viewConfirmQuit()
	case confirmLogoutState:
		return m.viewConfirmLogout()
	case themeState:
		return m.viewThemeShowcase()
	}
	return ""
}

// checkRenderedBlock checks the dimensions of a rendered block.
func checkRenderedBlock(t *testing.T, name, block string, width, height int) {
	t.Helper()
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("%s row %d is %d cells wide, want at most %d: %q", name, i, w, width, line)
		}
	}
	if len(lines) > height {
		t.Errorf("%s rendered %d rows, want at most %d", name, len(lines), height)
	}
}
