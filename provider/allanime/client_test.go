package Allanime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAllAnimeUserAgentMatchesCurrentReference(t *testing.T) {
	const want = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0"
	if allAnimeUserAgent != want {
		t.Fatalf("allAnimeUserAgent = %q, want %q", allAnimeUserAgent, want)
	}
}

func TestBuildEpisodeRequestURLUsesEncodedQueryValues(t *testing.T) {
	const baseURL = "https://api.allanime.day/api"
	const queryHash = "query-hash"

	requestURL, err := buildEpisodeRequestURL(baseURL, "show/id", "sub", "1.5", queryHash, "token-value")
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(requestURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("variables") != `{"episodeString":"1.5","showId":"show/id","translationType":"sub"}` {
		t.Fatalf("unexpected variables: %s", parsed.Query().Get("variables"))
	}
	if parsed.Query().Get("extensions") != `{"aaReq":"token-value","persistedQuery":{"sha256Hash":"query-hash","version":1}}` {
		t.Fatalf("unexpected extensions: %s", parsed.Query().Get("extensions"))
	}
}

func TestPersistedEpisodeRequestIncludesCryptoContract(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 7, 0, 0, time.UTC)
	material := cryptoMaterial{epoch: 4130, buildID: "44", key: make([]byte, 32)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("method = %s", req.Method)
		}
		if req.Header.Get("x-build-id") != "44" {
			t.Errorf("x-build-id = %q", req.Header.Get("x-build-id"))
		}
		if req.Header.Get("Referer") != "https://youtu-chan.com" || req.Header.Get("Origin") != "https://youtu-chan.com" {
			t.Errorf("unexpected origin headers: referer=%q origin=%q", req.Header.Get("Referer"), req.Header.Get("Origin"))
		}
		var extensions struct {
			AAReq string `json:"aaReq"`
			Query struct {
				Hash string `json:"sha256Hash"`
			} `json:"persistedQuery"`
		}
		if err := json.Unmarshal([]byte(req.URL.Query().Get("extensions")), &extensions); err != nil {
			t.Errorf("decode extensions: %v", err)
		}
		if extensions.AAReq == "" || extensions.Query.Hash != "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec" {
			t.Errorf("unexpected extensions: %+v", extensions)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"episode":{"sourceUrls":[]}}}`))
	}))
	defer server.Close()

	client := &AllanimeClient{
		http:    server.Client(),
		baseURL: server.URL,
		bootstrap: &cryptoBootstrap{
			now:       func() time.Time { return now },
			cached:    material,
			expiresAt: now.Add(time.Hour),
		},
		now: func() time.Time { return now },
	}
	body, used, err := client.doPersistedEpisode(context.Background(), "show/id", "1.5", "sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || used.buildID != "44" {
		t.Fatalf("body=%q material=%+v", body, used)
	}
}

func TestParseEpisodeSourcesResponseReturnsGraphQLError(t *testing.T) {
	_, err := parseEpisodeSourcesResponse([]byte(`{"errors":[{"message":"AA_CRYPTO_STALE"}],"data":{"episode":null}}`), cryptoMaterial{})
	if err == nil || err.Error() != "episode GraphQL error: AA_CRYPTO_STALE" {
		t.Fatalf("err = %v", err)
	}
}
