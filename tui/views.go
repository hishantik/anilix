package tui

import (
	"fmt"
	"strings"

	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/version"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *SearchModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var content string

	switch m.state {
	case homeState:
		content = m.viewHomeState()
	case searchState:
		content = m.viewSearchState()
	case detailState:
		content = m.viewDetailState()
	case confirmQuitState:
		content = m.viewConfirmQuit()
	case settingsState:
		content = m.viewSettings()
	case anilistLoginState:
		content = m.viewAniListLogin()
	}

	v := tea.NewView(m.renderChrome(content))
	v.AltScreen = true
	return v
}

func (m *SearchModel) viewSearchState() string {
	var lines []string

	// Search bar with rounded gradient border
	searchContent := m.textInput.View()
	searchBarWidth := m.width
	if searchBarWidth < 24 {
		searchBarWidth = 24
	}
	searchBar := gradientPopupBox(searchContent, searchBarWidth, 1)
	lines = append(lines, searchBar)
	lines = append(lines, "")

	if m.searchState.Loading {
		msg := lipgloss.JoinHorizontal(lipgloss.Center, m.loading.View(), " Searching...")
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(msg))
	} else if len(m.searchState.Results) > 0 {
		// Two-column layout: list on left, metadata preview on right
		leftW, rightW, _, isNarrow := searchLayout(m.width, m.height)

		listView := m.searchList.View()
		preview := m.renderMetadataPreview(rightW, m.height)

		if isNarrow || rightW < 20 {
			// Narrow: just the list, no preview
			lines = append(lines, listView)
		} else {
			// Constrain list to left panel width
			leftPanel := lipgloss.NewStyle().Width(leftW).MaxWidth(leftW).Render(listView)
			rightPanel := lipgloss.NewStyle().Width(rightW).MaxWidth(rightW).Render(preview)
			gap := strings.Repeat(" ", 2)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, gap, rightPanel))
		}
	} else if m.textInput.Value() == "" && len(m.recentSearches) > 0 {
		lines = append(lines, m.renderRecentSearches())
	} else if m.searchState.Err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Error).Render(fmt.Sprintf("Error: %v", m.searchState.Err)))
	}

	return strings.Join(lines, "\n")
}

// searchLayout computes two-column dimensions for the search results view.
func searchLayout(totalWidth, totalHeight int) (leftW, rightW, availableH int, isNarrow bool) {
	availableH = totalHeight - 4
	if availableH < 5 {
		availableH = 5
	}

	if totalWidth < 80 {
		isNarrow = true
		w := totalWidth - 4
		if w < 20 {
			w = 20
		}
		return w, 0, availableH, true
	}

	// Left panel (list) gets ~55%, right panel (preview) gets ~45%
	leftW = int(float64(totalWidth) * 0.55)
	if leftW < 35 {
		leftW = 35
	}

	rightW = totalWidth - leftW - 2 // 2 for gap
	if rightW < 30 {
		rightW = 30
	}
	if rightW > 80 {
		rightW = 80
	}

	return leftW, rightW, availableH, false
}

