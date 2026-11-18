package hianime

import "testing"

// showPageFixture is the detail block of a TV show page, trimmed to the
// markup the parser reads. The values are the ones hianime served for
// Demon Slayer.
const showPageFixture = `
<div class="anisc-detail">
    <div class="film-stats">
        <div class="tick">
            <div class="tick-item tick-pg">R</div>
            <div class="tick-item tick-quality">HD</div>
            <div class="tick-item tick-sub">
                <i class="fas fa-closed-captioning mr-1"></i>26
            </div>
            <div class="tick-item tick-dub">
                <i class="fas fa-microphone mr-1"></i>26
            </div>
            <div class="tick-item tick-eps">
                26
            </div>
            <span class="dot"></span>
            <span class="item">TV</span>
            <span class="dot"></span>
            <span class="item">23m</span>
        </div>
        <div class="clearfix"></div>
    </div>
</div>
<div class="anisc-info-wrap">
    <div class="item item-title">
        <span class="item-head">Overview:</span>
        <div class="text">
            Ever since the death of his father, the burden of supporting the family
            has fallen upon Tanjirou Kamado&#039;s shoulders.
        </div>
    </div>
    <div class="item item-title">
        <span class="item-head">Japanese:</span>
        <span class="name">鬼滅の刃</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Synonyms:</span>
        <span class="name">Kimetsu no Yaiba</span>
        <span class="name">Blade of Demon Destruction</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Aired:</span>
        <span class="name">Apr 6, 2019 to Sep 28, 2019</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Duration:</span>
        <span class="name">23m</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Status:</span>
        <span class="name">Finished Airing</span>
    </div>
    <div class="item item-title">
        <span class="item-head">MAL Score:</span>
        <span class="name">8.57</span>
    </div>
    <div class="item item-list">
        <span class="item-head">Genres:</span>
        <a href="https://hianime.fm/genres/action" title="Action"> Action </a>
        <a href="https://hianime.fm/genres/fantasy" title="Fantasy"> Fantasy </a>
        <a href="https://hianime.fm/genres/historical" title="Historical"> Historical </a>
    </div>
    <div class="item item-title">
        <span class="item-head">Studios:</span>
        <a class="name" href="https://hianime.fm/studios/ufotable" title="ufotable"> ufotable </a>
    </div>
    <div class="item item-title">
        <span class="item-head">Producers:</span>
        <a class="name" href="https://hianime.fm/producers/aniplex" title="Aniplex"> Aniplex </a>
        <a class="name" href="https://hianime.fm/producers/shueisha" title="Shueisha"> Shueisha </a>
    </div>
</div>
`

// moviePageFixture is a movie page: hianime omits Aired, Japanese and MAL
// Score on some of them, so those rows must not be required.
const moviePageFixture = `
<div class="film-stats">
    <div class="tick">
        <div class="tick-item tick-pg">R</div>
        <div class="tick-item tick-quality">HD</div>
        <div class="tick-item tick-eps">
            1
        </div>
        <span class="dot"></span>
        <span class="item">MOVIE</span>
        <span class="dot"></span>
        <span class="item">138m</span>
    </div>
    <div class="clearfix"></div>
</div>
<div class="anisc-info-wrap">
    <div class="item item-title">
        <span class="item-head">Overview:</span>
        <div class="text">Tanjiro and Inosuke encounter a mountain laced with spider webs.</div>
    </div>
    <div class="item item-title">
        <span class="item-head">Synonyms:</span>
        <span class="name">Demon Slayer: Natagumo Mountain Arc</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Duration:</span>
        <span class="name">138m</span>
    </div>
    <div class="item item-title">
        <span class="item-head">Status:</span>
        <span class="name">Finished Airing</span>
    </div>
    <div class="item item-list">
        <span class="item-head">Genres:</span>
        <a href="https://hianime.fm/genres/fantasy" title="Fantasy"> Fantasy </a>
        <a href="https://hianime.fm/genres/mystery" title="Mystery"> Mystery </a>
    </div>
</div>
`

