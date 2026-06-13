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
)

const (
	BaseURL    = "https://graphql.anilist.co"
	RateLimit  = 10 // requests per second (AniList is generous)
	UserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
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

// GetTrendingAnime fetches currently trending anime from AniList
func (c *Client) GetTrendingAnime(ctx context.Context, limit int) ([]MediaData, error) {
	c.rateLimiter.waitForToken()

	gql := `query ($perPage: Int, $type: MediaType, $sort: [MediaSort]) {
		Page(page: 1, perPage: $perPage) {
			media(type: $type, sort: $sort, countryOfOrigin: JP) {
				id
				title { romaji english native userPreferred }
				coverImage { extraLarge large medium }
				type format status
				description(asHtml: false)
				startDate { year }
				seasonYear
				episodes genres
				averageScore popularity trending
			}
		}
	}`

	variables := map[string]interface{}{
		"perPage": limit,
		"type":    "ANIME",
		"sort":    []string{"TRENDING_DESC"},
	}

	resp, err := c.doGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, err
	}

	return c.parseMediaPage(resp)
}

// GetAnimeByGenre fetches popular anime filtered by genre from AniList
func (c *Client) GetAnimeByGenre(ctx context.Context, genre string, limit int) ([]MediaData, error) {
	c.rateLimiter.waitForToken()

	gql := `query ($genre: String, $perPage: Int) {
		Page(page: 1, perPage: $perPage) {
			media(type: ANIME, genre: $genre, sort: [POPULARITY_DESC], countryOfOrigin: JP) {
				id
				title { romaji english native userPreferred }
				coverImage { extraLarge large medium }
				type format status
				description(asHtml: false)
				startDate { year }
				seasonYear
				episodes genres
				averageScore popularity
			}
		}
	}`

	variables := map[string]interface{}{
		"genre":   genre,
		"perPage": limit,
	}

	resp, err := c.doGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, err
	}

	return c.parseMediaPage(resp)
}

// GetPopularAnime fetches most popular anime from AniList
func (c *Client) GetPopularAnime(ctx context.Context, limit int) ([]MediaData, error) {
	c.rateLimiter.waitForToken()

	gql := `query ($perPage: Int) {
		Page(page: 1, perPage: $perPage) {
			media(type: ANIME, sort: [POPULARITY_DESC], countryOfOrigin: JP) {
				id
				title { romaji english native userPreferred }
				coverImage { extraLarge large medium }
				type format status
				startDate { year }
				seasonYear
				episodes genres
				averageScore popularity
			}
		}
	}`

	variables := map[string]interface{}{
		"perPage": limit,
	}

	resp, err := c.doGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, err
	}

	return c.parseMediaPage(resp)
}

// parseMediaPage extracts MediaData from a paginated GraphQL response
func (c *Client) parseMediaPage(resp []byte) ([]MediaData, error) {
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

// GetRecommendations fetches recommended anime for a given AniList ID
func (c *Client) GetRecommendations(ctx context.Context, anilistID int, limit int) ([]MediaData, error) {
	c.rateLimiter.waitForToken()

	gql := `query ($id: Int, $perPage: Int) {
		Media(id: $id, type: ANIME) {
			recommendations(perPage: $perPage, sort: RATING_DESC) {
				nodes {
					id
					mediaRecommendation {
						id
						title { romaji english native userPreferred }
						coverImage { extraLarge large medium }
						type format status
						startDate { year }
						episodes genres
						averageScore popularity
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"id":      anilistID,
		"perPage": limit,
	}

	resp, err := c.doGraphQL(ctx, gql, variables)
	if err != nil {
		return nil, err
	}

	return parseRecommendationsResponse(resp, limit), nil
}

// parseRecommendationsResponse extracts a []MediaData from an AniList
// recommendations response. The shape is:
//
//	Media.recommendations.nodes[].mediaRecommendation
//
// where `Media.recommendations` is a RecommendationConnection whose list field
// is `nodes`, and each Recommendation node wraps the recommended Media under
// `mediaRecommendation` (not `node` and not `recommendations`).
func parseRecommendationsResponse(raw []byte, limit int) []MediaData {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(raw, &rawResp); err != nil {
		return nil
	}

	mediaObj, ok := rawResp["Media"].(map[string]interface{})
	if !ok {
		return nil
	}

	recsObj, ok := mediaObj["recommendations"].(map[string]interface{})
	if !ok {
		return []MediaData{}
	}

	recsList, ok := recsObj["nodes"].([]interface{})
	if !ok {
		return []MediaData{}
	}

	results := make([]MediaData, 0, len(recsList))
	for _, r := range recsList {
		recObj, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		// Each Recommendation wraps the recommended Media under
		// mediaRecommendation — NOT `node` (which doesn't exist on
		// Recommendation).
		nodeObj, ok := recObj["mediaRecommendation"].(map[string]interface{})
		if !ok {
			continue
		}
		dataBytes, _ := json.Marshal(nodeObj)
		var media MediaData
		if err := json.Unmarshal(dataBytes, &media); err == nil && media.ID != 0 {
			results = append(results, media)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
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
