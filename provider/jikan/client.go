package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hishantik/anilix/curl"
)

const (
	jikanBaseURL = "https://api.jikan.moe/v4"
	RateLimit    = 3 // requests per second
)

type JikanClient struct {
	baseURL     string
	rateLimiter *rateLimiter
	cache       map[int]*AnimeData
	cacheMu     sync.RWMutex
}

type rateLimiter struct {
	tokens        int
	maxTokens     int
	refillInterval time.Duration
	lastRefill    time.Time
	mu           sync.Mutex
}

func newRateLimiter(maxTokens int, refillInterval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refillInterval: refillInterval,
		lastRefill:    time.Now(),
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

func NewClient(baseURL string) *JikanClient {
	if baseURL == "" {
		baseURL = jikanBaseURL
	}
	return &JikanClient{
		baseURL:     baseURL,
		rateLimiter: newRateLimiter(RateLimit, time.Second),
		cache:       make(map[int]*AnimeData),
	}
}

func (c *JikanClient) SearchAnime(ctx context.Context, query string) (*AnimeResponse, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime?q=%s&limit=20", c.baseURL, query)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result AnimeResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetAnime fetches full anime details by MAL ID (with caching)
func (c *JikanClient) IsCached(malID int) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	_, ok := c.cache[malID]
	return ok
}

func (c *JikanClient) GetAnime(ctx context.Context, malID int) (*AnimeData, error) {
	// Check cache first
	c.cacheMu.RLock()
	if cached, ok := c.cache[malID]; ok {
		c.cacheMu.RUnlock()
		return cached, nil
	}
	c.cacheMu.RUnlock()

	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d", c.baseURL, malID)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result AnimeSingleResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the result
	c.cacheMu.Lock()
	c.cache[malID] = &result.Data
	c.cacheMu.Unlock()

	return &result.Data, nil
}

// GetEpisodes fetches episode list for an anime by MAL ID
func (c *JikanClient) GetEpisodes(ctx context.Context, malID int) ([]Episode, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d/episodes", c.baseURL, malID)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result EpisodesResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetTopAnime fetches top/popular anime from Jikan
func (c *JikanClient) GetTopAnime(ctx context.Context, filter string, limit int) ([]AnimeData, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/top/anime?filter=%s&limit=%d", c.baseURL, filter, limit)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result AnimeResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GenresResponse represents the Jikan API genres response
type GenresResponse struct {
	Data []Genre `json:"data"`
}

// GetAnimeGenres fetches the list of anime genres from Jikan
func (c *JikanClient) GetAnimeGenres(ctx context.Context) ([]Genre, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/genres/anime", c.baseURL)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result GenresResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetEpisode fetches a single episode by MAL ID and episode number
func (c *JikanClient) GetEpisode(ctx context.Context, malID int, episodeNumber int) (*Episode, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d/episodes/%d", c.baseURL, malID, episodeNumber)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result EpisodeSingleResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}