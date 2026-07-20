package miruro

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type browserResponse struct {
	Status     int
	Body       string
	Obfuscated string
}

type browserFetcher interface {
	Fetch(ctx context.Context, origin, relativeURL string) (browserResponse, error)
	Close() error
}

type BrowserTransport struct {
	fetcher browserFetcher
	origins []string
}

func NewBrowserTransport(fetcher browserFetcher, origins []string) *BrowserTransport {
	return &BrowserTransport{fetcher: fetcher, origins: append([]string(nil), origins...)}
}

func (t *BrowserTransport) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	pipeQuery := make(map[string]string, len(query))
	for key := range query {
		pipeQuery[key] = query.Get(key)
	}
	encoded, err := encodePipeRequest(path, pipeQuery)
	if err != nil {
		return nil, err
	}
	relativeURL := pipePath + "?e=" + url.QueryEscape(encoded)

	var lastErr error
	for _, origin := range t.origins {
		response, err := t.fetcher.Fetch(ctx, strings.TrimRight(origin, "/"), relativeURL)
		if err != nil {
			lastErr = err
			continue
		}
		if response.Status == http.StatusForbidden || response.Status == http.StatusUnauthorized || response.Status == 444 {
			lastErr = fmt.Errorf("%w: browser HTTP %d", ErrAccessDenied, response.Status)
			continue
		}
		if response.Status != http.StatusOK {
			lastErr = fmt.Errorf("Miruro browser pipe returned HTTP %d", response.Status)
			continue
		}
		return decodePipeResponse([]byte(response.Body), response.Obfuscated)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no Miruro browser origins configured")
	}
	return nil, lastErr
}

func (t *BrowserTransport) Close() error { return t.fetcher.Close() }
