package miruro

import (
	"context"
	"net/url"
	"testing"
)

type routeTransport struct {
	routes map[string][]byte
	calls  []string
}

func (t *routeTransport) Get(_ context.Context, path string, query url.Values) ([]byte, error) {
	key := path
	if provider := query.Get("provider"); provider != "" {
		key += ":" + provider
	}
	t.calls = append(t.calls, key)
	return t.routes[key], nil
}

func (t *routeTransport) Close() error { return nil }

func TestClientCatalogPreservesOpaqueIDsAndTranslation(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"episodes": []byte(`{
			"providers": {
				"kiwi": {"episodes": {"sub": [{"id":"b3BhcXVlLWtpd2k","number":1,"title":"Start"}]}},
				"bonk": {"episodes": {"sub": [{"id":"opaque-bonk","number":"1"}], "dub": [{"id":"opaque-dub","number":1}]}}
			}
		}`),
	}}
	client := NewClient(transport)

	catalog, err := client.Catalog(context.Background(), 20)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	sub := catalog.Candidates("sub", "1")
	if len(sub) != 2 || sub[0].ID != "opaque-bonk" || sub[1].ID != "b3BhcXVlLWtpd2k" {
		t.Fatalf("sub candidates = %+v", sub)
	}
	dub := catalog.Candidates("dub", "1")
	if len(dub) != 1 || dub[0].ID != "opaque-dub" {
		t.Fatalf("dub candidates = %+v", dub)
	}
}

func TestClientCatalogUsesMiruroProviderOrder(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"config":   []byte(`{"providerOrder":["kiwi","bonk"],"streaming":{"kiwi":{"player":"native","visible":true},"bonk":{"player":"native","visible":true}}}`),
		"episodes": []byte(`{"providers":{"bonk":{"episodes":{"sub":[{"id":"bonk-1","number":1}]}},"kiwi":{"episodes":{"sub":[{"id":"kiwi-1","number":1}]}}}}`),
	}}
	client := NewClient(transport)
	catalog, err := client.Catalog(context.Background(), 20)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	candidates := catalog.Candidates("sub", "1")
	if len(candidates) != 2 || candidates[0].Provider != "kiwi" {
		t.Fatalf("candidates = %+v", candidates)
	}
}

func TestClientSourcesMapsStreamsAndSubtitles(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"sources:kiwi": []byte(`{
			"streams":[{"url":"https://video.example/master.m3u8","quality":"1080p","type":"hls","headers":{"Referer":"https://video.example/"}}],
			"subtitles":[{"file":"https://video.example/en.vtt","label":"English"}]
		}`),
	}}
	client := NewClient(transport)
	candidate := EpisodeCandidate{AniListID: 20, ID: "opaque", Provider: "kiwi", Category: "sub", Number: "1"}

	streams, err := client.Sources(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("streams = %+v", streams)
	}
	stream := streams[0]
	if stream.URL != "https://video.example/master.m3u8" || stream.Quality != "1080p" || stream.Provider != "kiwi" {
		t.Fatalf("stream = %+v", stream)
	}
	if stream.Referer != "https://video.example/" || !stream.NeedsReferrer {
		t.Fatalf("referrer mapping = %+v", stream)
	}
	if len(stream.Subtitles) != 1 || stream.Subtitles[0].Language != "English" {
		t.Fatalf("subtitles = %+v", stream.Subtitles)
	}
}

func TestClientSourcesRejectsNonHTTPMediaURL(t *testing.T) {
	transport := &routeTransport{routes: map[string][]byte{
		"sources:kiwi": []byte(`{"streams":[{"url":"javascript:alert(1)","quality":"auto"}]}`),
	}}
	client := NewClient(transport)
	_, err := client.Sources(context.Background(), EpisodeCandidate{AniListID: 20, ID: "opaque", Provider: "kiwi", Category: "sub"})
	if err == nil {
		t.Fatal("Sources succeeded with an unsafe URL")
	}
}
