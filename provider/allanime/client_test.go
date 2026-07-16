package Allanime

import (
	"net/url"
	"testing"
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

	requestURL, err := buildEpisodeRequestURL(baseURL, "show/id", "sub", "1.5", queryHash)
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
	if parsed.Query().Get("extensions") != `{"persistedQuery":{"sha256Hash":"query-hash","version":1}}` {
		t.Fatalf("unexpected extensions: %s", parsed.Query().Get("extensions"))
	}
}
