package hianime

import (
	"context"
	"testing"
	"time"

	"github.com/hishantik/anilix/source"
)

var _ source.Source = (*Provider)(nil)

// A missing episode number must error, not silently play episode 1.
func TestLookupDataIDMissingEpisode(t *testing.T) {
	p := NewProvider()
	p.episodes["999"] = cachedEpisodes{
		entries: []EpisodeEntry{{Number: "1", DataID: "d1"}, {Number: "2", DataID: "d2"}},
		fetched: time.Now(),
	}

	got, err := p.lookupDataID(context.Background(), &source.Episode{
		Anime:  &source.Anime{ShowID: "999"},
		Number: 12.5,
	})
	if err == nil {
		t.Fatalf("expected error for missing episode 12.5, got data-id %q", got)
	}
}

func TestParseSearchResults(t *testing.T) {
	// Mirrors the real search page: a results grid of <div class="flw-item">
	// blocks, followed by carousel sections that use a different markup
	// ('film-poster item-qtip' with data-id). The carousel entries must not
	// be mixed into the results - they carry unrelated show ids.
	page := `
	<div class="flw-item flw-item-big">
		<div class="film-poster">
			<div class="tick ltr">
				<div class="tick-item tick-eps">26</div>
			</div>
			<img src="https://cdn.anipixcdn.co/thumbnail/bebop.jpg" class="film-poster-img" alt="Cowboy Bebop" loading="lazy">
			<a href="https://hianime.at/watch/cowboy-bebop-1281" class="film-poster-ahref item-qtip" title="Cowboy Bebop" aria-label="Watch Cowboy Bebop" data-id="1281"><i class="fas fa-play"></i></a>
		</div>
		<div class="film-detail">
			<h3 class="film-name">
				<a href="https://hianime.at/cowboy-bebop-1281" title="Cowboy Bebop" class="dynamic-name" data-jname="Cowboy Bebop">Cowboy Bebop</a>
			</h3>
			<div class="fd-infor"><span class="fdi-item">TV</span></div>
		</div>
	</div>
	<div class="flw-item flw-item-big">
		<div class="film-poster">
			<div class="tick ltr">
				<div class="tick-item tick-eps">1</div>
			</div>
			<img src="https://cdn.anipixcdn.co/thumbnail/chase.webp" class="film-poster-img" alt="Chase" loading="lazy">
			<a href="https://hianime.at/watch/chase-5300" class="film-poster-ahref item-qtip" title="Chase" aria-label="Watch Chase" data-id="5300"><i class="fas fa-play"></i></a>
		</div>
		<div class="film-detail">
			<h3 class="film-name">
				<a href="https://hianime.at/chase-5300" title="Chase &amp; Run" class="dynamic-name">Chase &amp; Run</a>
			</h3>
			<div class="fd-infor"><span class="fdi-item">MOVIE</span></div>
		</div>
	</div>
	<div class="slider-item">
		<div class="film-poster item-qtip" data-id="1">
			<img src="https://hianime.at/storage/one-piece.webp" class="film-poster-img" alt="One Piece" loading="lazy">
		</div>
		<div class="film-detail"><h3 class="film-name"><a href="https://hianime.at/one-piece-1" title="One Piece" class="dynamic-name">One Piece</a></h3></div>
	</div>`

	results := parseSearchResults(page)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	if results[0].ShowID != "1281" || results[0].Title != "Cowboy Bebop" || results[0].Slug != "cowboy-bebop-1281" {
		t.Errorf("result 0 wrong: %+v", results[0])
	}
	if results[0].Poster != "https://cdn.anipixcdn.co/thumbnail/bebop.jpg" {
		t.Errorf("poster not captured: %+v", results[0])
	}
	if results[0].Type != "TV" {
		t.Errorf("type not captured: %+v", results[0])
	}
	if results[0].Episodes != 26 {
		t.Errorf("episode count not captured: %+v", results[0])
	}
	if results[1].ShowID != "5300" {
		t.Errorf("result 1 id wrong (carousel ids leaked in?): %+v", results[1])
	}
	if results[1].Title != "Chase & Run" {
		t.Errorf("HTML entities not unescaped: %q", results[1].Title)
	}
	if results[1].Type != "MOVIE" {
		t.Errorf("result 1 type wrong: %+v", results[1])
	}
	if results[1].Episodes != 1 {
		t.Errorf("result 1 episode count wrong: %+v", results[1])
	}
}

