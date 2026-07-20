package miruro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	catalog, err := client.Catalog(ctx, 20)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	candidates := catalog.Candidates("sub", "1")
	if len(candidates) == 0 {
		t.Fatal("Naruto episode 1 has no Miruro candidates")
	}

	var lastErr error
	for _, candidate := range candidates {
		streams, err := client.Sources(ctx, candidate)
		if err != nil {
			lastErr = err
			continue
		}
		if len(streams) > 0 {
			t.Logf("resolved %d stream(s) through %s", len(streams), candidate.Provider)
			return
		}
	}
	t.Fatalf("no candidate produced a playable stream: %v", lastErr)
}
