package kitsu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchFirst_exactMatch(t *testing.T) {
	// Kitsu response for "one piece" (One Piece id=12, MAL=21, AniList=21)
	const body = `{
  "data": [
    {
      "id": "12",
      "type": "anime",
      "attributes": {
        "canonicalTitle": "One Piece",
        "titles": {"en": "One Piece", "en_jp": "One Piece", "ja_jp": "ONE PIECE"},
        "synopsis": "Gol D. Roger...",
        "posterImage": {"tiny": "http://x/t.jpg", "large": "http://x/l.jpg"},
        "showType": "TV",
        "status": "current",
        "startDate": "1999-10-20",
        "averageRating": "88.12",
        "nsfw": false
      },
      "relationships": {
        "mappings": {
          "data": [
            {"type": "mappings", "id": "1175"},
            {"type": "mappings", "id": "254544"}
          ]
        }
      }
    }
  ],
  "included": [
    {"id": "1175", "type": "mappings", "attributes": {"externalSite": "myanimelist/anime", "externalId": "21"}},
    {"id": "254544", "type": "mappings", "attributes": {"externalSite": "anilist/anime", "externalId": "21"}}
  ],
  "meta": {"count": 1}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected Accept: application/vnd.api+json, got %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL + "/")

	ctx := context.Background()
	anime, err := client.SearchFirst(ctx, "One Piece")
	if err != nil {
		t.Fatalf("SearchFirst() error = %v", err)
	}
	if anime == nil {
		t.Fatal("SearchFirst() returned nil")
	}
	if anime.KitsuID != "12" {
		t.Errorf("KitsuID = %q, want %q", anime.KitsuID, "12")
	}
	if anime.Canonical != "One Piece" {
		t.Errorf("Canonical = %q, want %q", anime.Canonical, "One Piece")
	}
	if anime.MALID != 21 {
		t.Errorf("MALID = %d, want %d", anime.MALID, 21)
	}
	if anime.AniListID != 21 {
		t.Errorf("AniListID = %d, want %d", anime.AniListID, 21)
	}
	if anime.ShowType != "TV" {
		t.Errorf("ShowType = %q, want %q", anime.ShowType, "TV")
	}
	if anime.Status != "current" {
		t.Errorf("Status = %q, want %q", anime.Status, "current")
	}
}

func TestSearchFirst_noMatch(t *testing.T) {
	// Response with a different title that shouldn't match
	const body = `{
  "data": [
    {
      "id": "99",
      "type": "anime",
      "attributes": {
        "canonicalTitle": "Death Note",
        "titles": {"en": "Death Note"},
        "showType": "TV",
        "status": "finished"
      },
      "relationships": {
        "mappings": {
          "data": [
            {"type": "mappings", "id": "x1"}
          ]
        }
      }
    }
  ],
  "included": [
    {"id": "x1", "type": "mappings", "attributes": {"externalSite": "myanimelist/anime", "externalId": "1535"}}
  ],
  "meta": {"count": 1}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL + "/")

	ctx := context.Background()
	anime, err := client.SearchFirst(ctx, "One Piece")
	if err != nil {
		t.Fatalf("SearchFirst() error = %v", err)
	}
	// "One Piece" query should not match "Death Note"
	if anime != nil {
		t.Errorf("SearchFirst() returned %v, want nil (no match)", anime.Canonical)
	}
}