func TestParseShowMetadata(t *testing.T) {
	meta := parseShowMetadata(showPageFixture)

	if want := "Ever since the death of his father, the burden of supporting the family has fallen upon Tanjirou Kamado's shoulders."; meta.Synopsis != want {
		t.Errorf("Synopsis = %q, want %q", meta.Synopsis, want)
	}
	if meta.Japanese != "鬼滅の刃" {
		t.Errorf("Japanese = %q", meta.Japanese)
	}
	if len(meta.Synonyms) != 2 || meta.Synonyms[0] != "Kimetsu no Yaiba" {
		t.Errorf("Synonyms = %v", meta.Synonyms)
	}
	if meta.Aired != "Apr 6, 2019 to Sep 28, 2019" {
		t.Errorf("Aired = %q", meta.Aired)
	}
	if meta.Year != 2019 {
		t.Errorf("Year = %d, want 2019", meta.Year)
	}
	if meta.Duration != 23 {
		t.Errorf("Duration = %d, want 23", meta.Duration)
	}
	if meta.Status != "Finished Airing" {
		t.Errorf("Status = %q", meta.Status)
	}
	if meta.Score != 8.57 {
		t.Errorf("Score = %v, want 8.57", meta.Score)
	}
	if len(meta.Genres) != 3 || meta.Genres[0] != "Action" || meta.Genres[2] != "Historical" {
		t.Errorf("Genres = %v", meta.Genres)
	}
	if len(meta.Studios) != 1 || meta.Studios[0] != "ufotable" {
		t.Errorf("Studios = %v", meta.Studios)
	}
	if len(meta.Producers) != 2 || meta.Producers[1] != "Shueisha" {
		t.Errorf("Producers = %v", meta.Producers)
	}
	if meta.Type != "TV" {
		t.Errorf("Type = %q, want TV", meta.Type)
	}
	if meta.Rating != "R" {
		t.Errorf("Rating = %q, want R", meta.Rating)
	}
	if meta.Episodes != 26 {
		t.Errorf("Episodes = %d, want 26", meta.Episodes)
	}
}

func TestParseShowMetadataWithoutTheOptionalRows(t *testing.T) {
	meta := parseShowMetadata(moviePageFixture)

	if meta.Synopsis == "" {
		t.Error("movie page lost its synopsis")
	}
	if meta.Japanese != "" || meta.Aired != "" || meta.Year != 0 || meta.Score != 0 {
		t.Errorf("absent rows must stay empty: japanese=%q aired=%q year=%d score=%v",
			meta.Japanese, meta.Aired, meta.Year, meta.Score)
	}
	if meta.Type != "MOVIE" {
		t.Errorf("Type = %q, want MOVIE", meta.Type)
	}
	if meta.Duration != 138 {
		t.Errorf("Duration = %d, want 138", meta.Duration)
	}
	if meta.Episodes != 1 {
		t.Errorf("Episodes = %d, want 1", meta.Episodes)
	}
	if meta.Status != "Finished Airing" {
		t.Errorf("Status = %q", meta.Status)
	}
	if len(meta.Genres) != 2 || meta.Genres[1] != "Mystery" {
		t.Errorf("Genres = %v", meta.Genres)
	}
}

func TestParseShowMetadataOnAPageWithoutTheBlock(t *testing.T) {
	meta := parseShowMetadata(`<html><body>not a show page</body></html>`)
	if meta == nil {
		t.Fatal("parseShowMetadata returned nil")
	}
	if meta.Synopsis != "" || meta.Type != "" || meta.Genres != nil {
		t.Errorf("a page without the block must parse empty: %+v", meta)
	}
}

func TestParseShowMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"23m", 23},
		{"138m", 138},
		{"1h 23m", 83},
		{"2h", 120},
		{"", 0},
		{"unknown", 0},
	}
	for _, c := range cases {
		if got := parseShowMinutes(c.in); got != c.want {
			t.Errorf("parseShowMinutes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
