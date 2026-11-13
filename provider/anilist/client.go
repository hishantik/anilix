package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	BaseURL   = "https://graphql.anilist.co"
	RateLimit = 10 // requests per second (AniList is generous)
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
)

type Client struct {
	baseURL     string
	token       string
	rateLimiter *rateLimiter
	cache       map[int]*MediaData
	cacheMu     sync.RWMutex
}

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
		time.Sleep(50 * time.Millisecond)
	}
}

func NewClient() *Client {
	return &Client{
		baseURL:     BaseURL,
		rateLimiter: newRateLimiter(RateLimit, time.Second),
		cache:       make(map[int]*MediaData),
	}
}

func NewAuthenticatedClient(token string) *Client {
	return &Client{
		baseURL:     BaseURL,
		token:       token,
		rateLimiter: newRateLimiter(RateLimit, time.Second),
		cache:       make(map[int]*MediaData),
	}
}

func (c *Client) IsCached(anilistID int) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	_, ok := c.cache[anilistID]
	return ok
}

// GetAnime fetches anime details by AniList ID
func (c *Client) GetAnime(ctx context.Context, anilistID int) (*MediaData, error) {
	c.cacheMu.RLock()
	if cached, ok := c.cache[anilistID]; ok {
		c.cacheMu.RUnlock()
		return cached, nil
	}
	c.cacheMu.RUnlock()

	c.rateLimiter.waitForToken()

	query := `query ($id: Int) {
		Media(id: $id, type: ANIME) {
			id
			title { romaji english native userPreferred }
			coverImage { extraLarge large medium color }
			type
			format
			status
			description(asHtml: false)
			startDate { year month day }
			endDate { year month day }
			season
			seasonYear
			episodes
			duration
			genres
			synonyms
			averageScore
			meanScore
			popularity
			trending
			favourites
			siteUrl
		}
	}`

	variables := map[string]interface{}{
		"id": anilistID,
	}

	resp, err := c.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	var result MediaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Media.ID == 0 {
		return nil, fmt.Errorf("no media found for ID %d", anilistID)
	}

	c.cacheMu.Lock()
	c.cache[anilistID] = &result.Media
	c.cacheMu.Unlock()

	return &result.Media, nil
}

// GetAnimeBatch fetches multiple anime details in a single GraphQL query
func (c *Client) GetAnimeBatch(ctx context.Context, ids []int) (map[int]*MediaData, error) {
	if len(ids) == 0 {
		return make(map[int]*MediaData), nil
	}

	uncached := make([]int, 0, len(ids))
	results := make(map[int]*MediaData)

	c.cacheMu.RLock()
	for _, id := range ids {
		if cached, ok := c.cache[id]; ok {
			results[id] = cached
		} else {
			uncached = append(uncached, id)
		}
	}
	c.cacheMu.RUnlock()

	if len(uncached) == 0 {
		return results, nil
	}

	c.rateLimiter.waitForToken()

	var queryParts []string
	var varParts []string

	for i := range uncached {
		alias := fmt.Sprintf("q%d", i)
		queryParts = append(queryParts, fmt.Sprintf("%s: Media(id: $id%d, type: ANIME) { id title { romaji english native userPreferred } coverImage { extraLarge large medium } type format status description(asHtml: false) startDate { year } seasonYear episodes genres averageScore meanScore popularity siteUrl }", alias, i))
		varParts = append(varParts, fmt.Sprintf("$id%d: Int", i))
	}

	query := fmt.Sprintf("query (%s) { %s }", strings.Join(varParts, ", "), strings.Join(queryParts, " "))

	variables := make(map[string]interface{})
	for i, id := range uncached {
		variables[fmt.Sprintf("id%d", i)] = id
	}

	resp, err := c.doGraphQL(ctx, query, variables)
	if err != nil {
		return results, err
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		return results, fmt.Errorf("failed to decode response: %w", err)
	}

	for i, id := range uncached {
		alias := fmt.Sprintf("q%d", i)
		if data, ok := rawResp[alias].(map[string]interface{}); ok && data != nil {
			dataBytes, _ := json.Marshal(data)
			var media MediaData
			if err := json.Unmarshal(dataBytes, &media); err == nil && media.ID != 0 {
				results[id] = &media
				c.cacheMu.Lock()
				c.cache[id] = &media
				c.cacheMu.Unlock()
			}
		}
	}

	return results, nil
}

// ResolveByTitles resolves several titles to AniList entries using a single
// GraphQL request (aliased Page fields, one per title).
//
// It is used to attach AniList IDs and MAL IDs to search results: AniList
// entries carry both, so one request gives the TUI everything it needs for
// metadata and AniSkip. Only exact title matches are accepted - a fuzzy
// match could attach the wrong anime (e.g. "Mushishi: The Next Chapter" vs
// "MUSHI-SHI The Next Passage"), so unmatched titles are simply left empty.
func (c *Client) ResolveByTitles(ctx context.Context, titles []string) ([]TitleMatch, error) {
	matches := make([]TitleMatch, len(titles))
	for i, t := range titles {
		matches[i].Query = t
	}
	if len(titles) == 0 {
		return matches, nil
	}

	// Keep requests within reasonable GraphQL complexity limits.
	const chunkSize = 10
	for start := 0; start < len(titles); start += chunkSize {
		end := start + chunkSize
		if end > len(titles) {
			end = len(titles)
		}
		chunk, err := c.resolveTitleChunk(ctx, titles[start:end])
		if err != nil {
			return matches, err
		}
		for i, m := range chunk {
			matches[start+i] = m
		}
	}
	return matches, nil
}

