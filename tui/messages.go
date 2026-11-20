package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/provider/kitsu"
	"github.com/hishantik/anilix/source"
)

type SearchResultsMsg struct {
	Results []*source.Anime
}

// IDsResolvedMsg carries the AniList/MAL ids resolved in the background,
// keyed by result index, for the UI thread to apply.
type IDsResolvedMsg struct {
	IDs map[int]ExternalIDs
}

// MALIDsResolvedMsg carries the external IDs Kitsu resolved for search
// results, keyed by result index, along with the Kitsu entries themselves
// so the UI can build metadata when AniList knows nothing about an entry.
type MALIDsResolvedMsg struct {
	IDs    map[int]ExternalIDs
	Animes map[int]*kitsu.Anime
}

// ExternalIDs are the ids of one search result as Kitsu reports them.
type ExternalIDs struct {
	AniListID int
	MALID     int
}

type SearchErrorMsg struct {
	Err error
}

type MetadataFetchTriggered struct {
	Index int
}

type metadataDebounceMsg struct {
	Index int
}

type episodeMetadataDebounceMsg struct {
	Index int
}

// MetadataLoadedMsg is sent when a single API source returns.
type MetadataLoadedMsg struct {
	Metadata *MetadataPanel
	Index    int
}

// EpisodeTitlesLoadedMsg carries the Jikan episode titles fetched as the
// follow-up of the episode list: the list renders first, the titles arrive
// when they arrive.
type EpisodeTitlesLoadedMsg struct {
	Index  int
	Titles []string
}

// MetadataProgressMsg carries the new progress target of a metadata tier:
// the bar eases toward it while the next tier runs.
type MetadataProgressMsg struct {
	Target float64
}

// MetadataBatchLoadedMsg carries all metadata for a search result set.
type MetadataBatchLoadedMsg struct {
	MetadataMap map[int]*MetadataPanel
	// Target is the progress the bar moves to when this batch lands. The
	// follow-up show pages keep moving it to 1.0 afterwards.
	Target float64
	// FollowUp is the provisional pass that runs after this batch: the
	// hianime show pages of the entries no metadata API knew. It is a
	// second message so the real panels are not held back by the ~200KB
	// pages.
	FollowUp tea.Cmd
}

type EpisodesLoadedMsg struct {
	Episodes      []string
	EpisodeTitles []string
	Error         error
	// MALID is the id the provider recovered from an embed URL while
	// listing episodes, 0 when it found none; applied on the UI thread.
	MALID int
	// FollowUp fetches the Jikan episode titles while the episode list is
	// rendering and the bar is at 0.7; it keeps the bar responsive.
	FollowUp tea.Cmd
}

type EpisodeMetadataFetchTriggered struct {
	Index int
}

type EpisodeMetadataLoadedMsg struct {
	Metadata   *EpisodeMetadataPanel
	Index      int
	CacheKey   string         `json:"-"`
	RawEpisode *jikan.Episode `json:"-"`
}

type PlayStreamMsg struct{}

type TUIErrorMsg struct {
	Err error
}

type progressTickMsg struct{}

// CoverImageLoadedMsg is sent when a cover image has been downloaded and rendered.
type CoverImageLoadedMsg struct {
	Rendered         string // Rendered image (SGR half-block or Unicode placeholder text)
	KittyTransmitSeq string // APC sequence for Kitty graphics (empty for non-Kitty)
	KittyImageID     uint32 // Image ID for Kitty graphics cleanup
	Index            int    // Result index this cover belongs to
	URL              string // The URL the cover was downloaded from, so a late answer for an old entry is recognised
	Width            int    // The width the cover was rendered at
	Rows             int    // The number of rows the cover was rendered at
}

// TrackingUpdateMsg is sent after a tracking progress update completes.
type TrackingUpdateMsg struct {
	Status   string
	Progress int
	Err      error
}

// WatchlistLoadedMsg is sent when the saved list of the account came back
// from AniList, or when fetching it failed.
type WatchlistLoadedMsg struct {
	Entries []*anilist.WatchlistEntry
	Err     error
}

// TrackingStatusLoadedMsg is sent when the current tracking status for an anime is fetched.
type TrackingStatusLoadedMsg struct {
	Status   string
	Progress int
}

// AniListLoginMsg is sent when the AniList OAuth login flow completes.
type AniListLoginMsg struct {
	Err error
}

// UpdateCheckMsg is sent after checking GitHub for the latest release.
type UpdateCheckMsg struct {
	LatestVersion string
	Err           error
}

// UpdateCompleteMsg is sent after downloading and installing the update.
type UpdateCompleteMsg struct {
	Err error
}

// DiscoverLoadedMsg is sent when the discover screen finished fetching
// the trending and popular catalogues, or when fetching them failed.
type DiscoverLoadedMsg struct {
	Trending []*source.Anime
	Popular  []*source.Anime
	Err      error
}
