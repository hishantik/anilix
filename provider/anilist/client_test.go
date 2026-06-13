package anilist

import (
	"testing"
)

// TestParseRecommendationsResponse verifies the response parser understands
// AniList's RecommendationConnection shape:
//   Media.recommendations.nodes[].mediaRecommendation
// If this test fails, the recommendations UI shows "No recommendations available."
func TestParseRecommendationsResponse(t *testing.T) {
	raw := []byte(`{
		"Media": {
			"recommendations": {
				"nodes": [
					{
						"id": 1,
						"rating": 45,
						"mediaRecommendation": {
							"id": 21,
							"title": { "romaji": "ONE PIECE", "english": "ONE PIECE" },
							"coverImage": { "extraLarge": "http://x/one.jpg" },
							"type": "ANIME",
							"format": "TV",
							"episodes": 1100,
							"genres": ["Action"],
							"averageScore": 90,
							"popularity": 500000
						}
					},
					{
						"id": 2,
						"rating": 30,
						"mediaRecommendation": {
							"id": 1535,
							"title": { "romaji": "DEATH NOTE", "english": "Death Note" },
							"coverImage": { "extraLarge": "http://x/dn.jpg" },
							"type": "ANIME",
							"format": "TV",
							"episodes": 37,
							"genres": ["Thriller"],
							"averageScore": 84,
							"popularity": 300000
						}
					}
				]
			}
		}
	}`)

	results := parseRecommendationsResponse(raw, 12)

	if len(results) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(results))
	}
	if results[0].ID != 21 {
		t.Errorf("expected first ID=21 (mediaRecommendation), got %d", results[0].ID)
	}
	if results[0].Title.Romaji != "ONE PIECE" {
		t.Errorf("expected first title=ONE PIECE, got %q", results[0].Title.Romaji)
	}
	if results[1].ID != 1535 {
		t.Errorf("expected second ID=1535, got %d", results[1].ID)
	}
}

// TestParseRecommendationsResponse_Empty verifies the parser returns an empty
// slice (not nil/wrapped error) when AniList sends no recommendations.
func TestParseRecommendationsResponse_Empty(t *testing.T) {
	raw := []byte(`{ "Media": { "recommendations": { "nodes": [] } } }`)
	results := parseRecommendationsResponse(raw, 10)
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d items", len(results))
	}
}

// TestParseRecommendationsResponse_NoMedia checks that a malformed body (no
// Media key) does not panic and that the parser handles it gracefully. A
// nil slice is acceptable here; the downstream RecommendationsLoadedMsg marshals
// this without issue.
func TestParseRecommendationsResponse_NoMedia(t *testing.T) {
	raw := []byte(`{ "errors": [{ "message": "Not Found." }] }`)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parseRecommendationsResponse panicked on malformed body: %v", r)
		}
	}()
	results := parseRecommendationsResponse(raw, 10)
	if len(results) != 0 {
		t.Errorf("expected empty slice on errors-only body, got %d items", len(results))
	}
}
