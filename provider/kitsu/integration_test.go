package kitsu

import (
	"context"
	"testing"
	"time"
)

// TestIntegrationResolveByTitles hits the live Kitsu API. Skipped in -short
// runs like every other integration test.
func TestIntegrationResolveByTitles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	titles := []string{"One Piece", "Steins;Gate", "Nonexistent Anime XYZ"}
	matches, err := client.ResolveByTitles(ctx, titles)
	if err != nil {
		t.Fatalf("ResolveByTitles() error = %v", err)
	}
	if len(matches) != len(titles) {
		t.Fatalf("len(matches) = %d, want %d", len(matches), len(titles))
	}

	// One Piece: Kitsu id 12, MAL 21, AniList 21.
	if m := matches[0]; m.Anime == nil {
		t.Error("One Piece: Anime = nil, want match")
	} else {
		if m.Anime.AniListID != 21 {
			t.Errorf("One Piece AniListID = %d, want 21", m.Anime.AniListID)
		}
		if m.Anime.MALID != 21 {
			t.Errorf("One Piece MALID = %d, want 21", m.Anime.MALID)
		}
		t.Logf("One Piece: kitsu=%s anilist=%d mal=%d", m.Anime.KitsuID, m.Anime.AniListID, m.Anime.MALID)
	}

	// Steins;Gate: MAL 9253, AniList 9253.
	if m := matches[1]; m.Anime == nil {
		t.Error("Steins;Gate: Anime = nil, want match")
	} else {
		t.Logf("Steins;Gate: kitsu=%s anilist=%d mal=%d", m.Anime.KitsuID, m.Anime.AniListID, m.Anime.MALID)
	}

	// No such anime: no match, no error.
	if m := matches[2]; m.Anime != nil {
		t.Errorf("Nonexistent Anime XYZ matched %q, want nil", m.Anime.Canonical)
	}
}
