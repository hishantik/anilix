package miruro

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
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

func TestHTTPTransportReturnsFastMirrorWithoutWaitingForSlowMirror(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer fast.Close()

	transport := NewHTTPTransport(http.DefaultClient, []string{slow.URL, fast.URL})
	started := time.Now()
	if _, err := transport.Get(context.Background(), "config", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Get took %s, want under 1s", elapsed)
	}
}

func TestHTTPTransportPrefersLastSuccessfulMirror(t *testing.T) {
	var loserCalls atomic.Int32
	loser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loserCalls.Add(1)
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer loser.Close()
	winner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer winner.Close()

	transport := NewHTTPTransport(http.DefaultClient, []string{loser.URL, winner.URL})
	if _, err := transport.Get(context.Background(), "config", nil); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	firstLoserCalls := loserCalls.Load()
	if _, err := transport.Get(context.Background(), "config", nil); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got := loserCalls.Load(); got != firstLoserCalls {
		t.Fatalf("loser calls = %d, want unchanged at %d", got, firstLoserCalls)
	}
}

func TestHTTPTransportPreservesAccessDeniedWhenAllMirrorsDeny(t *testing.T) {
	denied := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "denied", http.StatusForbidden)
		}))
	}
	first, second := denied(), denied()
	defer first.Close()
	defer second.Close()

	transport := NewHTTPTransport(http.DefaultClient, []string{first.URL, second.URL})
	_, err := transport.Get(context.Background(), "config", nil)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("error = %v, want ErrAccessDenied", err)
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
