package anilist

import (
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	cases := [][2]string{
		{"ONE PIECE", "onepiece"},
		{"One Piece!", "onepiece"},
		{"Frieren: Beyond Journey’s End", "frierenbeyondjourneysend"},
		{"Frieren: Beyond Journey's End", "frierenbeyondjourneysend"},
		{"Chainsaw Man – The Movie: Reze Arc", "chainsawmanthemovierezearc"},
		{"Chainsaw Man the Movie: Reze Arc", "chainsawmanthemovierezearc"},
		{"攻殻機動隊", "攻殻機動隊"},
	}
	for _, c := range cases {
		if got := normalizeTitle(c[0]); got != c[1] {
			t.Errorf("normalizeTitle(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestTitleMatches(t *testing.T) {
	media := &MediaData{
		ID: 21,
		Title: Title{
			English: "ONE PIECE",
			Romaji:  "ONE PIECE",
		},
		Synonyms: []string{"One Piece: The Movie"},
	}

	if !titleMatches("one piece", media) {
		t.Error("case-insensitive match failed")
	}
	if !titleMatches("One Piece!", media) {
		t.Error("punctuation-insensitive match failed")
	}
	if !titleMatches("One Piece: The Movie", media) {
		t.Error("synonym match failed")
	}
	if titleMatches("One Piece Movie 1", media) {
		t.Error("different title must not match")
	}
	if titleMatches("", media) {
		t.Error("empty query must not match")
	}
}