func TestParseEpisodeList(t *testing.T) {
	page := `{\"html\":\"<div id=\\\"ssc-list\\\">` +
		`<a class=\"ssl-item ep-item\" data-number=\\\"1\\\" data-id=\\\"1171\\\" title=\\\"Romance Dawn\\\">1</a>` +
		`<a class=\"ssl-item ep-item\" data-number=\\\"2\\\" data-id=\\\"1172\\\" title=\\\"2</a>` +
		`\"}`

	entries := parseEpisodeList(page)
	if len(entries) != 2 {
		t.Fatalf("expected 2 episodes, got %d: %+v", len(entries), entries)
	}
	if entries[0].Number != "1" || entries[0].DataID != "1171" {
		t.Errorf("entry 0 wrong: %+v", entries[0])
	}
	if entries[1].Number != "2" || entries[1].DataID != "1172" {
		t.Errorf("entry 1 wrong: %+v", entries[1])
	}
}

func TestParseServers(t *testing.T) {
	// Real markup: server names change over time (ZokoAnime, HD-1, ...), and
	// the embed host is what identifies a decodable player.
	page := `\"<div class=\\\"ps__-list\\\">` +
		`<div class=\\\"server-item\\\" data-type=\\\"sub\\\" data-server-id=\\\"1\\\" data-server-name=\\\"Vidstream\\\" data-hash=\\\"AAAA\\\">` +
		`<div class=\\\"server-item\\\" data-type=\\\"sub\\\" data-server-id=\\\"2\\\" data-server-name=\\\"ZokoAnime\\\" data-hash=\\\"aHR0cHM6Ly96b2tvYW5pbWUudmlkZW8vc3RyZWFtL21hbC8yMS8xL3N1Yg==\\\">` +
		`<div class=\\\"server-item\\\" data-type=\\\"dub\\\" data-server-id=\\\"3\\\" data-server-name=\\\"ZokoAnime\\\" data-hash=\\\"aHR0cHM6Ly96b2tvYW5pbWUudmlkZW8vc3RyZWFtL21hbC8yMS8xL2R1Yg==\\\">` +
		`\"}`

	cands := parseServers(page)
	if len(cands) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %+v", len(cands), cands)
	}

	// The exact translation wins.
	sub := rankEmbeds(cands, "sub")[0]
	if sub.Embed != "https://zokoanime.video/stream/mal/21/1/sub" {
		t.Errorf("wrong sub embed: %q", sub.Embed)
	}
	if !sub.HasMAL {
		t.Errorf("MAL id not detected in embed: %+v", sub)
	}

	dub := rankEmbeds(cands, "dub")[0]
	if dub.Embed != "https://zokoanime.video/stream/mal/21/1/dub" {
		t.Errorf("wrong dub embed: %q", dub.Embed)
	}

	// An unknown translation falls back to sub.
	fallback := rankEmbeds(cands, "raw")[0]
	if fallback.Embed != "https://zokoanime.video/stream/mal/21/1/sub" {
		t.Errorf("fallback should be sub, got %q", fallback.Embed)
	}
}