// renderMetadataPreview renders a metadata sidebar for the search results view.
func (m *SearchModel) renderMetadataPreview(width int, termHeight int) string {
	meta := m.searchState.Metadata

	// No metadata at all yet — show loading or empty
	if meta == nil || meta.Title == "" {
		if m.searchState.MetadataLoading {
			return lipgloss.NewStyle().Foreground(Theme.Faint).Render(m.loading.View() + " Loading metadata...")
		}
		return lipgloss.NewStyle().Foreground(Theme.Faint).Render("No metadata loaded")
	}

	var sections []string

	// Cover image — compact allocation so metadata sits close below
	if meta.CoverImage != "" {
		cover := truncateToRows(meta.CoverImage, coverMaxRows(termHeight))
		sections = append(sections, cover)
	}

	// Title
	sections = append(sections, lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Width(width).MaxWidth(width).Render(meta.Title))
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		sections = append(sections, lipgloss.NewStyle().Foreground(Theme.Faint).Width(width).MaxWidth(width).Render(meta.TitleEnglish))
	}
	if meta.TitleNative != "" {
		sections = append(sections, lipgloss.NewStyle().Foreground(Theme.Faint).Width(width).MaxWidth(width).Render(meta.TitleNative))
	}

	// Score badge
	if meta.Score > 0 {
		sections = append(sections, "")
		sections = append(sections, scoreBadge(meta.Score))
	}

	// Compact info card
	sections = append(sections, "")
	sections = append(sections, compactInfoCard(meta))

	// Stats
	var stats []string
	if meta.Rank > 0 {
		stats = append(stats, statLine("Rank", fmt.Sprintf("#%d", meta.Rank)))
	}
	if meta.Popularity > 0 {
		stats = append(stats, statLine("Popularity", fmt.Sprintf("#%d", meta.Popularity)))
	}
	if len(stats) > 0 {
		sections = append(sections, "")
		sections = append(sections, strings.Join(stats, "\n"))
	}

	// Genre tags
	if len(meta.Genres) > 0 {
		sections = append(sections, "")
		var tagLines []string
		var currentLine string
		for _, g := range meta.Genres {
			tag := genreTag(g)
			candidate := currentLine
			if candidate != "" {
				candidate += " "
			}
			candidate += tag
			if lipgloss.Width(candidate) > width && currentLine != "" {
				tagLines = append(tagLines, currentLine)
				currentLine = tag
			} else {
				currentLine = candidate
			}
		}
		if currentLine != "" {
			tagLines = append(tagLines, currentLine)
		}
		sections = append(sections, strings.Join(tagLines, "\n"))
	}

	// Gradient separator
	sections = append(sections, "")
	sections = append(sections, gradientLine(width))

	// Synopsis (shorter preview than detail view)
	if meta.Synopsis != "" {
		synopsis := stripHTML(meta.Synopsis)
		maxLen := width * 3 // ~3 lines
		if maxLen > 0 {
			synopsis = truncateSynopsis(synopsis, maxLen)
		}
		sections = append(sections, "")
		sections = append(sections, lipgloss.NewStyle().Foreground(Theme.Text).Width(width).Render(synopsis))
	}

	// Source
	if meta.Source != "" {
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render("Source: "+meta.Source))
	}

	content := strings.Join(sections, "\n")
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(content)
}

func (m *SearchModel) viewDetailState() string {
	meta := m.searchState.Metadata
	if meta == nil {
		meta = &MetadataPanel{}
	}

	leftW, rightW, _, isNarrow := detailLayout(m.width, m.height)

	if isNarrow {
		return m.renderDetailSingleColumn(meta, rightW)
	}

	leftPanel := m.renderDetailLeftPanel(meta, leftW, m.height)
	rightPanel := m.renderDetailRightPanel(meta, rightW, m.height)
	gap := strings.Repeat(" ", 2)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, gap, rightPanel)
}

// detailLayout computes two-column dimensions for the detail view.
func detailLayout(totalWidth, totalHeight int) (leftW, rightW, availableH int, isNarrow bool) {
	availableH = totalHeight - 4 // reserve chrome space
	if availableH < 5 {
		availableH = 5
	}

	if totalWidth < 80 {
		isNarrow = true
		w := totalWidth - 4
		if w < 20 {
			w = 20
		}
		return 0, w, availableH, true
	}

	leftW = int(float64(totalWidth) * 0.35)
	if leftW < 28 {
		leftW = 28
	}
	if leftW > 50 {
		leftW = 50
	}

	rightW = totalWidth - leftW - 2
	if rightW < 40 {
		rightW = 40
	}
	// Cap right panel at ~100 for readability on wide terminals
	if rightW > 100 {
		rightW = 100
	}

	return leftW, rightW, availableH, false
}

// truncateToRows truncates a string to at most n lines.
func truncateToRows(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}

// coverMaxRows computes the maximum terminal rows for a cover image.
// Shared between view rendering and download commands to prevent
// Kitty image/placeholder height mismatches.
func coverMaxRows(termHeight int) int {
	n := (termHeight - 4) / 2
	if n < 8 {
		n = 8
	}
	if n > 24 {
		n = 24
	}
	return n
}

