package tui

import (
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/source"
)

// TUI state machine states
type tuiState int

const (
	searchState tuiState = iota
	// animeState is the details of the selected anime: the screen the
	// compact layout opens, and the step between the results and the
	// episode list.
	animeState
	detailState
	historyState
	// watchlistState is the saved list of the account: the entries AniList
	// keeps for the user.
	watchlistState
	confirmQuitState
	settingsState
	anilistLoginState
	themeState
	// anilistState is the AniList account screen: the authentication
	// status, the tracking and the login and logout actions.
	anilistState
	// confirmLogoutState asks before the AniList credentials are dropped.
	confirmLogoutState
)

// SelectionResult holds the selected anime and episode
type SelectionResult struct {
	Anime   *source.Anime
	Episode string
}

// SearchState holds the state for the anime search TUI
type SearchState struct {
	Query           string
	Results         []*source.Anime
	Selected        int
	Metadata        *MetadataPanel
	Loading         bool
	MetadataLoading bool
	Err             error
	TranslationType string
}

// MetadataPanel holds merged metadata to display on the right panel
type MetadataPanel struct {
	Title        string
	TitleEnglish string
	TitleNative  string
	Cover        string
	CoverImage   string // Rendered ANSI cover art (half-block), empty if not available
	CoverWidth   int    // The width the cover was rendered at
	CoverRows    int    // The number of rows the cover was rendered at
	Year         int
	Type         string
	Status       string
	Episodes     int
	Score        float64
	Rank         int
	Popularity   int
	Genres       []string
	Synopsis     string
	Source       string // "Jikan", "AniList", "Jikan + AniList", "Kitsu", or "Hianime" (provisional placeholder)
}

// EpisodeState holds the state for episode selection
type EpisodeState struct {
	AnimeID          string
	Episodes         []string
	EpisodeTitles    []string
	Selected         int
	Loading          bool
	Err              error
	EpisodeMetadata  *EpisodeMetadataPanel
	MetadataLoading  bool
	Playing          bool
	TrackingStatus   string // "CURRENT", "COMPLETED", etc.
	TrackingProgress int
}

// EpisodeMetadataPanel holds metadata for a single episode
type EpisodeMetadataPanel struct {
	Title         string
	TitleJapanese string
	Aired         string
	Score         float64
	Filler        bool
	Recap         bool
	Synopsis      string
	Duration      int
}

// SettingsState holds the state for the settings popup
type SettingsState struct {
	Quality        string // "1080p", "720p", "480p", "360p", "auto"
	AniskipEnabled bool
	Cursor         int    // index into the grouped rows of the sheet
	Scroll         int    // first visible row, so the sheet fits small terminals
	UpdateStatus   string // "", "checking", "updated", "error"
	UpdateVersion  string // latest version tag, e.g. "v1.7.0"
	UpdateError    error
}

// WatchlistState holds the state for the watchlist screen: the saved list
// of the account, with the three phases a fetched screen has - loading,
// error and content.
type WatchlistState struct {
	Loading bool
	Err     error
	Entries []*anilist.WatchlistEntry
	Cursor  int
	Scroll  int // the first row of the visible window of a long list
}

// NewSearchState creates a new search state
func NewSearchState() *SearchState {
	return &SearchState{
		Query:    "",
		Results:  nil,
		Selected: 0,
		Metadata: nil,
		Loading:  false,
		Err:      nil,
		TranslationType: func() string {
			t := config.GetString("translation_type")
			if t == "" {
				return "sub"
			}
			return t
		}(),
	}
}

// NewEpisodeState creates a new episode state
func NewEpisodeState() *EpisodeState {
	return &EpisodeState{
		Episodes: nil,
		Selected: 0,
		Loading:  false,
	}
}