// resolveTitleChunk resolves one chunk of titles in a single request.
func (c *Client) resolveTitleChunk(ctx context.Context, titles []string) ([]TitleMatch, error) {
	c.rateLimiter.waitForToken()

	var queryParts []string
	var varParts []string
	variables := make(map[string]interface{})

	for i, t := range titles {
		alias := fmt.Sprintf("p%d", i)
		queryParts = append(queryParts, fmt.Sprintf("%s: Page(page: 1, perPage: 10) { media(search: $t%d, type: ANIME) { id idMal title { romaji english native } synonyms format episodes coverImage { extraLarge } startDate { year } } }", alias, i))
		varParts = append(varParts, fmt.Sprintf("$t%d: String", i))
		variables[fmt.Sprintf("t%d", i)] = t
	}

	query := fmt.Sprintf("query (%s) { %s }", strings.Join(varParts, ", "), strings.Join(queryParts, " "))

	resp, err := c.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	matches := make([]TitleMatch, len(titles))
	for i, t := range titles {
		matches[i].Query = t

		alias := fmt.Sprintf("p%d", i)
		page, ok := rawResp[alias].(map[string]interface{})
		if !ok {
			continue
		}
		media, ok := page["media"].([]interface{})
		if !ok {
			continue
		}
		for _, entry := range media {
			dataBytes, _ := json.Marshal(entry)
			var m MediaData
			if err := json.Unmarshal(dataBytes, &m); err != nil || m.ID == 0 {
				continue
			}
			if titleMatches(t, &m) {
				copied := m
				matches[i].Media = &copied
				break
			}
		}
	}
	return matches, nil
}

// titleMatches reports whether the entry's title is the queried title.
// Comparison is case/space/punctuation insensitive and also considers
// synonyms, but never accepts a merely similar title.
func titleMatches(query string, media *MediaData) bool {
	q := normalizeTitle(query)
	if q == "" {
		return false
	}
	for _, t := range []string{media.Title.English, media.Title.Romaji, media.Title.Native} {
		if t != "" && normalizeTitle(t) == q {
			return true
		}
	}
	for _, s := range media.Synonyms {
		if s != "" && normalizeTitle(s) == q {
			return true
		}
	}
	return false
}

// normalizeTitle lowercases a title and keeps only letters and digits, so
// "ONE PIECE" and "One Piece!" compare equal. Unicode-aware on purpose:
// AniList uses typographic punctuation ("Journey's End" with U+2019) while
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

func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	reqBody, err := json.Marshal(GraphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	return c.doGraphQLCurl(ctx, reqBody)
}

func (c *Client) doGraphQLCurl(ctx context.Context, reqBody []byte) ([]byte, error) {
	agent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	args := []string{"-s", "-X", "POST", c.baseURL,
		"-H", "Content-Type: application/json",
		"-H", "User-Agent: " + agent,
		"-H", "Referer: https://anilist.co",
	}

	if c.token != "" {
		args = append(args, "-H", "Authorization: Bearer "+c.token)
	}

	args = append(args, "-d", "@-")

	cmd := exec.CommandContext(ctx, "curl", args...)

	cmd.Stdin = bytes.NewReader(reqBody)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result GraphQLResponse
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse curl response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

// SearchAnime searches for anime by title
func (c *Client) SearchAnime(ctx context.Context, query string, limit int) ([]MediaData, error) {
	c.rateLimiter.waitForToken()

	gql := `query ($search: String, $perPage: Int) {
		Page(page: 1, perPage: $perPage) {
			media(search: $search, type: ANIME, sort: [SEARCH_MATCH]) {
				id
				title { romaji english native userPreferred }
				coverImage { extraLarge large medium }
				type
				format
				status
				description(asHtml: false)
				startDate { year }
				seasonYear
				episodes
				genres
				averageScore
				meanScore
				popularity
				siteUrl
			}
		}
	}`

	variables := map[string]interface{}{
		"search":  query,
		"perPage": limit,
	}

	resp, err := c.doGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, err
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	page, ok := rawResp["Page"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no Page in response")
	}

	mediaList, ok := page["media"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no media in response")
	}

	var results []MediaData
	for _, m := range mediaList {
		dataBytes, _ := json.Marshal(m)
		var media MediaData
		if err := json.Unmarshal(dataBytes, &media); err == nil && media.ID != 0 {
			results = append(results, media)
		}
	}

	return results, nil
}
