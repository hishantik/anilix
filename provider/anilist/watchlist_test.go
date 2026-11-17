package anilist

import (
	"encoding/json"
	"testing"
)

func TestWatchlistOfCollection(t *testing.T) {
	// The answer of the collection query, shaped the way AniList sends it:
	// the user's lists, each with the entries of one status group.
	response := `{
		"data": {
			"MediaListCollection": {
				"lists": [
					{
						"name": "Watching",
						"entries": [
							{
								"status": "CURRENT",
								"progress": 5,
								"media": {
									"id": 21,
									"title": {"userPreferred": "Frieren", "english": "Frieren: Beyond Journey's End"},
									"coverImage": {"large": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/default.jpg"},
									"episodes": 28,
									"format": "TV",
									"seasonYear": 2023,
									"averageScore": 90
								}
							}
						]
					},
					{
						"name": "Planning",
						"entries": [
							{
								"status": "PLANNING",
								"progress": 0,
								"media": {
									"id": 1,
									"title": {"userPreferred": "One Piece"},
									"format": "TV",
									"seasonYear": 1999
								}
							},
							{
								"status": "PLANNING",
								"progress": 0,
								"media": {}
							}
						]
					}
				]
			}
		}
	}`

	var result MediaListCollectionResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("the answer did not decode: %v", err)
	}

	entries := watchlistOfCollection(result.Data.MediaListCollection)
	if len(entries) != 2 {
		t.Fatalf("the collection flattened to %d entries, want 2 (a media-less entry is dropped)", len(entries))
	}

	first := entries[0]
	if first.Title != "Frieren" || first.Status != "CURRENT" || first.Progress != 5 {
		t.Errorf("the first entry lost its fields: %+v", first)
	}
	if first.Total != 28 || first.Year != 2023 || first.Format != "TV" {
		t.Errorf("the first entry lost its facts: %+v", first)
	}
	if first.Score < 8.9 || first.Score > 9.1 {
		t.Errorf("the average score is %.2f, want 9.0", first.Score)
	}
	if first.Cover == "" {
		t.Error("the first entry lost its cover")
	}

	second := entries[1]
	if second.Title != "One Piece" || second.Status != "PLANNING" {
		t.Errorf("the second entry lost its fields: %+v", second)
	}
	if second.Score != 0 || second.Cover != "" {
		t.Errorf("an entry without a score or cover invented one: %+v", second)
	}
}