func (m *SearchModel) renderDetailLeftPanel(meta *MetadataPanel, width int, termHeight int) string {
	var sections []string

	// Cover image or placeholder — compact allocation so metadata sits close below
	if meta.CoverImage != "" {
		cover := truncateToRows(meta.CoverImage, coverMaxRows(termHeight))
		sections = append(sections, cover)
	} else {
		sections = append(sections, coverPlaceholder(meta.Title, width))
	}

	// Score badge
	if meta.Score > 0 {
		sections = append(sections, scoreBadge(meta.Score))
	}

	// Stats
	var stats []string
	if meta.Rank > 0 {
		stats = append(stats, statLine("Rank", fmt.Sprintf("#%d", meta.Rank)))
	}
	if meta.Popularity > 0 {
		stats = append(stats, statLine("Popularity", fmt.Sprintf("#%d", meta.Popularity)))
	}
	if len(stats) > 0 {
		sections = append(sections, "")
		sections = append(sections, strings.Join(stats, "\n"))
	}

	// Compact info card
	sections = append(sections, "")
	sections = append(sections, compactInfoCard(meta))

	// Genre tags — wrap to fit within panel width
	if len(meta.Genres) > 0 {
		sections = append(sections, "")
		var tagLines []string
		var currentLine string
		for _, g := range meta.Genres {
			tag := genreTag(g)
			candidate := currentLine
			if candidate != "" {
				candidate += " "
			}
			candidate += tag
			if lipgloss.Width(candidate) > width && currentLine != "" {
				tagLines = append(tagLines, currentLine)
				currentLine = tag
			} else {
				currentLine = candidate
			}
		}
		if currentLine != "" {
			tagLines = append(tagLines, currentLine)
		}
		sections = append(sections, strings.Join(tagLines, "\n"))
	}

	// Source
	if meta.Source != "" {
		sections = append(sections, "")
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render("Source: "+meta.Source))
	}

	content := strings.Join(sections, "\n")
	// Width sets minimum, MaxWidth enforces maximum — prevents overflow
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(content)
}

func (m *SearchModel) renderDetailRightPanel(meta *MetadataPanel, width, height int) string {
	var lines []string

	// Title section
	titleStyle := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true)
	lines = append(lines, titleStyle.Width(width).Render(meta.Title))
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Width(width).Render(meta.TitleEnglish))
	}
	if meta.TitleNative != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Width(width).Render(meta.TitleNative))
	}

	// Gradient separator
	lines = append(lines, "")
	lines = append(lines, gradientLine(width))

	// Synopsis
	synopsisLines := 0
	if meta.Synopsis != "" {
		synopsis := stripHTML(meta.Synopsis)
		maxLen := width * 5 // ~5 lines
		if maxLen > 0 {
			synopsis = truncateSynopsis(synopsis, maxLen)
		}
		rendered := lipgloss.NewStyle().Foreground(Theme.Text).Width(width).Render(synopsis)
		lines = append(lines, "")
		lines = append(lines, rendered)
		synopsisLines = strings.Count(rendered, "\n") + 1
	}

	// Second separator
	lines = append(lines, "")
	lines = append(lines, gradientLine(width))
	lines = append(lines, "")

	// Calculate episode list height
	fixedLines := 4 // title lines (at least 1)
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		fixedLines++
	}
	if meta.TitleNative != "" {
		fixedLines++
	}
	fixedLines += 3            // separators + blank lines
	fixedLines += synopsisLines
	fixedLines += 3            // second separator + blanks
	episodeHeight := height - fixedLines - 8 // chrome reservation
	if episodeHeight < 5 {
		episodeHeight = 5
	}

	// Set episode list size for the right panel
	m.episodeList.SetSize(width, episodeHeight)

	// Tracking status
	if m.trackingEnabled && m.episodeState.TrackingStatus != "" {
		totalEp := 0
		if m.searchState.Metadata != nil {
			totalEp = m.searchState.Metadata.Episodes
		}
		lines = append(lines, renderWatchProgress(
			m.episodeState.TrackingStatus, m.episodeState.TrackingProgress, totalEp, width))
	} else if m.trackingEnabled {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("Not in your AniList"))
	}

	// Episode section header + list
	epCount := len(m.episodeState.Episodes)
	epHeader := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(
		fmt.Sprintf("\u2501\u2501 Episodes (%d) \u2501\u2501", epCount))

	if m.episodeState.Loading {
		msg := lipgloss.JoinHorizontal(lipgloss.Center, m.loading.View(), " Loading episodes...")
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(msg))
	} else if m.episodeState.Playing {
		selectedAnime := m.searchState.Results[m.searchState.Selected]
		epNum := ""
		if m.episodeState.Selected < len(m.episodeState.Episodes) {
			epNum = m.episodeState.Episodes[m.episodeState.Selected]
		}
		playMsg := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(
			fmt.Sprintf("Playing: %s - Episode %s", selectedAnime.Name, epNum))
		lines = append(lines, playMsg)
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("Launching player..."))
	} else if epCount > 0 {
		lines = append(lines, epHeader)
		lines = append(lines, m.episodeList.View())
	} else if m.episodeState.Err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Error).Render(fmt.Sprintf("Error: %v", m.episodeState.Err)))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("No episodes found"))
	}

	// Width + MaxWidth on each line to prevent overflow
	joined := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(joined)
}

