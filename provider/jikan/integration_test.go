package jikan

import (
	"testing"
)

func TestIntegration_JikanSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	jp := NewJikanProvider()
	results, err := jp.Search("Cowboy Bebop")
	if err != nil {
		t.Fatalf("Jikan search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	anime := results[0]
	if anime.MALID == 0 {
		t.Error("expected MAL ID to be set from Jikan")
	}

	t.Logf("Found anime: %s (MAL ID: %d)", anime.Name, anime.MALID)
}
