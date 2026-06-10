package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
	"github.com/hishantik/anilix/source"
)

const (
	// Fixed card dimensions — every card uses exactly these.
	cardHeight = 5 // lines: title + rating + genre + 2 blank rows
)

// HomeItem represents a single anime on the home screen.
type HomeItem struct {
	Name         string
	MALID        int
	AniListID    int
	AllAnimeID   string
	Cover        string
	Score        float64
	Type         string
	Genres       []string
	Year         int
	EpisodeCount int
	Episode      string // last watched episode (for Continue Watching)
	Source       string // "history", "trending", "popular", or genre name
}

// HomeSection represents a row of anime on the home screen.
type HomeSection struct {
	Title    string
	Items    []HomeItem
	Selected int
	Loading  bool
	Err      error
}

// HomeState holds the state for the home screen.
type HomeState struct {
	Sections      []*HomeSection
	ActiveSection int
}

// NewHomeState creates a new home state with default sections.
func NewHomeState() *HomeState {
	genres := []string{"Action", "Romance", "Comedy", "Fantasy"}
	sections := make([]*HomeSection, 0, 3+len(genres))

	sections = append(sections,
		&HomeSection{Title: "Continue Watching", Loading: true},
		&HomeSection{Title: "Trending Now", Loading: true},
		&HomeSection{Title: "Popular Anime", Loading: true},
	)

	for _, g := range genres {
		sections = append(sections, &HomeSection{
			Title:   g + " Anime",
			Loading: true,
		})
	}

	return &HomeState{
		Sections:      sections,
		ActiveSection: 0,
	}
}

// viewHomeState renders the home screen.
func (m *SearchModel) viewHomeState() string {
	if m.homeScreen == nil {
		return "Loading home..."
	}

	hs := m.homeScreen

	// Available space (chrome: title + blank + help = ~6 lines)
	availHeight := m.height - 6
	if availHeight < 4 {
		availHeight = 4
	}

	// Card dimensions — full width, no panel
	gridW := m.width
	cardWidth := 26
	cardsPerRow := (gridW - 4) / cardWidth
	if cardsPerRow < 1 {
		cardsPerRow = 1
	}
	if cardsPerRow > 6 {
		cardsPerRow = 6
	}
	cardWidth = (gridW - 4) / cardsPerRow

	var lines []string
	linesUsed := 0

	for si, section := range hs.Sections {
		if linesUsed >= availHeight {
			break
		}

		// Section header
		isActive := si == hs.ActiveSection
		var header string
		if isActive {
			titlePart := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(section.Title)
			counter := ""
			if len(section.Items) > 0 {
				counter = lipgloss.NewStyle().Foreground(Theme.Faint).Render(
					fmt.Sprintf(" \u2022 %d/%d", section.Selected+1, len(section.Items)))
			}
			header = lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("\u2501\u2501 ") +
				titlePart + counter +
				lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(" \u2501\u2501")
		} else {
			header = lipgloss.NewStyle().Foreground(Theme.Border).Render("\u2500\u2500 ") +
				lipgloss.NewStyle().Foreground(Theme.Faint).Render(section.Title) +
				lipgloss.NewStyle().Foreground(Theme.Border).Render(" \u2500\u2500")
		}
		lines = append(lines, header)
		linesUsed++

		// Section content
		if section.Loading {
			loadingMsg := m.loading.View() + " Loading..."
			lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("   "+loadingMsg))
			linesUsed++
		} else if section.Err != nil {
			lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("   Failed to load"))
			linesUsed++
		} else if len(section.Items) == 0 {
			emptyMsg := "   No items"
			if section.Title == "Continue Watching" {
				emptyMsg = "   No watch history yet"
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(emptyMsg))
			linesUsed++
		} else if section.Title == "Continue Watching" {
			cardLine := m.renderContinueCardRow(section, isActive, cardWidth, cardsPerRow)
			lines = append(lines, cardLine)
			linesUsed++
		} else {
			cardLine := m.renderHomeCardRow(section, isActive, cardWidth, cardsPerRow)
			lines = append(lines, cardLine)
			linesUsed++
		}

		// Blank separator between sections
		lines = append(lines, "")
		linesUsed++
	}

	return strings.Join(lines, "\n")
}