// renderDetailSingleColumn renders the detail view as a single column for narrow terminals.
func (m *SearchModel) renderDetailSingleColumn(meta *MetadataPanel, width int) string {
	var lines []string

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true)
	lines = append(lines, titleStyle.Render(meta.Title))
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(meta.TitleEnglish))
	}
	if meta.TitleNative != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(meta.TitleNative))
	}

	// Compact info card
	lines = append(lines, compactInfoCard(meta))

	// Genres
	if len(meta.Genres) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(strings.Join(meta.Genres, ", ")))
	}

	// Score, Rank
	var scoreParts []string
	if meta.Score > 0 {
		scoreParts = append(scoreParts, fmt.Sprintf("Score: %.2f", meta.Score))
	}
	if meta.Rank > 0 {
		scoreParts = append(scoreParts, fmt.Sprintf("Rank: #%d", meta.Rank))
	}
	if meta.Popularity > 0 {
		scoreParts = append(scoreParts, fmt.Sprintf("Popularity: #%d", meta.Popularity))
	}
	if len(scoreParts) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(strings.Join(scoreParts, "  \u00b7  ")))
	}

	// Source
	if meta.Source != "" {
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Source: "+meta.Source))
	}

	// Synopsis
	if meta.Synopsis != "" {
		synopsis := stripHTML(meta.Synopsis)
		maxLen := width * 3
		if maxLen > 0 {
			synopsis = truncateSynopsis(synopsis, maxLen)
		}
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Text).Width(width).Render(synopsis))
	}

	// Separator
	sepWidth := width
	if sepWidth < 10 {
		sepWidth = 10
	}
	lines = append(lines, "")
	lines = append(lines, gradientLine(sepWidth))

	// Tracking status (single-column)
	if m.trackingEnabled && m.episodeState.TrackingStatus != "" {
		totalEp := 0
		if m.searchState.Metadata != nil {
			totalEp = m.searchState.Metadata.Episodes
		}
		lines = append(lines, renderWatchProgress(
			m.episodeState.TrackingStatus, m.episodeState.TrackingProgress, totalEp, width))
	} else if m.trackingEnabled {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("Not in your AniList"))
	}

	// Episodes
	epCount := len(m.episodeState.Episodes)
	epHeader := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(
		fmt.Sprintf("\u2501\u2501 Episodes (%d) \u2501\u2501", epCount))

	if m.episodeState.Loading {
		msg := lipgloss.JoinHorizontal(lipgloss.Center, m.loading.View(), " Loading episodes...")
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(msg))
	} else if m.episodeState.Playing {
		selectedAnime := m.searchState.Results[m.searchState.Selected]
		epNum := ""
		if m.episodeState.Selected < len(m.episodeState.Episodes) {
			epNum = m.episodeState.Episodes[m.episodeState.Selected]
		}
		playMsg := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(
			fmt.Sprintf("Playing: %s - Episode %s", selectedAnime.Name, epNum))
		lines = append(lines, playMsg)
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("Launching player..."))
	} else if epCount > 0 {
		lines = append(lines, epHeader)
		lines = append(lines, m.episodeList.View())
	} else if m.episodeState.Err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Error).Render(fmt.Sprintf("Error: %v", m.episodeState.Err)))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render("No episodes found"))
	}

	return strings.Join(lines, "\n")
}

