package hianime

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ShowMetadata is the detail block of a hianime show page: the fields the
// site itself renders. It is a placeholder source: the page carries no
// stable ids, and the synopsis, score and genres are the site's own copy
// (the MAL score included), so real metadata panels win over it.
type ShowMetadata struct {
	Synopsis  string
	Japanese  string
	Synonyms  []string
	Aired     string
	Year      int
	Duration  int // minutes, 0 when the page does not say
	Status    string
	Score     float64 // MAL score as shown by hianime, 0 when absent
	Genres    []string
	Studios   []string
	Producers []string
	Type      string
	Rating    string
	Episodes  int
}

// cachedShowMetadata is one cached show-page metadata block.
type cachedShowMetadata struct {
	meta    *ShowMetadata
	fetched time.Time
}

// ShowMetadata returns the detail block of a show page. Pages are ~200KB,
// so they are cached for the same 30 minutes as the episode list.
func (p *Provider) ShowMetadata(ctx context.Context, showID, pageURL string) (*ShowMetadata, error) {
	key := showID
	if key == "" {
		key = pageURL
	}
	if key == "" {
		return nil, fmt.Errorf("show metadata: no show id or url")
	}

	p.cacheMu.Lock()
	if cached, ok := p.showMeta[key]; ok && time.Since(cached.fetched) < episodeCacheTTL {
		p.cacheMu.Unlock()
		return cached.meta, nil
	}
	p.cacheMu.Unlock()

	if pageURL == "" {
		return nil, fmt.Errorf("show metadata: no page url for show %s", showID)
	}
	if strings.HasPrefix(pageURL, "/") {
		pageURL = BaseURL + pageURL
	}

	page, err := get(ctx, pageURL, "")
	if err != nil {
		return nil, err
	}
	meta := parseShowMetadata(page)

	p.cacheMu.Lock()
	p.showMeta[key] = cachedShowMetadata{meta: meta, fetched: time.Now()}
	trimCacheLocked(p.showMeta, showMetaMax, episodeCacheTTL, func(c cachedShowMetadata) time.Time { return c.fetched })
	p.cacheMu.Unlock()
	return meta, nil
}

var (
	showOverviewRe  = regexp.MustCompile(`(?s)<span class="item-head">Overview:</span>\s*<div class="text">(.*?)</div>`)
	showAnchorRe    = regexp.MustCompile(`(?s)<a\b([^>]*)>(.*?)</a>`)
	showTitleAttrRe = regexp.MustCompile(`title="([^"]*)"`)
	showFilmTypeRe  = regexp.MustCompile(`<span class="item">([^<]+)</span>`)
	showRatingRe    = regexp.MustCompile(`<div class="tick-item tick-pg">([^<]*)</div>`)
	showEpisodesRe  = regexp.MustCompile(`<div class="tick-item tick-eps">\s*(\d+)`)
	showTagRe       = regexp.MustCompile(`<[^>]+>`)
	showSpaceRe     = regexp.MustCompile(`\s+`)
	showYearRe      = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
	showHoursRe     = regexp.MustCompile(`(\d+)\s*h(?:\s*(\d+)\s*m)?`)
	showMinutesRe   = regexp.MustCompile(`(\d+)\s*m`)
)

// parseShowMetadata extracts the info block and the film stats of a show
// page. A page that misses the block yields an empty struct, not an error:
// the caller still has the search-card placeholder.
func parseShowMetadata(page string) *ShowMetadata {
	meta := &ShowMetadata{}

	if m := showOverviewRe.FindStringSubmatch(page); m != nil {
		meta.Synopsis = cleanShowText(m[1])
	}
	meta.Japanese = showNameField(page, "Japanese")
	meta.Synonyms = showNameList(page, "Synonyms")
	meta.Aired = showNameField(page, "Aired")
	if y := showYearRe.FindString(meta.Aired); y != "" {
		meta.Year, _ = strconv.Atoi(y)
	}
	if d := showNameField(page, "Duration"); d != "" {
		meta.Duration = parseShowMinutes(d)
	}
	meta.Status = showNameField(page, "Status")
	if score := showNameField(page, "MAL Score"); score != "" {
		meta.Score, _ = strconv.ParseFloat(score, 64)
	}
	meta.Genres = showLinkList(page, "Genres")
	meta.Studios = showLinkList(page, "Studios")
	meta.Producers = showLinkList(page, "Producers")

	meta.Type, meta.Rating, meta.Episodes = parseFilmStats(page)
	return meta
}

