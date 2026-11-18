package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/hianime"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/provider/kitsu"
)

// TestSearchFlowLive exercises the search path end to end: hianime results
// first, then the background ID resolution that feeds the metadata batch.
func TestSearchFlowLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := hianime.NewProvider()
	ac := anilist.NewClient()
	jc := jikan.NewClient("https://api.jikan.moe/v4")
	_ = jc

	t0 := time.Now()
	results, err := hianimeSearchResults(p, "one piece")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	searchTime := time.Since(t0)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	t.Logf("search returned %d results in %s", len(results), searchTime.Round(time.Millisecond))

	// The list must be usable before IDs are resolved.
	for _, r := range results {
		if r.Name == "" || r.ShowID == "" {
			t.Fatalf("incomplete result: %+v", r)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resolveAnimeIDs(ctx, ac, results)

	withAniList, withMAL := 0, 0
	for _, r := range results {
		if r.AniListID > 0 {
			withAniList++
		}
		if r.MALID > 0 {
			withMAL++
		}
	}
	t.Logf("resolved: %d/%d with AniList ID, %d/%d with MAL ID", withAniList, len(results), withMAL, len(results))
	if withAniList == 0 {
		t.Error("no result got an AniList ID")
	}

	// The metadata batch must return panels for the resolved IDs.
	ids := make([]int, 0, withAniList)
	for _, r := range results {
		if r.AniListID > 0 {
			ids = append(ids, r.AniListID)
		}
	}
	batch, err := ac.GetAnimeBatch(ctx, ids)
	if err != nil {
		t.Fatalf("metadata batch failed: %v", err)
	}
	if len(batch) == 0 {
		t.Error("metadata batch returned nothing")
	}
	t.Logf("metadata batch returned %d entries", len(batch))
}

// TestMovieMetadataLive walks the metadata path for a movie: hianime
// search, Kitsu title resolution, then the Kitsu panel that fills the
// metadata view when AniList has nothing to say about the entry.
func TestMovieMetadataLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := hianime.NewProvider()
	results, err := hianimeSearchResults(p, "spirited away")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}

	m := &SearchModel{kitsuClient: kitsu.NewClient()}
	msg, ok := m.resolveIDsViaKitsu(results)().(MALIDsResolvedMsg)
	if !ok {
		t.Fatal("resolveIDsViaKitsu returned an unexpected message")
	}
	if len(msg.Animes) == 0 {
		t.Fatal("Kitsu resolved no entries for the movie search")
	}

	// Find the movie itself among the resolved entries: a movie result
	// must come with a complete panel even without an AniList id.
	var panel *MetadataPanel
	for _, anime := range msg.Animes {
		if !strings.EqualFold(anime.ShowType, "movie") {
			continue
		}
		panel = kitsuToPanel(anime)
		t.Logf("movie: %q type=%s status=%s year=%d genres=%v anilist=%d mal=%d",
			anime.Canonical, panel.Type, panel.Status, panel.Year, panel.Genres, anime.AniListID, anime.MALID)
		break
	}
	if panel == nil {
		t.Fatal("no movie among the Kitsu resolutions")
	}
	if panel.Title == "" || panel.Cover == "" || panel.Synopsis == "" {
		t.Errorf("incomplete movie panel: title=%q cover=%q synopsis=%d chars",
			panel.Title, panel.Cover, len(panel.Synopsis))
	}
	if panel.Type != "MOVIE" {
		t.Errorf("Type = %q, want MOVIE", panel.Type)
	}
	if len(panel.Genres) == 0 {
		t.Error("movie panel has no genres")
	}
}