func (m *SearchModel) viewConfirmQuit() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Theme.Text).
		Align(lipgloss.Center)

	promptStyle := lipgloss.NewStyle().
		Foreground(Theme.Text).
		Align(lipgloss.Center).
		MarginTop(1)

	selectedBg := Theme.Primary
	dimBg := Theme.Border

	btnYesStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Theme.Text).
		Background(selectedBg).
		Padding(0, 2).
		MarginRight(1)

	btnNoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Theme.Text).
		Background(selectedBg).
		Padding(0, 2).
		MarginLeft(1)

	if m.confirmSelect == 0 {
		btnNoStyle = btnNoStyle.Background(dimBg)
	} else {
		btnYesStyle = btnYesStyle.Background(dimBg)
	}

	popupWidth := 44
	innerWidth := popupWidth - 2
	boxWidth := innerWidth - 6 // 3 padding each side
	title := titleStyle.Width(boxWidth).Render("Quit Anilix?")
	prompt := promptStyle.Width(boxWidth).Render("Are you sure you want to quit?")
	buttons := lipgloss.NewStyle().MarginTop(1).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, btnYesStyle.Render("Yes"), btnNoStyle.Render("No")),
	)

	popup := lipgloss.JoinVertical(lipgloss.Center, title, prompt, buttons)
	popupBox := gradientPopupBox(popup, popupWidth, 3)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupBox)
}

func (m *SearchModel) viewSettings() string {
	selectedBg := Theme.Primary
	dimBg := Theme.Border
	fg := Theme.Text

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(fg).Align(lipgloss.Center)
	labelStyle := lipgloss.NewStyle().Foreground(fg).Width(14)

	selectedStyle := lipgloss.NewStyle().
		Bold(true).Foreground(fg).Background(selectedBg).Padding(0, 1)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(fg).Background(dimBg).Padding(0, 1)

	qualityVal := m.settingsState.Quality
	aniskipVal := "OFF"
	if m.settingsState.AniskipEnabled {
		aniskipVal = "ON"
	}

	qualityStyle := unselectedStyle
	if m.settingsState.Cursor == 0 {
		qualityStyle = selectedStyle
	}
	qualityRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("Quality:"),
		qualityStyle.Render(qualityVal),
	)

	aniskipStyle := unselectedStyle
	if m.settingsState.Cursor == 1 {
		aniskipStyle = selectedStyle
	}
	aniskipRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("AniSkip:"),
		aniskipStyle.Render(aniskipVal),
	)

	// AniList row
	anilistStyle := unselectedStyle
	if m.settingsState.Cursor == 2 {
		anilistStyle = selectedStyle
	}
	var anilistPill string
	if auth.IsLoggedIn() {
		anilistPill = lipgloss.NewStyle().
			Background(Theme.Success).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 1).
			Render("Connected")
	} else {
		anilistPill = lipgloss.NewStyle().
			Background(Theme.Error).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 1).
			Render("Disconnected")
	}
	anilistRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("AniList:"),
		anilistStyle.Render(anilistPill),
	)

	// Update row
	updateStyle := unselectedStyle
	if m.settingsState.Cursor == 3 {
		updateStyle = selectedStyle
	}
	var updateVal string
	switch m.settingsState.UpdateStatus {
	case "checking":
		updateVal = m.loading.View() + " Checking..."
	case "available":
		updateVal = m.settingsState.UpdateVersion + " available"
	case "up_to_date":
		updateVal = "Up to date"
	case "updating":
		updateVal = m.loading.View() + " Downloading..."
	case "updated":
		updateVal = "Restart to apply"
	case "error":
		updateVal = "Error"
	default:
		updateVal = version.Version
	}
	updateRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("Update:"),
		updateStyle.Render(updateVal),
	)

	popupWidth := 44
	innerWidth := popupWidth - 2
	boxWidth := innerWidth - 6 // 3 padding each side
	title := titleStyle.Width(boxWidth).Render("Settings")
	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().MarginTop(1).Render(qualityRow),
		lipgloss.NewStyle().MarginTop(1).Render(aniskipRow),
		lipgloss.NewStyle().MarginTop(1).Render(anilistRow),
		lipgloss.NewStyle().MarginTop(1).Render(updateRow),
	)

	popup := lipgloss.JoinVertical(lipgloss.Center, title, content)
	popupBox := gradientPopupBox(popup, popupWidth, 3)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupBox)
}

