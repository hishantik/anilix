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

type MediaListResponse struct {
	MediaList *MediaListEntry `json:"MediaList"`
}

type SaveMediaListResponse struct {
	SaveMediaListEntry MediaListEntry `json:"SaveMediaListEntry"`
}
