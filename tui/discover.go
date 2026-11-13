package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// The discovery screen.
//
// The screen is the search field, the results and - when the terminal has
// the room - a preview of the selected entry. The compact layout drops the
// preview column and opens the same content on a screen of its own, so a
// phone gets the details of an entry without a second column.
//
// Nothing here talks to a provider: the results, the metadata and the
// cover arrive through the messages of the model, exactly as they did
// before the screen was redesigned.

// The placeholders of the text field.
//
// The field changes its prompt with the screen it belongs to: the search
// field searches, the episode field filters the episodes.
const (
	placeholderSearch  = "Search anime..."
	placeholderEpisode = "Search episode..."
)

// pushState opens a screen and remembers the one behind it.
func (m *SearchModel) pushState(state tuiState) {
	m.backStack = append(m.backStack, m.state)
	m.state = state
}

// popState returns to the screen the user came from.
//
// The stack is empty at the root of the application: there is nowhere to go
// back to, and the caller decides what to do then.
func (m *SearchModel) popState() bool {
	if len(m.backStack) == 0 {
		return false
	}
	m.state = m.backStack[len(m.backStack)-1]
	m.backStack = m.backStack[:len(m.backStack)-1]
	return true
}

// goBack returns to the previous view.
//
// The walk is the same on every terminal: the results, the details of an
// entry, its episodes. The escape key always goes back one step.
func (m *SearchModel) goBack() tea.Cmd {
	previous := m.state

	if !m.popState() {
		m.state = searchState
	}
	if previous == detailState {
		m.episodeState = NewEpisodeState()
	}

	m.textInput.Blur()
	m.textInput.Placeholder = placeholderSearch

	// Coming back from the episodes, the tracking state was reset with
	// the episode state: the details screen asks for it again so its
	// AniList row keeps telling the truth.
	if m.state == animeState {
		return tea.Batch(m.clearCoverCmd(), m.coverCmd(), m.trackingStatusCmd())
	}

	return tea.Batch(m.clearCoverCmd(), m.coverCmd())
}

// trackingStatusCmd fetches the AniList progress of the selected entry.
//
// It is a no-op without tracking or without an AniList id: the details
// screen simply shows no AniList row then.
func (m *SearchModel) trackingStatusCmd() tea.Cmd {
	if !m.trackingEnabled {
		return nil
	}
	anime := m.GetSelectedAnime()
	if anime == nil {
		return nil
	}
	return m.fetchTrackingStatusCmd(anime.AniListID)
}

// openSelectedAnime opens the details of the selected result.
//
// The details are a screen of their own: a compact terminal has no room
// for a preview column, and the walk from a result to its episodes stays
// the same on every terminal.
func (m *SearchModel) openSelectedAnime() tea.Cmd {
	if m.GetSelectedAnime() == nil {
		return nil
	}

	m.searchState.Selected = m.searchList.Index()
	m.pushState(animeState)
	m.textInput.Blur()
	m.textInput.Placeholder = placeholderSearch
	m.animeScroll = 0

	if _, known := m.metadataCache[m.searchState.Selected]; !known {
		// The metadata of this entry is not known yet.
		m.searchState.MetadataLoading = true
		return tea.Batch(m.clearCoverCmd(), m.fetchMetadata(), m.coverCmd(), m.trackingStatusCmd())
	}

	return tea.Batch(m.clearCoverCmd(), m.coverCmd(), m.trackingStatusCmd())
}

// openEpisodes opens the episode list of the selected anime.
func (m *SearchModel) openEpisodes() tea.Cmd {
	anime := m.GetSelectedAnime()
	if anime == nil || anime.ShowID == "" {
		return nil
	}

	m.pushState(detailState)
	m.textInput.Placeholder = placeholderEpisode
	m.episodeState.AnimeID = anime.ShowID
	m.episodeState.Loading = true
	m.progressPercent = 0
	m.progressTarget = 0.4

	return tea.Batch(
		m.clearCoverCmd(),
		m.fetchEpisodes(anime),
		m.fetchTrackingStatusCmd(anime.AniListID),
		tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
			return progressTickMsg{}
		}),
		m.coverCmd(),
	)
}