func (m *SearchModel) viewAniListLogin() string {
	fg := Theme.Text
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(fg).Align(lipgloss.Center)

	popupWidth := 44
	innerWidth := popupWidth - 2
	boxWidth := innerWidth - 6

	title := titleStyle.Width(boxWidth).Render("AniList Login")
	spinnerView := m.loading.View()
	msg := lipgloss.NewStyle().Foreground(fg).Render("Waiting for browser authorization...")
	content := lipgloss.JoinVertical(lipgloss.Center,
		spinnerView+" "+msg,
		lipgloss.NewStyle().Foreground(Theme.Faint).MarginTop(1).Render("Press Esc to cancel"),
	)

	popup := lipgloss.JoinVertical(lipgloss.Center, title, lipgloss.NewStyle().MarginTop(1).Render(content))
	popupBox := gradientPopupBox(popup, popupWidth, 3)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupBox)
}

func (m *SearchModel) renderRecentSearches() string {
	var lines []string
	header := lipgloss.NewStyle().Foreground(Theme.Faint).Render("recent searches")
	lines = append(lines, header)
	lines = append(lines, gradientLine(30))

	for i, s := range m.recentSearches {
		if i == m.recentCursor {
			lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Primary).Render(fmt.Sprintf("  \u25b8 %s", s)))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(Theme.Faint).Render(fmt.Sprintf("    %s", s)))
		}
	}

	lines = append(lines, gradientLine(30))
	return strings.Join(lines, "\n")
}

func (m *SearchModel) renderChrome(content string) string {
	if m.height <= 0 {
		return ""
	}

	// Title bar
	titleBar := gradientTitle("ANILIX")
	switchText := renderSubDubSwitch(m.searchState.TranslationType)
	qualityText := m.settingsState.Quality
	if qualityText == "" {
		qualityText = "auto"
	}
	rightInfo := lipgloss.NewStyle().Render(
		fmt.Sprintf("%s  %s", switchText, lipgloss.NewStyle().Foreground(Theme.Primary).Render(qualityText)))

	// AniList logged-in indicator
	if m.trackingEnabled && m.anilistUsername != "" {
		rightInfo += "  " + lipgloss.NewStyle().Foreground(Theme.Success).Render("\u25cf "+m.anilistUsername)
	}

	// Pad title bar to push right info to the right
	titleBarWidth := lipgloss.Width(titleBar)
	rightInfoWidth := lipgloss.Width(rightInfo)
	gap := m.width - titleBarWidth - rightInfoWidth - 4
	if gap < 1 {
		gap = 1
	}
	headerLine := titleBar + strings.Repeat(" ", gap) + rightInfo

	// Back indicator (detail state only)
	var backLine string
	if m.state == detailState {
		backLine = lipgloss.NewStyle().Foreground(Theme.Faint).Render("  \u2190 esc back")
	}

	// Count content lines
	h := strings.Count(content, "\n") + 1
	chromeLines := 2 // title + blank
	if backLine != "" {
		chromeLines += 2 // back + blank
	}

	// Help bar — only for states without a list (confirm quit, settings, home)
	// searchState/detailState use the list's built-in help bar
	var helpView string
	var helpBox string
	helpHeight := 0
	if m.state == homeState {
		helpView = m.help.View(newHomeKeymap())
		helpBox = lipgloss.NewStyle().Foreground(Theme.Faint).Render(helpView)
		helpHeight = 1
	} else if m.state == confirmQuitState {
		helpView = m.help.View(confirmKeymap{m.keymap.ConfirmYes, m.keymap.ConfirmNo})
		helpBox = lipgloss.NewStyle().Foreground(Theme.Faint).Render(helpView)
		helpHeight = 1
	} else if m.state == settingsState {
		sHelp := settingsKeymap{
			Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("\u2191/k", "up")),
			Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("\u2193/j", "down")),
			Close: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		}
		if m.settingsState.Cursor <= 1 {
			sHelp.Left = key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("\u2190/h", "decrease"))
			sHelp.Right = key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("\u2192/l", "increase"))
		} else {
			sHelp.Enter = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
		}
		helpView = m.help.View(sHelp)
		helpBox = lipgloss.NewStyle().Foreground(Theme.Faint).Render(helpView)
		helpHeight = 1
	}

	// Progress bar
	var progressBar string
	if m.searchState.Loading || m.episodeState.Loading || m.episodeState.Playing {
		progressBar = lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).Render(m.progress.ViewAs(m.progressPercent)) + "\n"
	}

	// Calculate remaining space
	remaining := m.height - h - chromeLines - helpHeight - 2
	if remaining < 0 {
		remaining = 0
	}

	var result string
	result = headerLine + "\n\n"
	if backLine != "" {
		result += backLine + "\n\n"
	}
	result += content + strings.Repeat("\n", remaining) + progressBar + helpBox

	return result
}

