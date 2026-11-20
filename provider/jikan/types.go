package jikan

// AnimeResponse represents the Jikan API search response (list)
type AnimeResponse struct {
	Data []AnimeData `json:"data"`
}

// AnimeSingleResponse represents the Jikan API single anime response
type AnimeSingleResponse struct {
	Data AnimeData `json:"data"`
}

// AnimeData represents a single anime in Jikan response
type AnimeData struct {
	MalID         int         `json:"mal_id"`
	Title         string      `json:"title"`
	TitleEnglish  string      `json:"title_english"`
	TitleJapanese string      `json:"title_japanese"`
	URL           string      `json:"url"`
	Images        Images      `json:"images"`
	Year          interface{} `json:"year"` // can be null or int
	Genres        []Genre     `json:"genres"`
	Status        string      `json:"status"`
	Synopsis      string      `json:"synopsis"`
	Type          string      `json:"type"`       // "TV", "Movie", "OVA", etc.
	Episodes      int         `json:"episodes"`   // number of episodes
	Rating        string      `json:"rating"`     // "PG-13 - Teens 13 or older"
	Score         float64     `json:"score"`      // e.g., 8.02
	Rank          int         `json:"rank"`       // e.g., 726
	Popularity    int         `json:"popularity"` // e.g., 9
}

// Images contains image URLs
type Images struct {
	JPG JPGImages `json:"jpg"`
}

// JPGImages contains specific image URLs
type JPGImages struct {
	ImageURL      string `json:"image_url"`
	SmallImageURL string `json:"small_image_url"`
	LargeImageURL string `json:"large_image_url"`
}

// Genre represents an anime genre
type Genre struct {
	MalID int    `json:"mal_id"`
	Name  string `json:"name"`
}

// EpisodesResponse represents the Jikan API episodes response
type EpisodesResponse struct {
	Data []Episode `json:"data"`
}

// Episode represents a single episode
type Episode struct {
	MalID         int     `json:"mal_id"`
	EpisodeID     int     `json:"episode_id"`
	Title         string  `json:"title"`
	TitleJapanese string  `json:"title_japanese"`
	TitleRomanji  string  `json:"title_romanji"`
	Episode       string  `json:"episode"`
	URL           string  `json:"url"`
	Aired         string  `json:"aired"`
	Score         float64 `json:"score"`
	Filler        bool    `json:"filler"`
	Recap         bool    `json:"recap"`
	Synopsis      string  `json:"synopsis"`
	Duration      int     `json:"duration"`
}

// EpisodeSingleResponse represents the Jikan API single episode response
type EpisodeSingleResponse struct {
	Data Episode `json:"data"`
}
