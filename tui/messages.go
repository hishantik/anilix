package tui

import (
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
)

type SearchResultsMsg struct {
	Results []*source.Anime
}

// IDsResolvedMsg carries search results whose AniList/MAL IDs have been
// resolved in the background.
type IDsResolvedMsg struct {
	Results []*source.Anime
}

// MALIDsResolvedMsg carries the external IDs Kitsu resolved for search
// results, keyed by result index. Kitsu returns both the AniList and the
// MAL id of an entry in one request.
type MALIDsResolvedMsg struct {
	IDs map[int]ExternalIDs
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

// MetadataBatchLoadedMsg carries all metadata for a search result set.
type MetadataBatchLoadedMsg struct {
	MetadataMap map[int]*MetadataPanel
}

type EpisodesLoadedMsg struct {
	Episodes      []string
	EpisodeTitles []string
	Error         error
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