// compactInfoCard renders type/status/episodes/year as a compact 2-line info card.
func compactInfoCard(meta *MetadataPanel) string {
	// Line 1: Type • N Episodes
	var line1 []string
	if meta.Type != "" {
		line1 = append(line1, meta.Type)
	}
	if meta.Episodes > 0 {
		line1 = append(line1, fmt.Sprintf("%d Episodes", meta.Episodes))
	}

	// Line 2: Status • Year
	var line2 []string
	if meta.Status != "" {
		line2 = append(line2, meta.Status)
	}
	if meta.Year > 0 {
		line2 = append(line2, fmt.Sprintf("%d", meta.Year))
	}

	sep := lipgloss.NewStyle().Foreground(Theme.Faint).Render(" \u2022 ")

	var parts []string
	if len(line1) > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(Theme.Text).Render(strings.Join(line1, sep)))
	}
	if len(line2) > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(Theme.Faint).Render(strings.Join(line2, sep)))
	}

	return strings.Join(parts, "\n")
}

// renderWatchProgress renders a compact watch progress component.
// Produces a 2-line block: "Status • current/total" + progress bar with percentage.
func renderWatchProgress(status string, progress, totalEpisodes, width int) string {
	labels := map[string]string{
		"CURRENT":   "Watching",
		"PLANNING":  "Planned",
		"COMPLETED": "Completed",
		"DROPPED":   "Dropped",
		"PAUSED":    "Paused",
		"REPEATING": "Rewatching",
	}
	label := labels[status]
	if label == "" {
		label = status
	}

	// Status color based on state
	statusColor := Theme.Success
	switch status {
	case "DROPPED":
		statusColor = Theme.Error
	case "PLANNING":
		statusColor = Theme.Faint
	case "PAUSED":
		statusColor = Theme.Warning
	case "COMPLETED":
		statusColor = Theme.Primary
	}

	// Line 1: Status • ep/total
	episodeStr := fmt.Sprintf("%d", progress)
	if totalEpisodes > 0 {
		episodeStr = fmt.Sprintf("%d/%d", progress, totalEpisodes)
	}
	statusLine := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(label) +
		lipgloss.NewStyle().Foreground(Theme.Faint).Render(" \u2022 ") +
		lipgloss.NewStyle().Foreground(Theme.Text).Render(episodeStr)

	// Line 2: progress bar (only if total is known)
	if totalEpisodes > 0 && progress > 0 {
		ratio := float64(progress) / float64(totalEpisodes)
		if ratio > 1 {
			ratio = 1
		}
		bar := progressBar(ratio, width)
		return statusLine + "\n" + bar
	}

	return statusLine
}
