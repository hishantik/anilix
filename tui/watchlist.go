package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// The watchlist.
//
// The saved anime of the account. Anilix keeps no list of its own: the
// list the screen shows is the one AniList holds for the user, and the
// way an entry grows is the one that exists today - a played episode
// saves its progress, and the episode numbers climb from there. The
// screen only reads; saving and removing keep their existing paths.
//
// Because the list is fetched, the screen has the three phases a remote
// screen has: loading, error and content. An account that is not
// connected gets the honest empty state instead of a spinner.

// watchlistLayout computes the columns of the watchlist for a terminal.
//
// The widths come from the layout tier, like everywhere else: a compact
// terminal gets a single column and no artwork panel; a wider one gets
// the list and the preview of the selected entry.
func watchlistLayout(totalWidth, totalHeight int) (listW, previewW, availableH int, isNarrow bool) {
	availableH = totalHeight - 2
	if availableH < 4 {
		availableH = 4
	}

	previewW = ui.PreviewWidth(totalWidth)
	if previewW == 0 {
		w := totalWidth - 2
		if w < 20 {
			w = 20
		}
		return w, 0, availableH, true
	}

	listW = ui.ListWidth(totalWidth)
	return listW, previewW, availableH, false
}

// gotoWatchlist opens the watchlist screen.
//
// A connected account starts a fetch; an account that is not connected
// gets the screen with the empty state at once: it does not pretend to
// load something it cannot reach.
func (m *SearchModel) gotoWatchlist() tea.Cmd {
	if m.state != watchlistState {
		m.prevState = m.state
	}
	m.state = watchlistState
	m.watchlist = &WatchlistState{}
	m.watchlistResetCover()
	m.textInput.Blur()

	if auth.IsLoggedIn() {
		m.watchlist.Loading = true
		return tea.Batch(m.clearCoverCmd(), m.fetchWatchlistCmd())
	}
	return tea.Batch(m.clearCoverCmd())
}

// fetchWatchlistCmd reads the saved list of the account from AniList.
func (m *SearchModel) fetchWatchlistCmd() tea.Cmd {
	token := m.anilistToken
	if token == "" {
		return nil
	}
	client := anilist.NewAuthenticatedClient(token)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		entries, err := client.WatchlistOf(ctx)
		vals := make([]*anilist.WatchlistEntry, len(entries))
		for i := range entries {
			vals[i] = &entries[i]
		}
		return WatchlistLoadedMsg{Entries: vals, Err: err}
	}
}

// watchlistSelected returns the entry under the cursor, or nil.
func (m *SearchModel) watchlistSelected() *anilist.WatchlistEntry {
	if m.watchlist == nil || m.watchlist.Cursor < 0 || m.watchlist.Cursor >= len(m.watchlist.Entries) {
		return nil
	}
	return m.watchlist.Entries[m.watchlist.Cursor]
}

// handleWatchlistKey handles a key on the watchlist screen.
//
// Enter resumes an entry the way the history does: the provider searches
// by title, and the user lands on the results of what they saved.
func (m *SearchModel) handleWatchlistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Quit):
		m.prevState = m.state
		m.confirmSelect = 1
		m.state = confirmQuitState
		return m, nil

	case key.Matches(msg, m.keymap.Back):
		m.state = m.prevState
		return m, nil

	case key.Matches(msg, m.keymap.Up):
		if m.watchlist.Cursor > 0 {
			m.watchlist.Cursor--
			return m, m.watchlistCoverCmd()
		}
		return m, nil

	case key.Matches(msg, m.keymap.Down):
		if m.watchlist.Cursor < len(m.watchlist.Entries)-1 {
			m.watchlist.Cursor++
			return m, m.watchlistCoverCmd()
		}
		return m, nil

	case key.Matches(msg, m.keymap.Select):
		entry := m.watchlistSelected()
		if entry == nil {
			return m, nil
		}
		m.textInput.SetValue(entry.Title)
		m.lastQuery = entry.Title
		m.searchState.Query = entry.Title
		m.searchState.Loading = true
		m.searchState.Results = nil
		m.searchState.Err = nil
		m.progressPercent = 0
		m.progressTarget = 0.3
		m.textInput.Blur()
		m.state = searchState
		return m, m.doSearch(entry.Title)
	}

	return m, nil
}

// watchlistStatusLabel gives a list status its human word.
func watchlistStatusLabel(status string) string {
	switch status {
	case "CURRENT":
		return "Watching"
	case "PLANNING":
		return "Planned"
	case "COMPLETED":
		return "Completed"
	case "DROPPED":
		return "Dropped"
	case "PAUSED":
		return "Paused"
	case "REPEATING":
		return "Rewatching"
	}
	if status == "" {
		return "Saved"
	}
	return status
}

