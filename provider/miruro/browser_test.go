package miruro

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

type fakeBrowserFetcher struct {
	responses map[string]browserResponse
	calls     []string
}

func (f *fakeBrowserFetcher) Fetch(_ context.Context, origin, relativeURL string) (browserResponse, error) {
	f.calls = append(f.calls, origin+relativeURL)
	return f.responses[origin], nil
}

func (f *fakeBrowserFetcher) Close() error { return nil }

func TestBrowserTransportFetchesPipeFromPageOrigin(t *testing.T) {
	fetcher := &fakeBrowserFetcher{responses: map[string]browserResponse{
		"https://www.miruro.tv": {
			Status: http.StatusOK,
			Body:   `{"providers":{}}`,
		},
	}}
	transport := NewBrowserTransport(fetcher, []string{"https://www.miruro.tv"})

	body, err := transport.Get(context.Background(), "episodes", url.Values{"anilistId": {"20"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"providers":{}}` {
		t.Fatalf("body = %s", body)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] == "https://www.miruro.tv" {
		t.Fatalf("unexpected calls: %v", fetcher.calls)
	}
}

func TestBrowserTransportRotatesAfterForbidden(t *testing.T) {
	fetcher := &fakeBrowserFetcher{responses: map[string]browserResponse{
		"https://www.miruro.to": {Status: http.StatusForbidden, Body: "blocked"},
		"https://www.miruro.tv": {Status: http.StatusOK, Body: `{"providers":{"kiwi":{}}}`},
	}}
	transport := NewBrowserTransport(fetcher, []string{"https://www.miruro.to", "https://www.miruro.tv"})

	body, err := transport.Get(context.Background(), "episodes", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"providers":{"kiwi":{}}}` || len(fetcher.calls) != 2 {
		t.Fatalf("body=%s calls=%v", body, fetcher.calls)
	}
}
