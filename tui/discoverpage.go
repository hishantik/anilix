package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// The discover screen.
//
// The screen lists the top airing and the most popular anime, straight
// from Jikan, and - when the terminal has the room - a preview of the
// selected entry, artwork included where the terminal can draw it. It is
// the entry point the navigation drawer opens with: pick a title, and the
// search screen takes over with that query.
//
// The catalogue comes from Jikan because AniList has no MAL id on its
// trending list without a second request per entry, and Jikan answers
// with everything the list needs in one call. The circuit breaker of the
// client applies: when the API is down the screen says so instead of
// stalling.

// The two sections of the discover screen.
const (
	discoverSectionTrending = "trending"
	discoverSectionPopular  = "popular"
)

// discoverCoverIndex is the result index the discover screen sends its
// cover downloads with: a negative index never collides with the results
// of the search screen, which start at zero.
const discoverCoverIndex = -2

// gotoDiscover opens the discover screen.
//
// A catalogue that is already on screen is not fetched again: the screen
// keeps its list until the user leaves the application.
func (m *SearchModel) gotoDiscover() tea.Cmd {
	if m.state != discoverState {
		m.prevState = m.state
	}
	m.state = discoverState
	m.discover = &DiscoverState{Section: discoverSectionTrending}
	m.discoverResetCover()
	m.textInput.Blur()

	if m.discoverHasCache() {
		return tea.Batch(m.discoverCoverCmd(), m.clearCoverCmd())
	}
	m.discover.Loading = true
	return tea.Batch(m.fetchDiscoverCmd(), m.clearCoverCmd())
}

// discoverHasCache reports whether a catalogue is already on the model.
func (m *SearchModel) discoverHasCache() bool {
	return len(m.discover.Trending) > 0 || len(m.discover.Popular) > 0
}

// fetchDiscoverCmd reads the top airing and the most popular anime from
// Jikan in one background pass: both lists arrive together, so the screen
// never flips between the two sections while one is still loading.
func (m *SearchModel) fetchDiscoverCmd() tea.Cmd {
	client := m.jikanClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		trending, err := fetchDiscoverList(ctx, client, "airing")
		if err != nil {
			return DiscoverLoadedMsg{Err: err}
		}
		popular, err := fetchDiscoverList(ctx, client, "bypopularity")
		if err != nil {
			return DiscoverLoadedMsg{Err: err}
		}
		return DiscoverLoadedMsg{Trending: trending, Popular: popular}
	}
}

// fetchDiscoverList fetches one section of the discover catalogue and
// converts the entries into the common anime type.
func fetchDiscoverList(ctx context.Context, client *jikan.JikanClient, filter string) ([]*source.Anime, error) {
	resp, err := client.GetTopAnime(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("could not load the catalogue: %w", err)
	}
	animes := make([]*source.Anime, 0, len(resp.Data))
	for i := range resp.Data {
		animes = append(animes, jikan.MapToAnime(&resp.Data[i]))
	}
	return animes, nil
}

// discoverList returns the list of the section on screen.
func (m *SearchModel) discoverList() []*source.Anime {
	if m.discover.Section == discoverSectionPopular {
		return m.discover.Popular
	}
	return m.discover.Trending
}

// discoverSelected returns the anime under the cursor, or nil.
func (m *SearchModel) discoverSelected() *source.Anime {
	list := m.discoverList()
	if m.discover.Selected < 0 || m.discover.Selected >= len(list) {
		return nil
	}
	return list[m.discover.Selected]
}

// discoverCounter returns the position of the cursor in the list.
func (m *SearchModel) discoverCounter() string {
	total := len(m.discoverList())
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", m.discover.Selected+1, total)
}