func TestResolveByTitles(t *testing.T) {
	// Kitsu returns "Attack on Titan" for both exact forms.
	const body = `{
  "data": [
    {
      "id": "1",
      "type": "anime",
      "attributes": {
        "canonicalTitle": "Attack on Titan",
        "titles": {"en": "Attack on Titan"},
        "showType": "TV",
        "status": "finished"
      },
      "relationships": {
        "mappings": {
          "data": [
            {"type": "mappings", "id": "m1"}
          ]
        }
      }
    }
  ],
  "included": [
    {"id": "m1", "type": "mappings", "attributes": {"externalSite": "myanimelist/anime", "externalId": "16498"}}
  ],
  "meta": {"count": 1}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL + "/")

	ctx := context.Background()
	titles := []string{"Attack on Titan", "Steins;Gate"}
	matches, err := client.ResolveByTitles(ctx, titles)
	if err != nil {
		t.Fatalf("ResolveByTitles() error = %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("len(matches) = %d, want 2", len(matches))
	}
	if matches[0].Anime == nil {
		t.Errorf("matches[0].Anime = nil, want entry")
	}
	if matches[0].Anime != nil && matches[0].Anime.Canonical != "Attack on Titan" {
		t.Errorf("matches[0].Anime.Canonical = %q, want %q", matches[0].Anime.Canonical, "Attack on Titan")
	}
	if matches[1].Anime != nil {
		t.Errorf("matches[1].Anime = %v, want nil (no match)", matches[1].Anime.Canonical)
	}
}

func TestFlattenAnime_imagePick(t *testing.T) {
	// Test that pickImage prefers Original > Large > Medium > Small > Tiny.
	res := &AnimeResource{
		ID:   "5",
		Type: "anime",
		Attributes: &AnimeAttributes{
			CanonicalTitle: "Test",
			PosterImage:    ImageLinks{Tiny: "t.jpg", Small: "s.jpg", Medium: "m.jpg", Large: "l.jpg", Original: "o.jpg"},
			CoverImage:     ImageLinks{Tiny: "ct.jpg", Small: "cs.jpg"},
		},
	}
	anime := flattenAnime(res)
	if anime.Poster != "o.jpg" {
		t.Errorf("Poster = %q, want o.jpg", anime.Poster)
	}
	// CoverImage has no Large/Medium/Original, should fall back to Small.
	if anime.Cover != "cs.jpg" {
		t.Errorf("Cover = %q, want cs.jpg", anime.Cover)
	}
}

func TestFlattenAnime_nullFields(t *testing.T) {
	// episodeCount nil should produce 0, not crash.
	res := &AnimeResource{
		ID:   "6",
		Type: "anime",
		Attributes: &AnimeAttributes{
			CanonicalTitle: "Partial",
			ShowType:      "OVA",
		},
		Relationships: nil, // no mappings
	}
	anime := flattenAnime(res)
	if anime.EpisodeCount != 0 {
		t.Errorf("EpisodeCount = %d, want 0", anime.EpisodeCount)
	}
	if anime.EpisodeLength != 0 {
		t.Errorf("EpisodeLength = %d, want 0", anime.EpisodeLength)
	}
	// No mappings → both IDs stay zero.
	if anime.MALID != 0 {
		t.Errorf("MALID = %d, want 0", anime.MALID)
	}
	if anime.AniListID != 0 {
		t.Errorf("AniListID = %d, want 0", anime.AniListID)
	}
}

func TestAnime_matchesTitle(t *testing.T) {
	// Same title, different casing/punctuation should match.
	anime := &Anime{
		Canonical: "One Piece",
		Titles:    map[string]string{"en_jp": "ONE PIECE"},
	}
	q := normalizeTitle("one piece")
	if !anime.matchesTitle(q) {
		t.Error("expected 'one piece' to match 'One Piece'")
	}

	q2 := normalizeTitle("one piece!")
	if !anime.matchesTitle(q2) {
		t.Error("expected 'one piece!' to match 'One Piece'")
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"One Piece", "onepiece"},
		{"ONE PIECE!", "onepiece"},
		{"Steins;Gate", "steinsgate"},
		{"Jujutsu Kaisen", "jujutsukaisen"},
		{"Don't", "dont"},
		{"Journey's End", "journeysend"},
	}
	for _, c := range cases {
		got := normalizeTitle(c.input)
		if got != c.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestAtoi(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"21", 21},
		{"1535", 1535},
		{"", 0},
		{"abc", 0},
		{"42abc", 0},
	}
	for _, c := range cases {
		got := atoi(c.s)
		if got != c.want {
			t.Errorf("atoi(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestParseRating(t *testing.T) {
	cases := []struct {
		s    string
		want float64
		ok   bool
	}{
		{"88.12", 88.12, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, err := parseRating(c.s)
		if c.ok && err != nil {
			t.Errorf("parseRating(%q) unexpected error: %v", c.s, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parseRating(%q) expected error, got %f", c.s, got)
		}
		if c.ok && got != c.want {
			t.Errorf("parseRating(%q) = %f, want %f", c.s, got, c.want)
		}
	}
}

// roundTripperFunc adapter to inject test responses in client tests
// that need a real HTTP server.
func testServer(t *testing.T, path, query string, responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("path = %q, want %q", r.URL.Path, path)
		}
		if query != "" && r.URL.RawQuery != query {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, query)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write([]byte(responseBody))
	}))
}

func TestMappingParsing(t *testing.T) {
	// Verify all supported externalSite values are parsed.
	const body = `{
  "data": [
    {
      "id": "100",
      "type": "anime",
      "attributes": {"canonicalTitle": "Multi-ID", "showType": "TV", "status": "current"},
      "relationships": {
        "mappings": {
          "data": [
            {"type": "mappings", "id": "m1"},
            {"type": "mappings", "id": "m2"},
            {"type": "mappings", "id": "m3"},
            {"type": "mappings", "id": "m4"},
            {"type": "mappings", "id": "m5"}
          ]
        }
      }
    }
  ],
  "included": [
    {"id": "m1", "type": "mappings", "attributes": {"externalSite": "myanimelist/anime", "externalId": "999"}},
    {"id": "m2", "type": "mappings", "attributes": {"externalSite": "anilist/anime", "externalId": "888"}},
    {"id": "m3", "type": "mappings", "attributes": {"externalSite": "anidb", "externalId": "777"}},
    {"id": "m4", "type": "mappings", "attributes": {"externalSite": "thetvdb", "externalId": "666"}},
    {"id": "m5", "type": "mappings", "attributes": {"externalSite": "hulu", "externalId": "555"}}
  ],
  "meta": {"count": 1}
}`

	var payload AnimeResponse
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}

	mappings := make(map[string]*MappingAttributes, len(payload.Included))
	for _, inc := range payload.Included {
		if inc.Type == "mappings" && inc.Attributes != nil {
			mappings[inc.ID] = inc.Attributes
		}
	}

	if got := mappings["m1"].ExternalSite; got != "myanimelist/anime" {
		t.Errorf("m1 site = %q", got)
	}
	if got := mappings["m2"].ExternalSite; got != "anilist/anime" {
		t.Errorf("m2 site = %q", got)
	}

	// Only MAL and AniList IDs are extracted by applyMappings.
	anime := &Anime{}
	applyMappings(anime, payload.Data[0], mappings)
	if anime.MALID != 999 {
		t.Errorf("MALID = %d, want 999", anime.MALID)
	}
	if anime.AniListID != 888 {
		t.Errorf("AniListID = %d, want 888", anime.AniListID)
	}
}
