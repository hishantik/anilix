package miruro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const pipePath = "/api/secure/pipe"

var ErrAccessDenied = errors.New("Miruro access denied")

var OfficialOrigins = []string{
	"https://www.miruro.tv",
	"https://www.miruro.to",
	"https://www.miruro.bz",
	"https://www.miruro.ru",
}

type Transport interface {
	Get(ctx context.Context, path string, query url.Values) ([]byte, error)
	Close() error
}

func NewDefaultTransport(profileDir string) (Transport, error) {
	primary := NewHTTPTransport(&http.Client{Timeout: 20 * time.Second}, OfficialOrigins)
	browser, err := NewManagedBrowserTransport(profileDir, OfficialOrigins)
	if err != nil {
		browser = errorTransport{err: err}
	}
	return NewFallbackTransport(primary, browser), nil
}

type errorTransport struct{ err error }

func (t errorTransport) Get(context.Context, string, url.Values) ([]byte, error) {
	return nil, t.err
}
func (errorTransport) Close() error { return nil }

type HTTPTransport struct {
	client          *http.Client
	origins         []string
	mu              sync.RWMutex
	preferredOrigin string
}

type mirrorResult struct {
	origin string
	body   []byte
	err    error
}

func NewHTTPTransport(client *http.Client, origins []string) *HTTPTransport {
	return &HTTPTransport{client: client, origins: append([]string(nil), origins...)}
}

func (t *HTTPTransport) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	pipeQuery := make(map[string]string, len(query))
	for key := range query {
		pipeQuery[key] = query.Get(key)
	}
	encoded, err := encodePipeRequest(path, pipeQuery)
	if err != nil {
		return nil, err
	}

	origins := t.orderedOrigins()
	if len(origins) == 0 {
		return nil, errors.New("no Miruro mirrors configured")
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan mirrorResult, len(origins))
	launch := func(origin string) {
		go func() {
			attemptCtx, attemptCancel := context.WithTimeout(raceCtx, 5*time.Second)
			defer attemptCancel()
			body, err := t.getOrigin(attemptCtx, strings.TrimRight(origin, "/"), encoded)
			results <- mirrorResult{origin: origin, body: body, err: err}
		}()
	}

	launched := 0
	launch(origins[0])
	launched++
	if t.preferred() != "" && len(origins) > 1 {
		timer := time.NewTimer(75 * time.Millisecond)
		select {
		case first := <-results:
			timer.Stop()
			if first.err == nil {
				t.setPreferred(first.origin)
				return first.body, nil
			}
			for _, origin := range origins[1:] {
				launch(origin)
				launched++
			}
			return t.collectMirrorResults(ctx, results, launched-1, []error{first.err})
		case <-timer.C:
			for _, origin := range origins[1:] {
				launch(origin)
				launched++
			}
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	} else {
		for _, origin := range origins[1:] {
			launch(origin)
			launched++
		}
	}
	return t.collectMirrorResults(ctx, results, launched, nil)
}

func (t *HTTPTransport) collectMirrorResults(ctx context.Context, results <-chan mirrorResult, count int, failures []error) ([]byte, error) {
	for i := 0; i < count; i++ {
		select {
		case result := <-results:
			if result.err == nil {
				t.setPreferred(result.origin)
				return result.body, nil
			}
			failures = append(failures, result.err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	joined := errors.Join(failures...)
	if errors.Is(joined, ErrAccessDenied) {
		return nil, fmt.Errorf("%w on Miruro mirrors: %v", ErrAccessDenied, joined)
	}
	return nil, joined
}

func (t *HTTPTransport) preferred() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.preferredOrigin
}

func (t *HTTPTransport) setPreferred(origin string) {
	t.mu.Lock()
	t.preferredOrigin = origin
	t.mu.Unlock()
}

func (t *HTTPTransport) orderedOrigins() []string {
	origins := append([]string(nil), t.origins...)
	preferred := t.preferred()
	for i, origin := range origins {
		if origin == preferred {
			copy(origins[1:i+1], origins[0:i])
			origins[0] = preferred
			break
		}
	}
	return origins
}

func (t *HTTPTransport) getOrigin(ctx context.Context, origin, encoded string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+pipePath+"?e="+url.QueryEscape(encoded), nil)
	if err != nil {
		return nil, fmt.Errorf("build Miruro request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Miruro pipe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == 444 {
		return nil, fmt.Errorf("%w: HTTP %d", ErrAccessDenied, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Miruro pipe returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read Miruro pipe response: %w", err)
	}
	return decodePipeResponse(body, resp.Header.Get("X-Obfuscated"))
}

func (t *HTTPTransport) Close() error { return nil }

type FallbackTransport struct {
	primary  Transport
	fallback Transport
}

func NewFallbackTransport(primary, fallback Transport) *FallbackTransport {
	return &FallbackTransport{primary: primary, fallback: fallback}
}

func (t *FallbackTransport) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	body, err := t.primary.Get(ctx, path, query)
	if err == nil || !errors.Is(err, ErrAccessDenied) || t.fallback == nil {
		return body, err
	}
	return t.fallback.Get(ctx, path, query)
}

func (t *FallbackTransport) Close() error {
	var errs []error
	if t.primary != nil {
		errs = append(errs, t.primary.Close())
	}
	if t.fallback != nil {
		errs = append(errs, t.fallback.Close())
	}
	return errors.Join(errs...)
}
