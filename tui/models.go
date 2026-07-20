package tui

import (
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/source"
)

// TUI state machine states
type tuiState int

const (
	searchState tuiState = iota
	detailState
	confirmQuitState
	settingsState
	anilistLoginState
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
	Year         int
	Type         string
	Status       string
	Episodes     int
	Score        float64
	Rank         int
	Popularity   int
	Genres       []string
	Synopsis     string
	Source       string // "Jikan", "AniList", or "Jikan + AniList"
}

// EpisodeState holds the state for episode selection
type EpisodeState struct {
	AniListID        int
	BrowserURL       string
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
	Cursor         int    // 0 = quality, 1 = aniskip, 2 = anilist, 3 = update
	UpdateStatus   string // "", "checking", "updated", "error"
	UpdateVersion  string // latest version tag, e.g. "v1.7.0"
	UpdateError    error
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