// watchlistProgressLine renders the status and the progress of an entry.
func watchlistProgressLine(e *anilist.WatchlistEntry) string {
	line := watchlistStatusLabel(e.Status)
	switch {
	case e.Progress > 0 && e.Total > 0:
		line += " · Ep " + strconv.Itoa(e.Progress) + "/" + strconv.Itoa(e.Total)
	case e.Progress > 0:
		line += " · Ep " + strconv.Itoa(e.Progress)
	}
	return line
}

// watchlistFactsLine renders the static facts of an entry: the format,
// the year, and the score.
func watchlistFactsLine(e *anilist.WatchlistEntry) string {
	facts := make([]string, 0, 3)
	if e.Format != "" {
		facts = append(facts, e.Format)
	}
	if e.Year > 0 {
		facts = append(facts, strconv.Itoa(e.Year))
	}
	if e.Score > 0 {
		facts = append(facts, fmt.Sprintf("%.1f", e.Score))
	}
	return strings.Join(facts, " · ")
}

// viewWatchlistState renders the watchlist screen.
//
// A terminal with room for one column gets the list alone; a wider one
// gets the list and the preview of the selected entry, artwork included
// where the terminal can draw it.
func (m *SearchModel) viewWatchlistState() string {
	listW, previewW, height, isNarrow := watchlistLayout(m.contentWidth, m.contentHeight)

	if isNarrow || previewW < 20 {
		return m.renderWatchlistPanel(listW, height)
	}

	return joinPanels(
		m.renderWatchlistPanel(listW, height),
		listW,
		m.renderWatchlistPreview(previewW, height),
		previewW,
	)
}

// renderWatchlistPanel renders the list panel of the watchlist, cut to
// the height it has.
//
// Every entry renders as two lines - the headline and its status line -
// so the window moves in steps of rows, and the cursor row is always on
// screen: a saved list can be long, and a phone terminal cannot hold it.
func (m *SearchModel) renderWatchlistPanel(width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	header := ui.SectionHeader("Watchlist", m.watchlistCounter(), width)

	if m.watchlist.Loading {
		return header + "\n" + m.loadingLine("Loading your list...")
	}
	if m.watchlist.Err != nil {
		return header + "\n" + ui.ErrorState{
			Title:   "Could not load your list",
			Message: m.watchlist.Err.Error(),
			Hint:    "esc to go back, and try again from the AniList screen",
		}.Render(width)
	}
	if !auth.IsLoggedIn() {
		return header + "\n" + ui.EmptyState{
			Icon:  theme.Icon(theme.IconWatchlist),
			Title: "Not connected",
			Hint:  "The watchlist is the saved list of your AniList account. Log in from the AniList screen.",
		}.Render(width)
	}
	if len(m.watchlist.Entries) == 0 {
		return header + "\n" + ui.EmptyState{
			Icon:  theme.Icon(theme.IconWatchlist),
			Title: "Nothing saved yet",
			Hint:  "Anime you mark as watching on AniList show up here.",
		}.Render(width)
	}

	// Two rows per entry: the headline and the status line under it.
	visible := (height - 2) / 2
	if visible < 1 {
		visible = 1
	}

	start := m.watchlist.Scroll
	if m.watchlist.Cursor < start {
		start = m.watchlist.Cursor
	}
	if m.watchlist.Cursor >= start+visible {
		start = m.watchlist.Cursor - visible + 1
	}
	if start < 0 {
		start = 0
	}
	m.watchlist.Scroll = start

	rows := make([]string, 0, visible*2)
	for i := start; i < len(m.watchlist.Entries) && i < start+visible; i++ {
		e := m.watchlist.Entries[i]
		rows = append(rows, ui.ListItem{
			Title:       e.Title,
			Description: watchlistProgressLine(e),
			Trailing:    ui.RatingText(e.Score),
			Selected:    i == m.watchlist.Cursor,
			Focused:     true,
		}.Render(width))
	}
	return header + "\n" + joinLinesDropEmpty(rows)
}

// joinLinesDropEmpty joins rows, skipping the empty ones.
//
// A list item can end in a blank string; a blank row in the middle of a
// list reads as a gap between entries that does not exist.
func joinLinesDropEmpty(rows []string) string {
	kept := make([]string, 0, len(rows))
	for _, r := range rows {
		if r != "" {
			kept = append(kept, r)
		}
	}
	return strings.Join(kept, "\n")
}

// watchlistCounter returns the position of the cursor in the list.
func (m *SearchModel) watchlistCounter() string {
	total := len(m.watchlist.Entries)
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", m.watchlist.Cursor+1, total)
}

