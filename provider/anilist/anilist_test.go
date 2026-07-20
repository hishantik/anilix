package anilist

import "testing"

func TestMapToAnimePreservesMALID(t *testing.T) {
	provider := NewAniListProvider()
	anime := provider.mapToAnime(&MediaData{ID: 21, IDMal: 21, Title: Title{English: "One Piece"}})
	if anime.MALID != 21 {
		t.Fatalf("MALID = %d, want 21", anime.MALID)
	}
}
