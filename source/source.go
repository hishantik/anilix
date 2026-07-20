package source

// Season represents a single season of an anime, used to organize episodes
// when an anime has multiple cours or parts (e.g., "Season 2 - Cour 1").
type Season struct {
	Number int
	Name   string // e.g., "Season 1", "Season 2 - Cour 1"
}

// Source is the core abstraction for anime data providers such as Jikan, AniList, and Miruro.
// It defines the contract for searching anime, listing seasons/episodes, and resolving streams.
// This interface decouples the TUI/CLI from specific provider implementations.
type Source interface {
	// Name returns the human-readable provider name (e.g., "Jikan", "Miruro").
	Name() string
	// Search queries the provider for anime matching the given string.
	Search(query string) ([]*Anime, error)
	// SeasonsOf returns available seasons for an anime, enabling multi-season navigation.
	SeasonsOf(anime *Anime) ([]Season, error)
	// EpisodesOf returns episodes for a specific season of an anime.
	EpisodesOf(anime *Anime, season int) ([]*Episode, error)
	// StreamsOf resolves playable stream URLs for a given episode.
	StreamsOf(episode *Episode) ([]*Stream, error)
	// ID returns the unique provider identifier used in config and registration.
	ID() string
}
