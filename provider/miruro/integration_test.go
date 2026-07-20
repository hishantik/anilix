package miruro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hishantik/anilix/source"
)

func TestIntegrationCatalogAndPlayableSource(t *testing.T) {
	if os.Getenv("MIRURO_INTEGRATION") == "" {
		t.Skip("set MIRURO_INTEGRATION=1 to contact Miruro")
	}
	profile := filepath.Join(t.TempDir(), "browser-profile")
	transport, err := NewDefaultTransport(profile)
	if err != nil {
		t.Fatalf("NewDefaultTransport: %v", err)
	}
	defer transport.Close()
	client := NewClient(transport)
	provider := NewProvider(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	catalogStarted := time.Now()
	catalog, err := client.Catalog(ctx, 20)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	candidates := catalog.Candidates("sub", "1")
	if len(candidates) == 0 {
		t.Fatal("Naruto episode 1 has no Miruro candidates")
	}
	t.Logf("catalog resolved in %s", time.Since(catalogStarted).Round(time.Millisecond))

	var lastErr error
	lazyCandidates, err := provider.CandidateStreams(&source.Episode{Number: 1, Anime: &source.Anime{AniListID: 20}})
	if err != nil {
		t.Fatalf("CandidateStreams: %v", err)
	}
	for _, candidate := range lazyCandidates {
		started := time.Now()
		streams, err := candidate.Resolve()
		if err != nil {
			lastErr = err
			continue
		}
		if len(streams) > 0 {
			t.Logf("resolved %d stream(s) through %s in %s", len(streams), candidate.Provider, time.Since(started).Round(time.Millisecond))
			return
		}
	}
	t.Fatalf("no candidate produced a playable stream: %v", lastErr)
}
