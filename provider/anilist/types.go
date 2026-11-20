package anilist

import "encoding/json"

// GraphQL request/response types

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

// Single media query response

type MediaResponse struct {
	Media MediaData `json:"Media"`
}

// Batch media query response (aliased queries)

type BatchMediaResponse struct {
	Data map[string]MediaData `json:"-"`
}

// Media data from AniList

type MediaData struct {
	ID           int        `json:"id"`
	IDMal        int        `json:"idMal"`
	Title        Title      `json:"title"`
	CoverImage   CoverImage `json:"coverImage"`
	Type         string     `json:"type"`
	Format       string     `json:"format"`
	Status       string     `json:"status"`
	Description  string     `json:"description"`
	StartDate    FuzzyDate  `json:"startDate"`
	EndDate      FuzzyDate  `json:"endDate"`
	Season       string     `json:"season"`
	SeasonYear   int        `json:"seasonYear"`
	Episodes     int        `json:"episodes"`
	Duration     int        `json:"duration"`
	Genres       []string   `json:"genres"`
	Synonyms     []string   `json:"synonyms"`
	AverageScore int        `json:"averageScore"`
	MeanScore    int        `json:"meanScore"`
	Popularity   int        `json:"popularity"`
	Trending     int        `json:"trending"`
	Favourites   int        `json:"favourites"`
	SiteURL      string     `json:"siteUrl"`
}

// TitleMatch is the result of resolving one search title to an AniList entry.
// Media is nil when no entry matched the title exactly.
type TitleMatch struct {
	Query string
	Media *MediaData
}

type Title struct {
	Romaji        string `json:"romaji"`
	English       string `json:"english"`
	Native        string `json:"native"`
	UserPreferred string `json:"userPreferred"`
}

type CoverImage struct {
	ExtraLarge string `json:"extraLarge"`
	Large      string `json:"large"`
	Medium     string `json:"medium"`
	Color      string `json:"color"`
}

type FuzzyDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type any = interface{}

// Viewer query response

type ViewerData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ViewerResponse struct {
	Viewer ViewerData `json:"Viewer"`
}

// MediaList tracking types

type MediaListEntry struct {
	ID       int    `json:"id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}

// WatchlistEntry is one saved anime of the user's list, flattened to the
// fields the watchlist screen shows.
type WatchlistEntry struct {
	AniListID int
	Title     string
	Cover     string
	Status    string  // the list status: CURRENT, PLANNING, COMPLETED, ...
	Progress  int     // episodes the user marked as watched
	Total     int     // total episodes, zero when unknown
	Year      int     // season year, zero when unknown
	Score     float64 // the average score out of ten, zero when unknown
	Format    string  // TV, MOVIE, OVA, ...
}

// MediaListCollectionEntry is one entry of one list of the collection.
type MediaListCollectionEntry struct {
	Status   string    `json:"status"`
	Progress int       `json:"progress"`
	Media    MediaData `json:"media"`
}

// MediaListGroup is one named list of the collection, e.g. "Watching".
type MediaListGroup struct {
	Name    string                     `json:"name"`
	Entries []MediaListCollectionEntry `json:"entries"`
}

// MediaListCollection is the anime list collection of a user.
type MediaListCollection struct {
	Lists []MediaListGroup `json:"lists"`
}

type MediaListCollectionResponse struct {
	Data struct {
		MediaListCollection MediaListCollection `json:"MediaListCollection"`
	} `json:"data"`
}

type MediaListResponse struct {
	MediaList *MediaListEntry `json:"MediaList"`
}

type SaveMediaListResponse struct {
	SaveMediaListEntry MediaListEntry `json:"SaveMediaListEntry"`
}
