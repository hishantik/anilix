package miruro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const pipePath = "/api/secure/pipe"

var ErrAccessDenied = errors.New("Miruro access denied")

type Transport interface {
	Get(ctx context.Context, path string, query url.Values) ([]byte, error)
	Close() error
}

type HTTPTransport struct {
	client  *http.Client
	origins []string
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

	var lastErr error
	denied := false
	for _, origin := range t.origins {
		body, err := t.getOrigin(ctx, strings.TrimRight(origin, "/"), encoded)
		if err == nil {
			return body, nil
		}
		if errors.Is(err, ErrAccessDenied) {
			denied = true
		}
		lastErr = err
	}
	if denied {
		return nil, fmt.Errorf("%w on all Miruro mirrors", ErrAccessDenied)
	}
	if lastErr == nil {
		lastErr = errors.New("no Miruro mirrors configured")
	}
	return nil, lastErr
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
