package source

import "strconv"

// Episode represents a single episode within an anime season.
// It links back to its parent Anime for context during stream resolution.
type Episode struct {
	Number float64
	Title  string
	URL    string
	Season int // 0 = no season (single season anime)
	Anime  *Anime
}

// String returns a display-friendly episode identifier: the title if available,
// otherwise the episode number (integer or decimal for half-episodes).
func (e *Episode) String() string {
	if e.Title != "" {
		return e.Title
	}
	if e.Number == float64(int(e.Number)) {
		return strconv.Itoa(int(e.Number))
	}
	return strconv.FormatFloat(e.Number, 'f', 1, 64)
}