// coverCmd renders the cover of the selected entry for the current screen.
//
// The cover is rendered for the size of the panel it lands in, so a change
// of screen can ask for a new one.
func (m *SearchModel) coverCmd() tea.Cmd {
	if m.searchState.Selected >= len(m.searchState.Results) {
		return nil
	}
	meta := m.searchState.Metadata
	if meta == nil {
		return nil
	}
	return m.ensureCover(meta, m.searchState.Selected)
}

// renderDiscovery renders the result list and, when the terminal is wide
// enough, the preview of the selected entry.
func (m *SearchModel) renderDiscovery() string {
	listW, previewW, height, isNarrow := searchLayout(m.contentWidth, m.contentHeight)

	if isNarrow || previewW < 20 {
		// A narrow terminal has no room for a side panel: the results take
		// the whole width and the details get a screen of their own.
		return m.renderResultPanel(listW, height)
	}

	return joinPanels(
		m.renderResultPanel(listW, height),
		listW,
		m.renderPreviewPanel(previewW, height),
		previewW,
	)
}

// renderResultPanel renders the list of the results.
//
// The panel has a header with the counter of the list and the list itself.
// The list keeps its own scrolling: the design system decides how a row
// looks, the widget decides how the rows move.
func (m *SearchModel) renderResultPanel(width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	header := ui.SectionHeader("Results", m.resultCounter(), width)

	listHeight := height - 1
	if listHeight < 1 {
		listHeight = 1
	}
	m.searchList.SetSize(width, listHeight)

	return header + "\n" + m.searchList.View()
}

// resultCounter returns the position of the cursor in the list.
//
// It tells the user how far they are in the results: a list that scrolls
// without a counter gives no clue how much is left.
func (m *SearchModel) resultCounter() string {
	total := len(m.searchState.Results)
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", m.searchState.Selected+1, total)
}

// renderPreviewPanel renders the preview of the selected entry.
//
// The panel is the compact version of the details screen: the cover, the
// title, the alternative title, the facts and the tags. The synopsis is
// left to the details screen, which has the room for it.
func (m *SearchModel) renderPreviewPanel(width, height int) string {
	meta := m.searchState.Metadata

	header := ui.SectionHeader("Now viewing", "", width)

	if meta == nil || meta.Title == "" {
		body := mutedLine("No metadata loaded")
		if m.searchState.MetadataLoading {
			body = m.loadingLine("Loading metadata...")
		}
		return header + "\n" + body
	}

	lines := make([]string, 0, 8)
	lines = append(lines, header)

	if artRows := m.previewArtworkRows(width, height); artRows > 0 {
		lines = append(lines, m.renderArtwork(meta, width, artRows))
		lines = append(lines, "")
	}
	lines = append(lines, m.previewText(meta, width)...)

	return strings.Join(fitRows(lines, height), "\n")
}

