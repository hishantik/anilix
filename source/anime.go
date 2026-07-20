package source

// Anime represents a single anime title with its metadata from providers like Jikan/AniList.
// It serves as the central data model passed between search, episode listing, and stream resolution.
type Anime struct {
	Name         string
	URL          string
	Cover        string
	Year         int
	Genres       []string
	Status       string
	MALID        int    // MyAnimeList ID for metadata linkage
	AniListID    int    // AniList ID for metadata linkage
	EpisodeCount int    // Total episode count
	Type         string // "TV", "Movie", "OVA", etc.
	Rating       string // "PG-13 - Teens 13 or older"
	Score        float64
	Rank         int
	Popularity   int
	Source       Source
}

// String returns the anime name for display in lists and TUI selection.
func (a *Anime) String() string {
	return a.Name
}
