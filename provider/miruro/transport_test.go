package miruro

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHTTPTransportRotatesMirrorsAfterForbidden(t *testing.T) {
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer blocked.Close()

	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pipePath {
			t.Fatalf("path = %q", r.URL.Path)
		}
		raw, err := base64.RawURLEncoding.DecodeString(r.URL.Query().Get("e"))
		if err != nil {
			t.Fatalf("decode request: %v", err)
		}
		var envelope pipeEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if envelope.Path != "episodes" || envelope.Query["anilistId"] != "20" {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"providers":{}}`))
	}))
	defer working.Close()

	transport := NewHTTPTransport(http.DefaultClient, []string{blocked.URL, working.URL})
	body, err := transport.Get(context.Background(), "episodes", url.Values{"anilistId": {"20"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"providers":{}}` {
		t.Fatalf("body = %s", body)
	}
}

type stubTransport struct {
	body  []byte
	err   error
	calls int
}

func (s *stubTransport) Get(context.Context, string, url.Values) ([]byte, error) {
	s.calls++
	return s.body, s.err
}

func (s *stubTransport) Close() error { return nil }

func TestFallbackTransportUsesBrowserAfterAccessDenied(t *testing.T) {
	httpTransport := &stubTransport{err: ErrAccessDenied}
	browserTransport := &stubTransport{body: []byte(`{"providers":{"kiwi":{}}}`)}
	transport := NewFallbackTransport(httpTransport, browserTransport)

	body, err := transport.Get(context.Background(), "episodes", url.Values{"anilistId": {"20"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"providers":{"kiwi":{}}}` {
		t.Fatalf("body = %s", body)
	}
	if httpTransport.calls != 1 || browserTransport.calls != 1 {
		t.Fatalf("calls: http=%d browser=%d", httpTransport.calls, browserTransport.calls)
	}
}

func TestFallbackTransportDoesNotHideNonAccessError(t *testing.T) {
	want := errors.New("bad response")
	httpTransport := &stubTransport{err: want}
	browserTransport := &stubTransport{body: []byte(`{}`)}
	transport := NewFallbackTransport(httpTransport, browserTransport)

	_, err := transport.Get(context.Background(), "episodes", nil)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if browserTransport.calls != 0 {
		t.Fatalf("browser calls = %d, want 0", browserTransport.calls)
	}
}
