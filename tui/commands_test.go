package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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

// TestSearchMetadataCompleteLive covers the shortened-title regression:
// hianime lists "Demon Slayer: Entertainment District Arc" while Kitsu and
// AniList carry the full "Demon Slayer: Kimetsu no Yaiba - ..." title. Every
// search result must end up with a metadata panel; entries no metadata
// source knows at all get the provider's own placeholder.
func TestSearchMetadataCompleteLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := hianime.NewProvider()
	results, err := hianimeSearchResults(p, "demon slayer")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}

	ac := anilist.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resolveAnimeIDs(ctx, ac, results)

	kc := kitsu.NewClient()
	titles := make([]string, 0, len(results))
	for _, r := range results {
		titles = append(titles, r.Name)
	}
	matches, err := kc.ResolveByTitles(ctx, titles)
	if err != nil {
		t.Fatalf("kitsu resolve failed: %v", err)
	}

	m := &SearchModel{
		anilistClient:   ac,
		jikanClient:     jikan.NewClient("https://api.jikan.moe/v4"),
		hianimeProvider: p,
		metadataCache:   make(map[int]*MetadataPanel),
		kitsuAnimeCache: make(map[int]*kitsu.Anime),
	}
	for i, match := range matches {
		if match.Anime == nil || i >= len(results) {
			continue
		}
		m.kitsuAnimeCache[i] = match.Anime
		if results[i].AniListID == 0 {
			results[i].AniListID = match.Anime.AniListID
		}
		if results[i].MALID == 0 {
			results[i].MALID = match.Anime.MALID
		}
	}

	// The AniList tier reports its progress and hands the rest of the work
	// to the tail command. The runtime auto-executes commands that come
	// back as messages; the test does the same.
	raw := m.fetchMetadataBatch(results)()
	for {
		cmd, ok := raw.(tea.Cmd)
		if !ok {
			break
		}
		raw = cmd()
	}
	if batch, ok := raw.(tea.BatchMsg); ok {
		// The progress command lands first; the tail carries the panels.
		for _, cmd := range batch {
			if loaded, ok := cmd().(MetadataBatchLoadedMsg); ok {
				raw = loaded
				break
			}
		}
	}
	msg, ok := raw.(MetadataBatchLoadedMsg)
	if !ok {
		t.Fatalf("fetchMetadataBatch returned %T, want MetadataBatchLoadedMsg", raw)
	}
	// The show pages are the provisional follow-up pass: run it exactly as
	// the Update loop does and merge what it brought.
	if msg.FollowUp != nil {
		placeholderMsg, ok := msg.FollowUp().(MetadataBatchLoadedMsg)
		if !ok {
			t.Fatal("the placeholder pass returned an unexpected message")
		}
		for i, panel := range placeholderMsg.MetadataMap {
			if _, ok := msg.MetadataMap[i]; !ok {
				msg.MetadataMap[i] = panel
			}
		}
	}

	real := 0
	for i, r := range results {
		panel := msg.MetadataMap[i]
		if panel == nil {
			t.Errorf("result %d (%q) got no metadata panel", i, r.Name)
			continue
		}
		if panel.Title == "" {
			t.Errorf("result %d (%q) panel has no title", i, r.Name)
		}
		if panel.Source != "Hianime" {
			real++
		} else if panel.Synopsis == "" {
			// The placeholder pass reads the show page, so even an entry no
			// metadata API knows has a synopsis, genres and status.
			t.Errorf("result %d (%q) fell back to the bare search card", i, r.Name)
		}
	}
	t.Logf("%d/%d results with real metadata", real, len(results))

	// The shortened titles are the regression: they must not be left to
	// the provider placeholder while Kitsu knows the entry.
	shortened := []string{
		"Demon Slayer: Entertainment District Arc",
		"Demon Slayer Movie: Mugen Train",
		"Demon Slayer: Mt. Natagumo Arc",
		"Demon Slayer: The Hashira Meeting Arc",
		"Demon Slayer: Sibling's Bond",
	}
	for _, want := range shortened {
		found := false
		for i, r := range results {
			if r.Name != want {
				continue
			}
			found = true
			if panel := msg.MetadataMap[i]; panel == nil || panel.Source == "Hianime" {
				t.Errorf("%q fell back to the provider placeholder", want)
			}
		}
		if !found {
			t.Logf("hianime no longer returns %q", want)
		}
	}
}