func TestParseServersOnlyDub(t *testing.T) {
	// Some entries only have a dub server (and an internal id instead of a
	// MAL id) - playback must still work.
	page := `\"<div class=\\\"ps__-list\\\">` +
		`<div class=\\\"server-item\\\" data-type=\\\"dub\\\" data-server-name=\\\"ZokoAnime\\\" data-hash=\\\"aHR0cHM6Ly96b2tvYW5pbWUudmlkZW8vc3RyZWFtL2FuaW1lLzEyNzM3OS9kdWI=\\\">` +
		`\"}`

	cands := parseServers(page)
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	if cands[0].Type != "dub" {
		t.Errorf("wrong type: %+v", cands[0])
	}
	if cands[0].HasMAL {
		t.Errorf("internal-id embed must not report a MAL id: %+v", cands[0])
	}

	// Requesting sub falls back to the dub server.
	picked := rankEmbeds(cands, "sub")[0]
	if picked.Embed != "https://zokoanime.video/stream/anime/127379/dub" {
		t.Errorf("fallback embed wrong: %q", picked.Embed)
	}
}

func TestParseServersHostIndependent(t *testing.T) {
	// A server named HD-1 can still point at the decodable player.
	page := `\"<div class=\\\"ps__-list\\\">` +
		`<div class=\\\"server-item\\\" data-type=\\\"sub\\\" data-server-name=\\\"HD-1\\\" data-hash=\\\"aHR0cHM6Ly96b2tvYW5pbWUudmlkZW8vc3RyZWFtL21hbC8xNTQ5LzEvc3Vi\\\">` +
		`\"}`

	cands := parseServers(page)
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	if !cands[0].HasMAL {
		t.Errorf("MAL id not detected: %+v", cands[0])
	}
	if !playerHost(cands[0].Embed) {
		t.Errorf("player host not detected: %+v", cands[0])
	}
}

func TestDeobfuscateBlob(t *testing.T) {
	// Build a blob the same way the embed page does: base64(json XOR key)
	jsonStr := `{"src":"https://cdn.example.com/master.m3u8?abc=1","subtitles":[{"src":"https://cdn.example.com/sub.vtt","default":true,"label":"English"}]}`
	key := []byte(obfKey)
	xored := make([]byte, len(jsonStr))
	for i, c := range []byte(jsonStr) {
		xored[i] = c ^ key[i%len(key)]
	}
	blob := base64Encode(xored)

	cfg, err := deobfuscateBlob(blob)
	if err != nil {
		t.Fatalf("deobfuscate failed: %v", err)
	}
	if cfg.Src != "https://cdn.example.com/master.m3u8?abc=1" {
		t.Errorf("wrong src: %q", cfg.Src)
	}
	if len(cfg.Subtitles) != 1 || !cfg.Subtitles[0].Default {
		t.Errorf("subtitles not decoded: %+v", cfg.Subtitles)
	}
	if got := (&StreamJSON{Subtitles: cfg.Subtitles}).defaultSubtitle(); got != "https://cdn.example.com/sub.vtt" {
		t.Errorf("default subtitle wrong: %q", got)
	}
}

func TestParseMasterPlaylist(t *testing.T) {
	master := "https://cdn.example.com/path/master.m3u8"
	playlist := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1920x1080,NAME="1080"
1080/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=1280x720,NAME="720"
720/index.m3u8
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=100000,RESOLUTION=640x360,URI="iframe.m3u8"
`
	variants := parseMasterPlaylist(playlist, master)
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d: %+v", len(variants), variants)
	}
	if variants[0].Quality != "1080p" || variants[0].URL != "https://cdn.example.com/path/1080/index.m3u8" {
		t.Errorf("variant 0 wrong: %+v", variants[0])
	}
	if variants[1].Quality != "720p" {
		t.Errorf("variant 1 wrong: %+v", variants[1])
	}
}

func TestMalIDFromEmbed(t *testing.T) {
	if got := malIDFromEmbed("https://zokoanime.video/stream/mal/21/1/sub"); got != 21 {
		t.Errorf("expected 21, got %d", got)
	}
	if got := malIDFromEmbed("https://other.host/stream/abc/1/sub"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestSearchEscape(t *testing.T) {
	if got := searchEscape("one piece"); got != "one+piece" {
		t.Errorf("space not escaped: %q", got)
	}
	if got := searchEscape("  a&b+c  "); got != "a%26b%2Bc" {
		t.Errorf("query escaping wrong: %q", got)
	}
}
