package kitsu

// JSON:API response types for kitsu.io/api/edge.

// AnimeResponse is the top-level response of an anime listing request. The
// resource objects live in Data, and resources sideloaded with ?include=
// (here the mappings) live in Included.
type AnimeResponse struct {
	Data     []*AnimeResource `json:"data"`
	Included []*MappingResource `json:"included,omitempty"`
	Meta     AnimeMeta        `json:"meta,omitempty"`
}

// AnimeMeta carries the count Kitsu reports for a filter query.
type AnimeMeta struct {
	Count int `json:"count"`
}

// AnimeResource is one anime resource object.
type AnimeResource struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes *AnimeAttributes `json:"attributes"`
	// Relationships holds the related links; only the mapping ids are
	// interesting here and they are read from the included resources.
	Relationships *AnimeRelationships `json:"relationships"`
}

// AnimeAttributes are the anime fields the TUI shows or needs for ID
// resolution.
type AnimeAttributes struct {
	CanonicalTitle string            `json:"canonicalTitle"`
	Titles         map[string]string `json:"titles"`
	Slug           string            `json:"slug"`
	Synopsis       string            `json:"synopsis"`
	PosterImage    ImageLinks        `json:"posterImage"`
	CoverImage     ImageLinks        `json:"coverImage"`
	EpisodeCount  *int               `json:"episodeCount"`
	EpisodeLength *int               `json:"episodeLength"`
	ShowType       string            `json:"showType"`
	Status         string            `json:"status"`
	AverageRating  string            `json:"averageRating"`
	PopularityRank int               `json:"popularityRank"`
	RatingRank     int               `json:"ratingRank"`
	StartDate      string            `json:"startDate"`
	EndDate        string            `json:"endDate"`
	AgeRating      string            `json:"ageRating"`
	NSFW           bool              `json:"nsfw"`
}

// ImageLinks are the pre-rendered image variants Kitsu serves.
type ImageLinks struct {
	Tiny     string `json:"tiny"`
	Small    string `json:"small"`
	Medium   string `json:"medium"`
	Large    string `json:"large"`
	Original string `json:"original"`
}

// AnimeRelationships lists the resource links of an anime; only mappings
// matter here.
type AnimeRelationships struct {
	Mappings *RelationshipList `json:"mappings"`
}

// RelationshipList is the JSON:API relationship object: a list of resource
// identifiers.
type RelationshipList struct {
	Data []*RelationshipRef `json:"data"`
}

// RelationshipRef is one resource identifier inside a relationship.
type RelationshipRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// MappingResource is a sideloaded mapping, e.g. "myanimelist/anime" -> 21.
// It carries the external site and the external id that the TUI needs.
type MappingResource struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes *MappingAttributes `json:"attributes"`
}

// MappingAttributes identify the external resource a mapping points at.
type MappingAttributes struct {
	ExternalSite string `json:"externalSite"`
	ExternalID   string `json:"externalId"`
}