// renderContinueCardRow renders a horizontal row of Continue Watching cards.
func (m *SearchModel) renderContinueCardRow(section *HomeSection, isActive bool, cardWidth, maxCards int) string {
	start := 0
	if section.Selected >= maxCards {
		start = section.Selected - maxCards + 1
	}
	end := len(section.Items)
	if end-start > maxCards {
		end = start + maxCards
	}
	visibleItems := section.Items[start:end]

	var cards []string
	for i, item := range visibleItems {
		globalIdx := start + i
		isItemSelected := isActive && globalIdx == section.Selected
		card := m.renderContinueCard(item, cardWidth, isItemSelected)
		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// renderContinueCard renders a Continue Watching card with title, episode, and progress bar.
func (m *SearchModel) renderContinueCard(item HomeItem, width int, selected bool) string {
	innerW := width - 2
	if innerW < 10 {
		innerW = 10
	}
	titleMax := innerW - 2
	if titleMax < 1 {
		titleMax = 1
	}

	accentWidth := 0
	if selected {
		accentWidth = 2
	}
	textMax := titleMax - accentWidth
	if textMax < 1 {
		textMax = 1
	}

	// Line 1: Title (bright cyan if selected)
	title := trimAnimeName(item.Name, textMax)
	titleStyle := lipgloss.NewStyle().Foreground(Theme.Text).Width(textMax)
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Width(textMax)
	}
	titleLine := titleStyle.Render(title)

	// Line 2: Episode label
	epLabel := "Episode " + item.Episode
	epLine := lipgloss.NewStyle().Foreground(Theme.Faint).Width(textMax).Render(trimAnimeName(epLabel, textMax))

	// Line 3: Progress bar
	var progress float64
	epNum := parseEpisodeNum(item.Episode)
	if item.EpisodeCount > 0 && epNum > 0 {
		progress = float64(epNum) / float64(item.EpisodeCount)
		if progress > 1 {
			progress = 1
		}
	}
	barLine := progressBar(progress, textMax)

	// Build content — selected cards get a left accent bar
	var content string
	if selected {
		accent := lipgloss.NewStyle().Foreground(Theme.Primary).Render("\u2502 ")
		content = accent + titleLine + "\n" + accent + epLine + "\n" + accent + barLine
	} else {
		content = titleLine + "\n" + epLine + "\n" + barLine
	}

	if selected {
		return gradientPopupBox(content, width, 1)
	}
	return solidCardBox(content, width, 1, Theme.Border)
}

// parseEpisodeNum extracts the episode number from strings like "14", "ep 14", etc.
func parseEpisodeNum(ep string) int {
	ep = strings.TrimSpace(ep)
	// Handle "ep " or "episode " prefix
	for _, prefix := range []string{"ep ", "episode ", "Ep ", "Episode "} {
		if strings.HasPrefix(ep, prefix) {
			ep = ep[len(prefix):]
			break
		}
	}
	n, _ := strconv.Atoi(ep)
	return n
}

// renderHomeCardRow renders a horizontal row of cards for a section.
func (m *SearchModel) renderHomeCardRow(section *HomeSection, isActive bool, cardWidth, maxCards int) string {
	// Determine which slice of items to show
	start := 0
	if section.Selected >= maxCards {
		start = section.Selected - maxCards + 1
	}
	end := len(section.Items)
	if end-start > maxCards {
		end = start + maxCards
	}
	visibleItems := section.Items[start:end]

	var cards []string
	for i, item := range visibleItems {
		globalIdx := start + i
		isItemSelected := isActive && globalIdx == section.Selected
		card := m.renderHomeCard(item, cardWidth, isItemSelected)
		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// renderHomeCard renders a single anime card with fixed dimensions.
// Both focused and unfocused cards use the same box function for identical geometry.
// Focused cards get a left accent bar (│) that consumes 2 chars from the text area.
func (m *SearchModel) renderHomeCard(item HomeItem, width int, selected bool) string {
	// Content inner width: gradientPopupBox subtracts 2 for border chars.
	innerW := width - 2
	if innerW < 10 {
		innerW = 10
	}
	// Subtract 2 for horizontal padding (1 each side).
	titleMax := innerW - 2
	if titleMax < 1 {
		titleMax = 1
	}

	// For selected cards, reserve 2 chars for the accent bar + gap (│ )
	accentWidth := 0
	if selected {
		accentWidth = 2
	}
	textMax := titleMax - accentWidth
	if textMax < 1 {
		textMax = 1
	}

	// --- Always render exactly 3 content lines ---
	title := trimAnimeName(item.Name, textMax)
	titleStyle := lipgloss.NewStyle().Foreground(Theme.Text).Width(textMax)
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Width(textMax)
	}
	titleLine := titleStyle.Render(title)

	// Rating + type (single line, truncated if needed)
	var metaParts []string
	if item.Score > 0 {
		metaParts = append(metaParts, fmt.Sprintf("\u2605 %.1f", item.Score))
	}
	if item.Type != "" {
		metaParts = append(metaParts, item.Type)
	}
	metaPlain := strings.Join(metaParts, " \u2022 ")
	metaPlain = trimAnimeName(metaPlain, textMax)
	// Now apply colors to the truncated plain text.
	var metaRendered string
	if item.Score > 0 && item.Type != "" {
		parts := strings.SplitN(metaPlain, " \u2022 ", 2)
		metaRendered = lipgloss.NewStyle().Foreground(Theme.Warning).Render(parts[0]) +
			lipgloss.NewStyle().Foreground(Theme.Border).Render(" \u2022 ") +
			lipgloss.NewStyle().Foreground(Theme.Faint).Render(parts[1])
	} else if item.Score > 0 {
		metaRendered = lipgloss.NewStyle().Foreground(Theme.Warning).Render(metaPlain)
	} else if item.Type != "" {
		metaRendered = lipgloss.NewStyle().Foreground(Theme.Faint).Render(metaPlain)
	}
	ratingLine := lipgloss.NewStyle().Width(textMax).Render(metaRendered)

	// Genres (first 2, single line, truncated if needed)
	genreLine := ""
	if len(item.Genres) > 0 {
		g := item.Genres
		if len(g) > 2 {
			g = g[:2]
		}
		genreLine = trimAnimeName(strings.Join(g, ", "), textMax)
	} else if item.Year > 0 {
		genreLine = trimAnimeName(fmt.Sprintf("%d", item.Year), textMax)
	}
	genresRender := lipgloss.NewStyle().Foreground(Theme.Faint).Width(textMax).Render(genreLine)

	// Build content — selected cards get a left accent bar in primary color.
	var content string
	if selected {
		accent := lipgloss.NewStyle().Foreground(Theme.Primary).Render("\u2502 ")
		content = accent + titleLine + "\n" + accent + ratingLine + "\n" + accent + genresRender
	} else {
		content = titleLine + "\n" + ratingLine + "\n" + genresRender
	}

	if selected {
		return gradientPopupBox(content, width, 1)
	}
	// Unfocused: solid border, same geometry as focused card.
	return solidCardBox(content, width, 1, Theme.Border)
}

// selectHomeItem handles selecting an anime from the home screen.
func (m *SearchModel) selectHomeItem() tea.Cmd {
	sec := m.homeScreen.Sections[m.homeScreen.ActiveSection]
	if sec.Selected >= len(sec.Items) {
		return nil
	}
	item := sec.Items[sec.Selected]

	// Build a source.Anime from the HomeItem
	anime := &source.Anime{
		Name:         item.Name,
		MALID:        item.MALID,
		AniListID:    item.AniListID,
		AllAnimeID:   item.AllAnimeID,
		Cover:        item.Cover,
		Score:        item.Score,
		Type:         item.Type,
		Genres:       item.Genres,
		Year:         item.Year,
		EpisodeCount: item.EpisodeCount,
	}

	// Set up the search state with this anime
	m.searchState.Results = []*source.Anime{anime}
	m.searchState.Selected = 0
	m.searchState.Metadata = nil

	// Transition to detail state
	m.prevState = m.state
	m.state = detailState
	m.textInput.Placeholder = "Search episode..."
	m.episodeState.Loading = true
	m.progressPercent = 0
	m.progressTarget = 0.4

	var cmds []tea.Cmd

	// Fetch metadata for the detail view (cover, title, synopsis, etc.)
	m.searchState.MetadataLoading = true
	cmds = append(cmds, m.fetchMetadata())

	// If we have AllAnimeID, go directly to episodes
	if anime.AllAnimeID != "" {
		m.episodeState.AnimeID = anime.AllAnimeID
		cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
		if anime.AniListID > 0 {
			cmds = append(cmds, m.fetchTrackingStatusCmd(anime.AniListID))
		}
	} else {
		// Need to resolve AllAnimeID first
		cmds = append(cmds, resolveHomeItemAllAnimeID(anime, m.allanimeClient, m.searchState.TranslationType))
	}

	cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return progressTickMsg{}
	}))

	return tea.Batch(cmds...)
}