// handleDiscoverKey handles a key on the discover screen.
//
// Enter searches for the title the cursor is on: the search screen owns
// the results, the metadata and the playback, so the discover screen
// never builds a second path into the episode list.
func (m *SearchModel) handleDiscoverKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Quit):
		m.prevState = m.state
		m.confirmSelect = 1
		m.state = confirmQuitState
		return m, nil

	case key.Matches(msg, m.keymap.Back):
		m.state = m.prevState
		return m, nil

	case msg.String() == "left" || msg.String() == "h":
		if m.discover.Section == discoverSectionPopular {
			m.discover.Section = discoverSectionTrending
			m.discover.Selected = 0
			m.discover.Scroll = 0
			return m, m.discoverCoverCmd()
		}
		return m, nil

	case msg.String() == "right" || msg.String() == "l":
		if m.discover.Section == discoverSectionTrending {
			m.discover.Section = discoverSectionPopular
			m.discover.Selected = 0
			m.discover.Scroll = 0
			return m, m.discoverCoverCmd()
		}
		return m, nil

	case key.Matches(msg, m.keymap.Up):
		if m.discover.Selected > 0 {
			m.discover.Selected--
			return m, m.discoverCoverCmd()
		}
		return m, nil

	case key.Matches(msg, m.keymap.Down):
		if m.discover.Selected < len(m.discoverList())-1 {
			m.discover.Selected++
			return m, m.discoverCoverCmd()
		}
		return m, nil

	case key.Matches(msg, m.keymap.Select):
		anime := m.discoverSelected()
		if anime == nil {
			return m, nil
		}
		m.textInput.SetValue(anime.Name)
		m.lastQuery = anime.Name
		m.searchState.Query = anime.Name
		m.searchState.Loading = true
		m.searchState.Results = nil
		m.searchState.Err = nil
		m.progressPercent = 0
		m.progressTarget = 0.3
		m.textInput.Blur()
		m.state = searchState
		return m, tea.Batch(m.doSearch(anime.Name), m.loading.Tick)
	}

	return m, nil
}

// viewDiscoverState renders the discover screen.
//
// A terminal with room for one column gets the list alone; a wider one
// gets the list and the preview of the selected entry, artwork included
// where the terminal can draw it.
func (m *SearchModel) viewDiscoverState() string {
	listW, previewW, height, isNarrow := watchlistLayout(m.contentWidth, m.contentHeight)

	// The preview is for an entry: a loading, failed or empty screen gets
	// the full width, so the state is not squeezed next to a panel that
	// says "nothing selected" twice.
	if isNarrow || previewW < 20 || len(m.discoverList()) == 0 {
		if previewW >= 20 {
			listW = m.contentWidth - 2
			if listW < 20 {
				listW = 20
			}
		}
		return m.renderDiscoverPanel(listW, height)
	}

	return joinPanels(
		m.renderDiscoverPanel(listW, height),
		listW,
		m.renderDiscoverPreview(previewW, height),
		previewW,
	)
}

// renderDiscoverPanel renders the list panel of the discover screen, cut
// to the height it has.
//
// Every entry renders as two lines - the headline and its facts line -
// and the window of a long list moves in steps of rows, the same way the
// watchlist moves.
func (m *SearchModel) renderDiscoverPanel(width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	header := ui.SectionHeader(m.discoverHeading(), m.discoverCounter(), width)

	if m.discover.Loading {
		return header + "\n" + m.loadingLine("Loading the catalogue...")
	}
	if m.discover.Err != nil {
		return header + "\n" + ui.ErrorState{
			Title:   "Could not load the catalogue",
			Message: m.discover.Err.Error(),
			Hint:    "esc to go back, and try again from the drawer",
		}.Render(width)
	}

	list := m.discoverList()
	if len(list) == 0 {
		return header + "\n" + ui.EmptyState{
			Icon:  theme.Icon(theme.IconDiscover),
			Title: "Nothing to show",
			Hint:  "The catalogue is empty right now. Try again later.",
		}.Render(width)
	}

	// Two rows per entry: the headline and the facts line under it.
	visible := (height - 2) / 2
	if visible < 1 {
		visible = 1
	}

	start := m.discover.Scroll
	if m.discover.Selected < start {
		start = m.discover.Selected
	}
	if m.discover.Selected >= start+visible {
		start = m.discover.Selected - visible + 1
	}
	if start < 0 {
		start = 0
	}
	m.discover.Scroll = start

	rows := make([]string, 0, visible*2)
	for i := start; i < len(list) && i < start+visible; i++ {
		a := list[i]
		rows = append(rows, ui.ListItem{
			Title:       a.Name,
			Description: discoverFactsLine(a),
			Trailing:    ui.RatingText(float64(a.Score)),
			Selected:    i == m.discover.Selected,
			Focused:     true,
		}.Render(width))
	}
	return header + "\n" + joinLinesDropEmpty(rows)
}

// discoverHeading names the section on screen, with the switch hint.
func (m *SearchModel) discoverHeading() string {
	if m.discover.Section == discoverSectionPopular {
		return "Discover · Popular  ← airing"
	}
	return "Discover · Airing  popular →"
}

