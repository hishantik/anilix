package jikan

import (
	"os"
	"testing"

	"github.com/hishantik/anilix/source"
)

var _ source.Source = (*JikanProvider)(nil)

func TestJikanProvider_Name(t *testing.T) {
	jp := NewJikanProvider()

	if jp.Name() != "Jikan" {
		t.Errorf("expected Name() to return 'Jikan', got %s", jp.Name())
	}
}

func TestJikanProvider_ID(t *testing.T) {
	jp := NewJikanProvider()

	if jp.ID() != "jikan" {
		t.Errorf("expected ID() to return 'jikan', got %s", jp.ID())
	}
}

func TestJikanProvider_Search(t *testing.T) {
	if os.Getenv("JIKAN_INTEGRATION") == "" {
		t.Skip("set JIKAN_INTEGRATION=1 to contact Jikan")
	}
	jp := NewJikanProvider()

	results, err := jp.Search("Naruto")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results, got empty slice")
	}

	if results[0].MALID == 0 {
		t.Error("expected MALID to be set")
	}
}