// previewArtworkRows returns the number of rows the cover of the preview
// panel takes.
//
// The artwork gets the room the text does not need, and never more than the
// shape of a poster: a cover stretched over the whole panel would push the
// metadata out of the screen.
func (m *SearchModel) previewArtworkRows(width, height int) int {
	text := len(m.previewText(m.searchState.Metadata, width))

	rows := height - text - 2 // the header and the blank line under it
	if max := (width - 2) * 3 / 2; rows > max {
		rows = max
	}
	if rows > 20 {
		// A poster is tall, but the preview is a preview: the metadata of
		// the entry matters more than the size of its picture.
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

// previewText returns the text of the preview panel: the title, the
// alternative title, the facts, the score and the tags of the entry.
func (m *SearchModel) previewText(meta *MetadataPanel, width int) []string {
	if meta == nil {
		return nil
	}

	lines := make([]string, 0, 8)

	lines = append(lines, splitLines(wrapText(meta.Title, width, theme.Title.In(theme.Primary)))...)
	if alt := alternativeTitle(meta); alt != "" {
		lines = append(lines, splitLines(wrapText(alt, width, theme.Label.Style()))...)
	}
	if facts := factsLine(meta); facts != "" {
		lines = append(lines, wrapText(facts, width, theme.Label.Style()))
	}
	if meta.Score > 0 {
		score := ui.Rating(meta.Score)
		if rank := rankLine(meta); rank != "" {
			score += "  " + theme.Caption.Render(rank)
		}
		lines = append(lines, score)
	}
	if len(meta.Genres) > 0 {
		lines = append(lines, ui.ChipRow(meta.Genres, width, theme.SecondaryContainer)...)
	}
	if meta.Source != "" {
		lines = append(lines, theme.Caption.Render("Source: "+meta.Source))
	}

	return lines
}

// viewAnimeState renders the details of the selected anime.
//
// It is the screen the compact layout opens: a phone has no room for a
// second column, so the entry gets a screen of its own. On a wide terminal
// the same content is the preview panel of the discovery screen.
func (m *SearchModel) viewAnimeState() string {
	meta := m.searchState.Metadata
	if meta == nil || meta.Title == "" {
		return ui.EmptyState{
			Icon:  theme.Icon(theme.IconInfo),
			Title: "No details for this entry",
			Hint:  "Pick a result and press enter to see it here.",
		}.Render(m.contentWidth)
	}

	artW, artRows := m.animeArtworkBox()
	textW := m.contentWidth - artW - ui.PanelGutter()

	if artW == 0 || textW < 34 {
		// A single column: the cover on top of the details.
		return m.renderAnimeSingleColumn(meta, m.contentWidth, m.contentHeight)
	}

	left := m.renderArtwork(meta, artW, artRows)
	right := strings.Join(m.scrollLines(m.animeText(meta, textW), m.contentHeight), "\n")

	return joinPanels(left, artW, right, textW)
}

// animeArtworkBox returns the size of the cover of the details screen.
//
// On a wide terminal the cover is a column of its own; on a compact one it
// sits on top of the details, centred in the column.
func (m *SearchModel) animeArtworkBox() (width, rows int) {
	if ui.BreakpointOf(m.contentWidth).IsTwoColumn() && m.contentWidth >= 70 {
		width = 30
	} else {
		width = m.contentWidth
		if width > 28 {
			width = 28
		}
	}

	rows = (width - 2) * 3 / 2
	if max := m.contentHeight - 8; rows > max {
		rows = max
	}
	if rows > 20 {
		rows = 20
	}
	if rows < 6 {
		rows = 6
	}
	return width, rows
}

// renderAnimeSingleColumn renders the details of an entry in one column.
//
// The cover comes first: it is the thing the eye looks for, and it is the
// part that needs the width.
func (m *SearchModel) renderAnimeSingleColumn(meta *MetadataPanel, width, height int) string {
	artW, artRows := m.animeArtworkBox()

	lines := make([]string, 0, 8)
	if art := m.renderArtwork(meta, artW, artRows); art != "" {
		lines = append(lines, splitLines(centreBlock(art, width))...)
		lines = append(lines, "")
	}

	room := height - len(lines)
	if room < 1 {
		room = 1
	}
	lines = append(lines, m.scrollLines(m.animeText(meta, width), room)...)

	return strings.Join(fitRows(lines, height), "\n")
}

// scrollLines applies the scroll offset of the details screen.
//
// A compact terminal cannot show the whole entry at once, and the details
// screen has no list to scroll: the arrow keys move the text.
func (m *SearchModel) scrollLines(lines []string, height int) []string {
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}

	scroll := m.animeScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll == 0 {
		return fitRows(lines, height)
	}
	return fitRows(lines[scroll:], height)
}

// animeText returns the text of the details screen: the title, the facts,
// the metadata table, the tags and the synopsis.
func (m *SearchModel) animeText(meta *MetadataPanel, width int) []string {
	if meta == nil {
		return nil
	}

	lines := make([]string, 0, 16)

	lines = append(lines, splitLines(wrapText(meta.Title, width, theme.Headline.In(theme.Primary)))...)
	if alt := alternativeTitle(meta); alt != "" {
		lines = append(lines, splitLines(wrapText(alt, width, theme.Label.Style()))...)
	}
	if facts := factsLine(meta); facts != "" {
		lines = append(lines, wrapText(facts, width, theme.Label.Style()))
	}

	if meta.Score > 0 {
		lines = append(lines, "")
		score := ui.Rating(meta.Score)
		if rank := rankLine(meta); rank != "" {
			score += "  " + theme.Caption.Render(rank)
		}
		lines = append(lines, score)
	}

	if facts := metaFacts(meta); len(facts) > 0 {
		lines = append(lines, "")
		lines = append(lines, facts...)
	}

	if row := m.anilistFactRow(); row != "" {
		lines = append(lines, "")
		lines = append(lines, row)
	}

	if len(meta.Genres) > 0 {
		lines = append(lines, "")
		lines = append(lines, ui.ChipRow(meta.Genres, width, theme.SecondaryContainer)...)
	}

	if meta.Synopsis != "" {
		lines = append(lines, "")
		lines = append(lines, ui.Divider(false, width))
		lines = append(lines, "")
		lines = append(lines, splitLines(wrapText(stripHTML(meta.Synopsis), width, theme.Body.Style()))...)
	}

	if meta.Source != "" {
		lines = append(lines, "")
		lines = append(lines, theme.Caption.Render("Source: "+meta.Source))
	}

	return lines
}

// alternativeTitle returns the alternative title of an anime.
//
// The native title comes first: it is the one the series is known by in its
// own country, and the one that tells two entries with the same name apart.
func alternativeTitle(meta *MetadataPanel) string {
	if meta == nil {
		return ""
	}
	for _, candidate := range []string{meta.TitleNative, meta.TitleEnglish} {
		if candidate != "" && candidate != meta.Title {
			return candidate
		}
	}
	return ""
}

// factsLine returns the compact facts of an anime: the year, the format,
// the number of episodes and the status.
func factsLine(meta *MetadataPanel) string {
	if meta == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if meta.Year > 0 {
		parts = append(parts, strconv.Itoa(meta.Year))
	}
	if meta.Type != "" {
		parts = append(parts, meta.Type)
	}
	if meta.Episodes > 0 {
		parts = append(parts, fmt.Sprintf("%d eps", meta.Episodes))
	}
	if meta.Status != "" {
		parts = append(parts, meta.Status)
	}
	return strings.Join(parts, " · ")
}

// rankLine returns the rank and the popularity of an anime.
func rankLine(meta *MetadataPanel) string {
	if meta == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if meta.Rank > 0 {
		parts = append(parts, fmt.Sprintf("#%d rank", meta.Rank))
	}
	if meta.Popularity > 0 {
		parts = append(parts, fmt.Sprintf("#%d popular", meta.Popularity))
	}
	return strings.Join(parts, " · ")
}

// joinPanels puts two panels side by side with a vertical rule between them.
//
// The rule is the visual separation of the two columns: without it the text
// of the preview runs into the text of the list.
func joinPanels(left string, leftW int, right string, rightW int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	rows := len(leftLines)
	if len(rightLines) > rows {
		rows = len(rightLines)
	}

	rule := theme.Fg(lipgloss.NewStyle(), theme.OutlineVariant).Render("│")

	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		out = append(out,
			padLine(leftLines, i, leftW)+" "+rule+" "+padLine(rightLines, i, rightW))
	}
	return strings.Join(out, "\n")
}

// padLine returns the i-th line of a block, padded to the given width.
//
// A block that is shorter than the column is padded with spaces, so the
// rule between two panels stays straight.
func padLine(lines []string, i, width int) string {
	if width < 1 {
		width = 1
	}
	if i >= len(lines) {
		return strings.Repeat(" ", width)
	}
	line := lines[i]
	switch w := lipgloss.Width(line); {
	case w < width:
		return line + strings.Repeat(" ", width-w)
	case w > width:
		return lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return line
}

// centreBlock centres a block of text in the given width.
func centreBlock(block string, width int) string {
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w >= width {
			out = append(out, lipgloss.NewStyle().MaxWidth(width).Render(line))
			continue
		}
		gap := width - w
		left := gap / 2
		out = append(out, strings.Repeat(" ", left)+line+strings.Repeat(" ", gap-left))
	}
	return strings.Join(out, "\n")
}

// fitRows trims a block of lines to at most n rows.
func fitRows(lines []string, n int) []string {
	if n < 0 {
		n = 0
	}
	if len(lines) > n {
		return lines[:n]
	}
	return lines
}

// splitLines splits a block of text into its lines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
