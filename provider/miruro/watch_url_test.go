package miruro

import "testing"

func TestWatchURLUsesAniListIDAndSlug(t *testing.T) {
	got := WatchURL(21, "ONE PIECE")
	want := "https://www.miruro.tv/watch/21/one-piece"
	if got != want {
		t.Fatalf("WatchURL = %q, want %q", got, want)
	}
}

func TestWatchURLHandlesNonASCIIAndEmptyTitle(t *testing.T) {
	if got := WatchURL(20, "ナルト"); got != "https://www.miruro.tv/watch/20/anime" {
		t.Fatalf("WatchURL = %q", got)
	}
}