// renderWatchlistPreview renders the preview of the selected entry: the
// artwork and the facts.
func (m *SearchModel) renderWatchlistPreview(width, height int) string {
	header := ui.SectionHeader("Your list", "", width)

	entry := m.watchlistSelected()
	if entry == nil {
		return header + "\n" + mutedLine("Nothing selected")
	}

	lines := make([]string, 0, 8)
	lines = append(lines, header)

	if artRows := watchlistArtworkRows(entry, width, height); artRows > 0 {
		lines = append(lines, m.renderArtwork(m.watchlistArtMeta(entry), width, artRows))
		lines = append(lines, "")
	}
	lines = append(lines, m.watchlistPreviewText(entry, width)...)

	return strings.Join(fitRows(lines, height), "\n")
}

// watchlistArtMeta builds the artwork input of the selected entry.
//
// The metadata panel is the shape the renderer speaks; a watchlist entry
// fills only what the preview needs: the title and the picture.
func (m *SearchModel) watchlistArtMeta(e *anilist.WatchlistEntry) *MetadataPanel {
	return &MetadataPanel{
		Title:      e.Title,
		Cover:      e.Cover,
		CoverImage: m.watchlistCoverValue(e),
		CoverWidth: m.watchlistCoverW,
		CoverRows:  m.watchlistCoverR,
	}
}

// watchlistCoverValue returns the rendered artwork of an entry.
//
// The cached picture counts only when it belongs to the entry on screen:
// a download that came back after the cursor moved is dropped.
func (m *SearchModel) watchlistCoverValue(e *anilist.WatchlistEntry) string {
	if e.Cover == "" || e.Cover != m.watchlistCoverURL {
		return ""
	}
	return m.watchlistCover
}

// watchlistPreviewText returns the text of the preview: the title, the
// progress and the facts of the entry.
func (m *SearchModel) watchlistPreviewText(e *anilist.WatchlistEntry, width int) []string {
	lines := make([]string, 0, 4)

	lines = append(lines, splitLines(wrapText(e.Title, width, theme.Title.In(theme.Primary)))...)
	lines = append(lines, wrapText(watchlistProgressLine(e), width, theme.Label.In(theme.Text)))
	if facts := watchlistFactsLine(e); facts != "" {
		lines = append(lines, wrapText(facts, width, theme.Label.Style()))
	}
	return lines
}

// watchlistArtworkRows returns the number of rows the artwork of the
// preview takes.
//
// The artwork gets the room the text does not need, capped to the shape
// of a poster, exactly like the preview of the search screen.
func watchlistArtworkRows(e *anilist.WatchlistEntry, width, height int) int {
	if e.Cover == "" {
		return 0
	}

	text := 3                 // title, progress, facts
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

// watchlistCoverCmd renders the artwork of the entry under the cursor.
//
// The command is issued when the cursor moves: it is free when the entry
// is already rendered at the size of the panel, and it forgets the
// picture of the previous entry before it asks for a new one.
func (m *SearchModel) watchlistCoverCmd() tea.Cmd {
	entry := m.watchlistSelected()
	if entry == nil || entry.Cover == "" {
		m.watchlistCover = ""
		m.watchlistCoverW = 0
		m.watchlistCoverR = 0
		m.watchlistCoverURL = ""
		return nil
	}

	width, rows := m.coverTarget()
	if width < 4 || rows < 3 {
		m.watchlistCover = ""
		m.watchlistCoverW = 0
		m.watchlistCoverR = 0
		return nil
	}

	inner := width - 2
	innerRows := rows - 2
	if m.watchlistCover != "" && m.watchlistCoverURL == entry.Cover &&
		m.watchlistCoverW == inner && m.watchlistCoverR == innerRows {
		return nil
	}

	m.watchlistCover = ""
	m.watchlistCoverW = 0
	m.watchlistCoverR = 0
	m.watchlistCoverURL = entry.Cover
	return downloadCoverCmd(entry.Cover, inner, innerRows, watchlistCoverIndex)
}

// watchlistResetCover forgets the artwork of the previous screen.
func (m *SearchModel) watchlistResetCover() {
	m.watchlistCover = ""
	m.watchlistCoverW = 0
	m.watchlistCoverR = 0
	m.watchlistCoverURL = ""
}

// watchlistCoverIndex is the result index the watchlist sends its cover
// downloads under.
//
// The result indexes of the search start at zero: a negative index is
// never a search result, so the load message cannot be mistaken for one.
// The cover is recognised by its URL instead of the index.
const watchlistCoverIndex = -1