// discoverFactsLine renders the static facts of an entry: the format,
// the year, the episode count and the genres.
func discoverFactsLine(a *source.Anime) string {
	facts := make([]string, 0, 4)
	if a.Type != "" {
		facts = append(facts, a.Type)
	}
	if a.Year > 0 {
		facts = append(facts, strconv.Itoa(a.Year))
	}
	if a.EpisodeCount > 0 {
		facts = append(facts, fmt.Sprintf("Ep %d", a.EpisodeCount))
	}
	if len(a.Genres) > 0 {
		join := strings.Join(a.Genres, ", ")
		if len(join) > 24 {
			join = join[:24] + "…"
		}
		facts = append(facts, join)
	}
	return strings.Join(facts, " · ")
}

// renderDiscoverPreview renders the preview of the selected entry: the
// artwork and the facts, the same shape the watchlist preview uses.
func (m *SearchModel) renderDiscoverPreview(width, height int) string {
	header := ui.SectionHeader("Top anime", "", width)

	anime := m.discoverSelected()
	if anime == nil {
		return header + "\n" + mutedLine("Nothing selected")
	}

	lines := make([]string, 0, 8)
	lines = append(lines, header)

	if artRows := discoverArtworkRows(anime, width, height); artRows > 0 {
		lines = append(lines, m.renderArtwork(m.discoverArtMeta(anime), width, artRows))
		lines = append(lines, "")
	}
	lines = append(lines, m.discoverPreviewText(anime, width)...)

	return strings.Join(fitRows(lines, height), "\n")
}

// discoverArtMeta builds the artwork input of the selected entry.
//
// The metadata panel is the shape the renderer speaks; a discover entry
// fills only what the preview needs: the title and the picture.
func (m *SearchModel) discoverArtMeta(a *source.Anime) *MetadataPanel {
	return &MetadataPanel{
		Title:      a.Name,
		Cover:      a.Cover,
		CoverImage: m.discoverCoverValue(a),
		CoverWidth: m.discoverCoverW,
		CoverRows:  m.discoverCoverR,
	}
}

// discoverCoverValue returns the rendered artwork of an entry.
//
// The cached picture counts only when it belongs to the entry on screen:
// a download that came back after the cursor moved is dropped.
func (m *SearchModel) discoverCoverValue(a *source.Anime) string {
	if a.Cover == "" || a.Cover != m.discoverURL {
		return ""
	}
	return m.discoverCover
}

// discoverPreviewText returns the text of the preview: the title, the
// facts and the score of the entry.
func (m *SearchModel) discoverPreviewText(a *source.Anime, width int) []string {
	lines := make([]string, 0, 4)

	lines = append(lines, splitLines(wrapText(a.Name, width, theme.Title.In(theme.Primary)))...)
	lines = append(lines, wrapText(discoverFactsLine(a), width, theme.Label.In(theme.Text)))
	if a.Score > 0 {
		score := fmt.Sprintf("Score %.1f", float64(a.Score))
		if a.Rank > 0 {
			score += fmt.Sprintf(" · Rank #%d", a.Rank)
		}
		lines = append(lines, wrapText(score, width, theme.Label.Style()))
	}
	return lines
}

// discoverArtworkRows returns the number of rows the artwork of the
// preview takes, exactly like the watchlist computes it.
func discoverArtworkRows(a *source.Anime, width, height int) int {
	if a.Cover == "" {
		return 0
	}

	text := 3                 // title, facts, score
	rows := height - text - 2 // the header and the blank line under it
	if max := (width - 2) * 3 / 2; rows > max {
		rows = max
	}
	if rows > 20 {
		rows = 20
	}
	if rows > height-4 {
		rows = height - 4
	}
	if rows < 6 {
		rows = 6
	}
	if rows < 0 {
		rows = 0
	}
	return rows
}

// discoverCoverCmd renders the artwork of the entry under the cursor,
// the same way the watchlist renders its covers.
func (m *SearchModel) discoverCoverCmd() tea.Cmd {
	anime := m.discoverSelected()
	if anime == nil || anime.Cover == "" {
		m.discoverResetCover()
		return nil
	}

	width, rows := m.coverTarget()
	if width < 4 || rows < 3 {
		m.discoverResetCover()
		return nil
	}

	inner := width - 2
	innerRows := rows - 2
	if m.discoverCover != "" && m.discoverURL == anime.Cover &&
		m.discoverCoverW == inner && m.discoverCoverR == innerRows {
		return nil
	}

	m.discoverResetCover()
	m.discoverURL = anime.Cover
	return downloadCoverCmd(anime.Cover, inner, innerRows, discoverCoverIndex)
}

// discoverResetCover forgets the artwork of the previous screen.
func (m *SearchModel) discoverResetCover() {
	m.discoverCover = ""
	m.discoverCoverW = 0
	m.discoverCoverR = 0
	m.discoverURL = ""
}
