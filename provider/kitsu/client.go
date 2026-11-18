package kitsu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hishantik/anilix/curl"
)

const (
	// BaseURL is the JSON:API endpoint of Kitsu.
	BaseURL = "https://kitsu.io/api/edge"
	// RateLimit keeps requests polite; Kitsu served 10 rapid requests
	// without a reply, but there is no published rate limit to lean on.
	RateLimit = 3
	// UserAgent identifies the client to the API.
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
)

// Client talks to the Kitsu JSON:API. It is a metadata client like the
// AniList and Jikan clients: the TUI holds one instance for the session.
type Client struct {
	baseURL     string
	rateLimiter *rateLimiter
	cache       map[string]*Anime
	cacheMu     sync.RWMutex
}

// rateLimiter is a token bucket, shared shape with the other clients.
type rateLimiter struct {
	tokens         int
	maxTokens      int
	refillInterval time.Duration
	lastRefill     time.Time
	mu             sync.Mutex
}

func newRateLimiter(maxTokens int, refillInterval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillInterval: refillInterval,
		lastRefill:     time.Now(),
	}
}

func (r *rateLimiter) acquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Since(r.lastRefill)
	if now >= r.refillInterval {
		r.tokens = r.maxTokens
		r.lastRefill = time.Now()
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

func (r *rateLimiter) waitForToken() {
	for !r.acquire() {
		time.Sleep(100 * time.Millisecond)
	}
}

// NewClient builds a client for the public Kitsu API.
func NewClient() *Client {
	return NewClientWithBaseURL(BaseURL)
}

// NewClientWithBaseURL builds a client against another endpoint, used by
// the tests to point at a stub server.
func NewClientWithBaseURL(baseURL string) *Client {
	if baseURL == "" {
		baseURL = BaseURL
	}
	return &Client{
		baseURL:     baseURL,
		rateLimiter: newRateLimiter(RateLimit, time.Second),
		cache:       make(map[string]*Anime),
	}
}

// Anime is a Kitsu entry flattened to what the TUI needs: the titles for
// matching, the attributes for display and the external ids that drive
// AniList metadata and AniSkip.
type Anime struct {
	KitsuID       string
	Canonical     string
	Titles        map[string]string
	Synopsis      string
	Poster        string
	Cover         string
	EpisodeCount  int
	EpisodeLength int
	ShowType      string
	Status        string
	StartDate     string
	AverageRating float64
	NSFW          bool

	// AniListID and MALID come from the sideloaded mappings; both are
	// zero when Kitsu has no such mapping for the entry.
	AniListID int
	MALID     int
}

// TitleMatch is the result of resolving one search title to a Kitsu entry.
// Anime is nil when no entry matched the title exactly.
type TitleMatch struct {
	Query string
	Anime *Anime
}

// ResolveByTitles resolves several titles to Kitsu entries using one
// request per chunk of titles. Each request carries the sideloaded
// mappings, so the AniList and MAL ids of the whole page arrive together.
//
// Matching is exact after normalisation, never fuzzy, because a near-match
// would attach the wrong anime. Unmatched titles are simply left empty.
func (c *Client) ResolveByTitles(ctx context.Context, titles []string) ([]TitleMatch, error) {
	matches := make([]TitleMatch, len(titles))
	for i, t := range titles {
		matches[i].Query = t
	}
	if len(titles) == 0 {
		return matches, nil
	}

	// One request per title: Kitsu filters on one text at a time, and a
	// chunk of independent searches cannot share a request the way an
	// aliased GraphQL query can. The rate limiter keeps the request rate
	// civilised.
	for i, t := range titles {
		anime, err := c.SearchFirst(ctx, t)
		if err != nil {
			return matches, err
		}
		matches[i].Anime = anime
	}
	return matches, nil
}

// SearchFirst searches Kitsu for the title and returns the first entry
// whose title matches exactly, with the AniList and MAL ids from the
// sideloaded mappings. It returns nil when nothing matches.
func (c *Client) SearchFirst(ctx context.Context, title string) (*Anime, error) {
	resp, err := c.search(ctx, title, 20)
	if err != nil {
		return nil, err
	}

	want := normalizeTitle(title)
	if want == "" {
		return nil, nil
	}
	for _, a := range resp.Animes {
		if a.matchesTitle(want) {
			return a, nil
		}
	}
	return nil, nil
}

// matchesTitle reports whether the entry is the queried title.
// Comparison is case/space/punctuation insensitive over every known title
// form, but never accepts a merely similar title.
func (a *Anime) matchesTitle(normalizedQuery string) bool {
	for _, t := range a.titleForms() {
		if t != "" && normalizeTitle(t) == normalizedQuery {
			return true
		}
	}
	return false
}

// titleForms lists the title strings of the entry that a query may match.
func (a *Anime) titleForms() []string {
	forms := make([]string, 0, len(a.Titles)+1)
	forms = append(forms, a.Canonical)
	for _, t := range a.Titles {
		forms = append(forms, t)
	}
	return forms
}

// search performs one text filter against the Kitsu API with mappings
// sideloaded, and flattens the JSON:API envelope.
func (c *Client) search(ctx context.Context, text string, limit int) (*searchResult, error) {
	// url.Values.Encode percent-encodes the square brackets of the JSON:API
	// filters: the raw "filter[text]=" form is read by curl's glob engine
	// as a character range and fails with exit 3.
	values := url.Values{}
	values.Set("filter[text]", text)
	values.Set("page[limit]", fmt.Sprintf("%d", limit))
	values.Set("include", "mappings")
	u := fmt.Sprintf("%s/anime?%s", c.baseURL, values.Encode())

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var payload AnimeResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// The mappings arrive as a flat "included" array; the anime
	// relationships name the mapping ids each entry owns. Build the
	// mapping lookup once and walk every anime.
	mappings := make(map[string]*MappingAttributes, len(payload.Included))
	for _, inc := range payload.Included {
		if inc.Type == "mappings" && inc.Attributes != nil {
			mappings[inc.ID] = inc.Attributes
		}
	}

	result := &searchResult{
		Animes: make([]*Anime, 0, len(payload.Data)),
		Count:  payload.Meta.Count,
	}
	for _, res := range payload.Data {
		if res == nil || res.Attributes == nil {
			continue
		}
		anime := flattenAnime(res)
		applyMappings(anime, res, mappings)
		result.Animes = append(result.Animes, anime)
	}
	return result, nil
}

// searchResult is the flattened page of a search.
type searchResult struct {
	Animes []*Anime
	Count  int
}

// flattenAnime copies the resource attributes into the flat Anime.
func flattenAnime(res *AnimeResource) *Anime {
	attrs := res.Attributes
	anime := &Anime{
		KitsuID:   res.ID,
		Canonical: attrs.CanonicalTitle,
		Titles:    attrs.Titles,
		Synopsis:  attrs.Synopsis,
		ShowType:  attrs.ShowType,
		Status:    attrs.Status,
		StartDate: attrs.StartDate,
		NSFW:      attrs.NSFW,
	}
	anime.Poster = pickImage(attrs.PosterImage)
	anime.Cover = pickImage(attrs.CoverImage)
	if attrs.EpisodeCount != nil {
		anime.EpisodeCount = *attrs.EpisodeCount
	}
	if attrs.EpisodeLength != nil {
		anime.EpisodeLength = *attrs.EpisodeLength
	}
	if r, err := parseRating(attrs.AverageRating); err == nil {
		anime.AverageRating = r
	}
	return anime
}

// applyMappings reads the anime's mapping relationship and copies the
// AniList and MAL ids from the sideloaded mappings.
func applyMappings(anime *Anime, res *AnimeResource, mappings map[string]*MappingAttributes) {
	if res.Relationships == nil || res.Relationships.Mappings == nil {
		return
	}
	for _, ref := range res.Relationships.Mappings.Data {
		attrs, ok := mappings[ref.ID]
		if !ok {
			continue
		}
		switch attrs.ExternalSite {
		case "myanimelist/anime":
			anime.MALID = atoi(attrs.ExternalID)
		case "anilist/anime":
			anime.AniListID = atoi(attrs.ExternalID)
		}
	}
}

// pickImage returns the best available image link, largest first.
func pickImage(links ImageLinks) string {
	for _, s := range []string{links.Original, links.Large, links.Medium, links.Small, links.Tiny} {
		if s != "" {
			return s
		}
	}
	return ""
}

func parseRating(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty rating")
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0, err
	}
	return v, nil
}

func atoi(s string) int {
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		v = v*10 + int(r-'0')
	}
	return v
}

// normalizeTitle lowercases a title and keeps only letters and digits, so
// "ONE PIECE" and "One Piece!" compare equal. Unicode-aware on purpose:
// titles carry typographic punctuation ("Journey's End" with U+2019) while
// other sources use plain ASCII, and both must match.
func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// get performs one GET through the curl package, as every other provider
// does: it keeps the PRoot/Termux DNS and TLS behaviour consistent.
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	headers := map[string]string{
		"Accept":     "application/vnd.api+json",
		"User-Agent": UserAgent,
	}
	body, err := curl.Get(ctx, u, headers)
	if err != nil {
		return nil, err
	}
	return []byte(body), nil
}