// showNameField reads a "<span class="name">" field of the info block.
func showNameField(page, label string) string {
	re := regexp.MustCompile(`(?s)<span class="item-head">` + regexp.QuoteMeta(label) + `:</span>\s*<span class="name">(.*?)</span>`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return cleanShowText(m[1])
}

// showNameList reads a field whose values are several <span class="name">
// elements (the Synonyms row renders one span per synonym).
func showNameList(page, label string) []string {
	re := regexp.MustCompile(`(?s)<span class="item-head">` + regexp.QuoteMeta(label) + `:</span>(.*?)</div>`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	spanRe := regexp.MustCompile(`(?s)<span class="name">(.*?)</span>`)
	var out []string
	for _, s := range spanRe.FindAllStringSubmatch(m[1], -1) {
		if v := cleanShowText(s[1]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// showLinkList reads a field whose values are links (Genres, Studios,
// Producers). The link title is the canonical name; the text is a fallback.
func showLinkList(page, label string) []string {
	re := regexp.MustCompile(`(?s)<span class="item-head">` + regexp.QuoteMeta(label) + `:</span>(.*?)</div>`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	var out []string
	for _, a := range showAnchorRe.FindAllStringSubmatch(m[1], -1) {
		name := ""
		if t := showTitleAttrRe.FindStringSubmatch(a[1]); t != nil {
			name = html.UnescapeString(t[1])
		} else {
			name = cleanShowText(a[2])
		}
		name = strings.TrimSpace(name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parseFilmStats reads the type, rating and episode count from the stats
// row under the poster.
func parseFilmStats(page string) (format, rating string, episodes int) {
	start := strings.Index(page, `<div class="film-stats">`)
	if start < 0 {
		return "", "", 0
	}
	block := page[start:]
	if end := strings.Index(block, `<div class="clearfix">`); end > 0 {
		block = block[:end]
	}
	if m := showRatingRe.FindStringSubmatch(block); m != nil {
		rating = cleanShowText(m[1])
	}
	if m := showEpisodesRe.FindStringSubmatch(block); m != nil {
		episodes, _ = strconv.Atoi(m[1])
	}
	for _, m := range showFilmTypeRe.FindAllStringSubmatch(block, -1) {
		word := strings.ToUpper(strings.TrimSpace(cleanShowText(m[1])))
		if isShowFormat(word) {
			format = word
			break
		}
	}
	return format, rating, episodes
}

// isShowFormat reports whether the word is one of the formats hianime puts
// in the stats row.
func isShowFormat(word string) bool {
	switch word {
	case "TV", "TV SHORT", "MOVIE", "OVA", "ONA", "SPECIAL", "MUSIC", "CM", "PV":
		return true
	}
	return false
}

// parseShowMinutes turns "23m" or "1h 23m" into minutes.
func parseShowMinutes(s string) int {
	if m := showHoursRe.FindStringSubmatch(s); m != nil {
		total, _ := strconv.Atoi(m[1])
		total *= 60
		if m[2] != "" {
			mm, _ := strconv.Atoi(m[2])
			total += mm
		}
		return total
	}
	if m := showMinutesRe.FindStringSubmatch(s); m != nil {
		mm, _ := strconv.Atoi(m[1])
		return mm
	}
	return 0
}

// cleanShowText strips the markup of a field value and collapses the
// whitespace left behind.
func cleanShowText(s string) string {
	s = showTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(showSpaceRe.ReplaceAllString(s, " "))
}
