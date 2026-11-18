package tui

import (
	"testing"

	"github.com/hishantik/anilix/provider/hianime"
	"github.com/hishantik/anilix/provider/kitsu"
	"github.com/hishantik/anilix/source"
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

func TestSearchResultPanel(t *testing.T) {
	r := &source.Anime{
		Name:         "Demon Slayer: Kimetsu no Yaiba Infinity Castle III",
		Cover:        "https://cdn.example/poster.jpg",
		Type:         "MOVIE (1 EPS)",
		EpisodeCount: 1,
		Year:         2027,
	}
	panel := searchResultPanel(r)
	if panel == nil {
		t.Fatal("searchResultPanel() = nil, want panel")
	}
	if panel.Title != r.Name {
		t.Errorf("Title = %q, want %q", panel.Title, r.Name)
	}
	if panel.Cover != r.Cover {
		t.Errorf("Cover = %q, want %q", panel.Cover, r.Cover)
	}
	if panel.Type != "MOVIE" {
		t.Errorf("Type = %q, want MOVIE", panel.Type)
	}
	if panel.Year != 2027 {
		t.Errorf("Year = %d, want 2027", panel.Year)
	}
	if panel.Source != "Hianime" {
		t.Errorf("Source = %q, want Hianime", panel.Source)
	}
}

func TestSearchResultPanel_nil(t *testing.T) {
	if got := searchResultPanel(nil); got != nil {
		t.Errorf("searchResultPanel(nil) = %v, want nil", got)
	}
	if got := searchResultPanel(&source.Anime{}); got != nil {
		t.Errorf("searchResultPanel(empty) = %v, want nil", got)
	}
}

func TestHasRealMetadata(t *testing.T) {
	m := &SearchModel{metadataCache: map[int]*MetadataPanel{
		0: nil,
		1: {Source: "Hianime"},
		2: {Source: "Kitsu"},
	}}
	cases := map[int]bool{0: false, 1: false, 2: true, 3: false}
	for idx, want := range cases {
		if got := m.hasRealMetadata(idx); got != want {
			t.Errorf("hasRealMetadata(%d) = %v, want %v", idx, got, want)
		}
	}
}

func TestHianimeToPanelLayersTheShowPageOverTheCard(t *testing.T) {
	r := &source.Anime{
		ShowID:       "9999",
		Name:         "Demon Slayer: Kimetsu no Yaiba Infinity Castle III",
		Cover:        "https://cdn.example/poster.jpg",
		Type:         "MOVIE (1 EPS)",
		EpisodeCount: 1,
		Year:         2027,
	}
	meta := &hianime.ShowMetadata{
		Synopsis: "The final battle.",
		Japanese: "鬼滅の刃",
		Year:     2025,
		Status:   "Finished Airing",
		Score:    8.57,
		Genres:   []string{"Action", "Fantasy"},
		Type:     "MOVIE",
		Episodes: 1,
	}

	panel := hianimeToPanel(r, meta)
	if panel == nil {
		t.Fatal("hianimeToPanel() = nil, want panel")
	}
	if panel.Title != r.Name {
		t.Errorf("Title = %q, want the search card title %q", panel.Title, r.Name)
	}
	if panel.Cover != r.Cover {
		t.Errorf("Cover = %q, want the search card cover", panel.Cover)
	}
	if panel.Synopsis != meta.Synopsis {
		t.Errorf("Synopsis = %q, want the page synopsis", panel.Synopsis)
	}
	if panel.TitleNative != meta.Japanese {
		t.Errorf("TitleNative = %q, want %q", panel.TitleNative, meta.Japanese)
	}
	if panel.Score != meta.Score {
		t.Errorf("Score = %v, want %v", panel.Score, meta.Score)
	}
	if len(panel.Genres) != 2 || panel.Genres[0] != "Action" {
		t.Errorf("Genres = %v", panel.Genres)
	}
	if panel.Status != "FINISHED" {
		t.Errorf("Status = %q, want FINISHED", panel.Status)
	}
	if panel.Type != "MOVIE" {
		t.Errorf("Type = %q, want MOVIE", panel.Type)
	}
	if panel.Episodes != 1 {
		t.Errorf("Episodes = %d, want 1", panel.Episodes)
	}
	// The search card already knows the year: the page's premiere year
	// must not overwrite it.
	if panel.Year != 2027 {
		t.Errorf("Year = %d, want the search card year 2027", panel.Year)
	}
	if panel.Source != "Hianime" {
		t.Errorf("Source = %q, want Hianime", panel.Source)
	}
	if (&SearchModel{metadataCache: map[int]*MetadataPanel{0: panel}}).hasRealMetadata(0) {
		t.Error("the hianime panel must stay provisional")
	}
}

func TestHianimeToPanelFallsBackToTheCard(t *testing.T) {
	r := &source.Anime{Name: "Some Unaired Film", Cover: "https://cdn.example/p.jpg", Type: "MOVIE"}

	card := searchResultPanel(r)
	if got := hianimeToPanel(r, nil); got == nil || got.Source != card.Source || got.Title != card.Title {
		t.Errorf("hianimeToPanel(r, nil) = %+v, want the search card", got)
	}
	if got := hianimeToPanel(nil, nil); got != nil {
		t.Errorf("hianimeToPanel(nil, nil) = %+v, want nil", got)
	}
}

func TestHianimeStatus(t *testing.T) {
	cases := map[string]string{
		"Finished Airing":  "FINISHED",
		"Currently Airing": "RELEASING",
		"Not yet aired":    "NOT_YET_RELEASED",
		"":                 "",
		"Hiatus":           "HIATUS",
	}
	for in, want := range cases {
		if got := hianimeStatus(in); got != want {
			t.Errorf("hianimeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
