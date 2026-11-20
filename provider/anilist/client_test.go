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

func TestMatchUniqueMediaSubset_shortenedTitle(t *testing.T) {
	// hianime shortens the title; the query words are a subset of exactly
	// one candidate, so that candidate is accepted.
	candidates := []*MediaData{
		{ID: 1, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba"}},
		{ID: 2, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba Entertainment District Arc"}},
	}
	got := matchUniqueMediaSubset(candidates, "Demon Slayer: Entertainment District Arc")
	if got == nil || got.ID != 2 {
		t.Fatalf("matchUniqueMediaSubset() = %+v, want the arc (id 2)", got)
	}
}

func TestMatchUniqueMediaSubset_ambiguous(t *testing.T) {
	// Both films contain the query, so a part-less query must attach
	// neither.
	candidates := []*MediaData{
		{ID: 1, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba - Infinity Castle - Part 1"}},
		{ID: 2, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba Infinity Castle Part 2"}},
	}
	if got := matchUniqueMediaSubset(candidates, "Demon Slayer: Kimetsu no Yaiba Infinity Castle"); got != nil {
		t.Errorf("matchUniqueMediaSubset() = %+v, want nil (ambiguous)", got)
	}
}

func TestMatchUniqueMediaSubset_romanNumeral(t *testing.T) {
	candidates := []*MediaData{
		{ID: 1, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba - Infinity Castle - Part 1"}},
		{ID: 2, Title: Title{English: "Demon Slayer: Kimetsu no Yaiba Infinity Castle Part 2"}},
	}
	got := matchUniqueMediaSubset(candidates, "Demon Slayer: Kimetsu no Yaiba Infinity Castle II")
	if got == nil || got.ID != 2 {
		t.Fatalf("matchUniqueMediaSubset() = %+v, want the second film (id 2)", got)
	}
}

func TestMatchUniqueMediaSubset_shortQuery(t *testing.T) {
	// Two-word queries do not get the fallback: "One Piece" must not
	// attach "One Piece Film: Red".
	candidates := []*MediaData{
		{ID: 1, Title: Title{English: "One Piece Film: Red"}},
	}
	if got := matchUniqueMediaSubset(candidates, "One Piece"); got != nil {
		t.Errorf("matchUniqueMediaSubset() = %+v, want nil (query too short)", got)
	}
}

func TestTitleTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Sibling's Bond", []string{"siblings", "bond"}},
		{"Sibling`s Bond", []string{"siblings", "bond"}},
		{"Infinity Castle II", []string{"infinity", "castle", "2"}},
		{"Frieren: Beyond Journey’s End", []string{"frieren", "beyond", "journeys", "end"}},
	}
	for _, c := range cases {
		got := titleTokens(c.in)
		if len(got) != len(c.want) {
			t.Errorf("titleTokens(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("titleTokens(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}
