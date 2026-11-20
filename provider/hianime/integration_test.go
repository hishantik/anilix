package hianime

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hishantik/anilix/source"
)

// base64Encode is a tiny helper for tests (kept out of the client).
func base64Encode(b []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, 0, (len(b)+2)/3*4)
	for i := 0; i < len(b); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], b[i:])
		out = append(out,
			chars[chunk[0]>>2],
			chars[(chunk[0]&0x03)<<4|chunk[1]>>4],
		)
		if n > 1 {
			out = append(out, chars[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		} else {
			out = append(out, '=')
		}
		if n > 2 {
			out = append(out, chars[chunk[2]&0x3f])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

// Integration tests hit the live hianime.at site; they are skipped in -short
// mode, matching the pattern used by the other providers.
func TestIntegration_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := NewProvider()
	results, err := p.Search("one piece")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("no results")
	}
	for _, r := range results {
		if r.ShowID == "" || r.Name == "" {
			t.Fatalf("incomplete result: %+v", r)
		}
	}
}

func TestIntegration_ShowMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := NewProvider()
	results, err := p.Search("demon slayer")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	meta, err := p.ShowMetadata(ctx, results[0].ShowID, results[0].URL)
	if err != nil {
		t.Fatalf("ShowMetadata failed: %v", err)
	}
	if meta.Synopsis == "" {
		t.Error("the show page came back without a synopsis")
	}
	if len(meta.Genres) == 0 {
		t.Error("the show page came back without genres")
	}
	if meta.Type == "" {
		t.Error("the show page came back without a format")
	}
	t.Logf("%q: type=%s rating=%s status=%s year=%d score=%.2f duration=%dm genres=%v studios=%v episodes=%d synopsis=%d chars",
		results[0].Name, meta.Type, meta.Rating, meta.Status, meta.Year, meta.Score,
		meta.Duration, meta.Genres, meta.Studios, meta.Episodes, len(meta.Synopsis))

	// The second call is served from the provider cache, not the network.
	cached, err := p.ShowMetadata(ctx, results[0].ShowID, results[0].URL)
	if err != nil || cached != meta {
		t.Errorf("the second call was not served from the cache: err=%v same=%v", err, cached == meta)
	}
}

func TestIntegration_EpisodesOf(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := NewProvider()
	anime := &source.Anime{ShowID: "1", Name: "One Piece"}
	eps, err := p.EpisodesOf(anime, 0)
	if err != nil {
		t.Fatalf("EpisodesOf failed: %v", err)
	}
	if len(eps) == 0 {
		t.Fatalf("no episodes")
	}
	if anime.MALID != 21 {
		t.Errorf("MALID not enriched from embed URL: got %d", anime.MALID)
	}
}

func TestIntegration_FullStreamExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := NewProvider()
	// Sprinkle one piece episodes: pick a low episode number for reliability.
	episodeNum := float64(1 + rand.Intn(3))
	anime := &source.Anime{ShowID: "1", Name: "One Piece"}

	streams, err := p.StreamsOf(&source.Episode{Number: episodeNum, Anime: anime})
	if err != nil {
		t.Fatalf("StreamsOf failed: %v", err)
	}
	if len(streams) == 0 {
		t.Fatalf("no streams")
	}
	fmt.Printf("got %d streams (best: %s %s)\n", len(streams), streams[0].Quality, streams[0].URL)
	if streams[0].Referer == "" {
		t.Errorf("referer missing on stream")
	}
	time.Sleep(100 * time.Millisecond)
}
