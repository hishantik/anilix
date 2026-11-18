package tui

import (
	"testing"

	"github.com/hishantik/anilix/provider/kitsu"
)

func TestKitsuToPanel(t *testing.T) {
	a := &kitsu.Anime{
		KitsuID:       "176",
		Canonical:     "Spirited Away",
		Titles:        map[string]string{"en": "Spirited Away", "en_jp": "Sen to Chihiro no Kamikakushi", "ja_jp": "千と千尋の神隠し"},
		Synopsis:      "A movie.",
		Poster:        "https://media.kitsu.app/poster.jpg",
		Genres:        []string{"Adventure", "Drama", "Fantasy"},
		EpisodeCount:  1,
		ShowType:      "movie",
		Status:        "finished",
		StartDate:     "2001-07-20",
		AverageRating: 82.73,
		AniListID:     199,
		MALID:         199,
	}

	panel := kitsuToPanel(a)
	if panel == nil {
		t.Fatal("kitsuToPanel() = nil, want panel")
	}
	if panel.Title != "Spirited Away" {
		t.Errorf("Title = %q, want Spirited Away", panel.Title)
	}
	if panel.TitleEnglish != "Spirited Away" {
		t.Errorf("TitleEnglish = %q", panel.TitleEnglish)
	}
	if panel.TitleNative != "千と千尋の神隠し" {
		t.Errorf("TitleNative = %q", panel.TitleNative)
	}
	if panel.Cover != "https://media.kitsu.app/poster.jpg" {
		t.Errorf("Cover = %q", panel.Cover)
	}
	if panel.Type != "MOVIE" {
		t.Errorf("Type = %q, want MOVIE", panel.Type)
	}
	if panel.Status != "FINISHED" {
		t.Errorf("Status = %q, want FINISHED", panel.Status)
	}
	if panel.Episodes != 1 {
		t.Errorf("Episodes = %d, want 1", panel.Episodes)
	}
	if panel.Score != 8.273 {
		t.Errorf("Score = %f, want 8.273", panel.Score)
	}
	if panel.Year != 2001 {
		t.Errorf("Year = %d, want 2001", panel.Year)
	}
	if len(panel.Genres) != 3 || panel.Genres[0] != "Adventure" {
		t.Errorf("Genres = %v", panel.Genres)
	}
	if panel.Source != "Kitsu" {
		t.Errorf("Source = %q, want Kitsu", panel.Source)
	}
}

func TestKitsuToPanel_nil(t *testing.T) {
	if got := kitsuToPanel(nil); got != nil {
		t.Errorf("kitsuToPanel(nil) = %v, want nil", got)
	}
}

func TestKitsuFormat(t *testing.T) {
	cases := map[string]string{
		"tv":      "TV",
		"movie":   "MOVIE",
		"ova":     "OVA",
		"ona":     "ONA",
		"special": "SPECIAL",
		"music":   "MUSIC",
		"":        "",
	}
	for in, want := range cases {
		if got := kitsuFormat(in); got != want {
			t.Errorf("kitsuFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKitsuStatus(t *testing.T) {
	cases := map[string]string{
		"current":  "RELEASING",
		"finished": "FINISHED",
		"tba":      "TBA",
		"upcoming": "NOT_YET_RELEASED",
		"":         "",
	}
	for in, want := range cases {
		if got := kitsuStatus(in); got != want {
			t.Errorf("kitsuStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
