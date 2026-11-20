package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

	// downUntil marks the API as unreachable (it goes down for hours at a
	// time). While set, requests are skipped instead of waiting for a
	// timeout, which would otherwise stall every search.
	downUntil time.Time
	downMu    sync.RWMutex
}

// outageWindow is how long the client keeps skipping requests after a
// failed call.
const outageWindow = 5 * time.Minute

// IsDown reports whether the API recently failed and should be skipped.
func (c *JikanClient) IsDown() bool {
	c.downMu.RLock()
	defer c.downMu.RUnlock()
	return time.Now().Before(c.downUntil)
}

// markDown records a failed call so subsequent requests are skipped.
func (c *JikanClient) markDown() {
	c.downMu.Lock()
	c.downUntil = time.Now().Add(outageWindow)
	c.downMu.Unlock()
}

// markUp clears the outage marker after a successful call.
func (c *JikanClient) markUp() {
	c.downMu.Lock()
	c.downUntil = time.Time{}
	c.downMu.Unlock()
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
	if c.IsDown() {
		return nil, fmt.Errorf("jikan api unreachable (recent failure), skipping")
	}
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime?q=%s&limit=20", c.baseURL, url.QueryEscape(query))

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		c.markDown()
		return nil, fmt.Errorf("curl failed: %w", err)
	}
	c.markUp()

	var result AnimeResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetTopAnime fetches the top airing anime (the Jikan /top/anime
// endpoint), which powers the discover screen.
func (c *JikanClient) GetTopAnime(ctx context.Context, filter string) (*AnimeResponse, error) {
	if c.IsDown() {
		return nil, fmt.Errorf("jikan api unreachable (recent failure), skipping")
	}
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/top/anime?limit=20&filter=%s", c.baseURL, url.QueryEscape(filter))

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		c.markDown()
		return nil, fmt.Errorf("curl failed: %w", err)
	}
	c.markUp()

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

	if c.IsDown() {
		return nil, fmt.Errorf("jikan api unreachable (recent failure), skipping")
	}

	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d", c.baseURL, malID)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		c.markDown()
		return nil, fmt.Errorf("curl failed: %w", err)
	}
	c.markUp()

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
	if c.IsDown() {
		return nil, fmt.Errorf("jikan api unreachable (recent failure), skipping")
	}
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d/episodes", c.baseURL, malID)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		c.markDown()
		return nil, fmt.Errorf("curl failed: %w", err)
	}
	c.markUp()

	var result EpisodesResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetEpisode fetches a single episode by MAL ID and episode number
func (c *JikanClient) GetEpisode(ctx context.Context, malID int, episodeNumber int) (*Episode, error) {
	// Same circuit breaker as every other call: Jikan goes down for hours
	// at a time and one dead endpoint must not stall playback.
	if c.IsDown() {
		return nil, fmt.Errorf("jikan api unreachable (recent failure), skipping")
	}
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d/episodes/%d", c.baseURL, malID, episodeNumber)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Accept":     "application/json",
	}

	response, err := curl.Get(ctx, url, headers)
	if err != nil {
		c.markDown()
		return nil, fmt.Errorf("curl failed: %w", err)
	}
	c.markUp()

	var result EpisodeSingleResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}
