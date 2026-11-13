package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/player"
	"github.com/hishantik/anilix/version"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

func (m *SearchModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	m.syncDelegates()

	var content string

	switch m.state {
	case searchState:
		content = m.viewSearchState()
	case animeState:
		content = m.viewAnimeState()
	case detailState:
		content = m.viewDetailState()
	case historyState:
		content = m.viewHistoryState()
	case confirmQuitState:
		content = m.viewConfirmQuit()
	case settingsState:
		content = m.viewSettings()
	case anilistLoginState:
		content = m.viewAniListLogin()
	case anilistState:
		content = m.viewAniListState()
	case confirmLogoutState:
		content = m.viewConfirmLogout()
	case themeState:
		content = m.viewThemeShowcase()
	}

	v := tea.NewView(m.renderShell(content))
	v.AltScreen = true
	return v
}

// syncDelegates tells the result lists whether they hold the keyboard.
//
// A list that lost the focus keeps its cursor but mutes the row: the user
// can still see what is selected, and the keys go somewhere else.
func (m *SearchModel) syncDelegates() {
	focused := !m.textInput.Focused() && !m.navHasFocus()

	if m.searchDelegate != nil {
		m.searchDelegate.focused = focused
	}
	if m.episodeDelegate != nil {
		m.episodeDelegate.focused = focused
	}
}

// viewSearchState renders the search screen: the search field, and below it
// either the results, the recent searches or the state of the last request.
func (m *SearchModel) viewSearchState() string {
	lines := []string{
		m.renderSearchField(),
		"",
	}

	switch {
	case m.searchState.Loading:
		lines = append(lines, m.loadingLine("Searching..."))

	case len(m.searchState.Results) > 0:
		lines = append(lines, m.renderDiscovery())

	case m.textInput.Value() == "" && len(m.recentSearches) > 0:
		lines = append(lines, m.renderRecentSearches())

	case m.searchState.Err != nil:
		lines = append(lines, ui.ErrorState{
			Title:   "Search failed",
			Message: m.searchState.Err.Error(),
			Hint:    "ctrl+r to retry",
		}.Render(m.contentWidth))

	case m.textInput.Value() != "" && !m.textInput.Focused():
		// A query ran and came back with nothing: the screen must still
		// say so instead of going blank. The input is blurred once a
		// search completes, and it is empty until the user types, so
		// this case only matches after a finished search.
		lines = append(lines, ui.EmptyState{
			Icon:  theme.Icon(theme.IconSearch),
			Title: "No results for " + m.textInput.Value(),
			Hint:  "Try a shorter title or check the spelling.",
		}.Render(m.contentWidth))
	}

	return strings.Join(lines, "\n")
}

// renderSearchField renders the search field.
//
// The field keeps its own frame: the text input only renders the value, so
// the icon, the border and the focus treatment belong to the screen and
// not to the widget.
func (m *SearchModel) renderSearchField() string {
	return ui.InputField{
		Content:     m.textInput.View(),
		Placeholder: m.textInput.Placeholder,
		Focused:     m.textInput.Focused(),
		Icon:        theme.Icon(theme.IconSearch),
		Width:       m.contentWidth,
	}.Render()
}

// viewHistoryState renders the watch history.
//
// The screen is the list of the episodes that were played: what it was,
// which episode, and how long ago. Enter resumes the entry.
func (m *SearchModel) viewHistoryState() string {
	entries := m.watchHistory()
	if len(entries) == 0 {
		return ui.EmptyState{
			Icon:  theme.Icon(theme.IconHistory),
			Title: "Nothing watched yet",
			Hint:  "The episodes you play are listed here, newest first.",
		}.Render(m.contentWidth)
	}

	lines := []string{
		ui.SectionHeader("Watch history", fmt.Sprintf("%d", len(entries)), m.contentWidth),
		"",
	}
	for i, e := range entries {
		lines = append(lines, ui.ListItem{
			Title:       e.Name,
			Description: "Episode " + e.Episode + " · " + agoLabel(e.At),
			Selected:    i == m.recentCursor,
			Focused:     true,
		}.Render(m.contentWidth))
	}

	return strings.Join(lines, "\n")
}

// searchLayout computes two-column dimensions for the search results view.
//
// The widths come from the layout tier of the terminal, not from a literal
// column count: the tier is the single place that decides how wide a
// screen is allowed to be.
func searchLayout(totalWidth, totalHeight int) (leftW, rightW, availableH int, isNarrow bool) {
	availableH = totalHeight - 4
	if availableH < 5 {
		availableH = 5
	}

	rightW = ui.PreviewWidth(totalWidth)
	if rightW == 0 {
		// A compact terminal has a single column: the list takes it all.
		w := totalWidth - 4
		if w < 20 {
			w = 20
		}
		return w, 0, availableH, true
	}

	leftW = ui.ListWidth(totalWidth)
	return leftW, rightW, availableH, false
}

// metaFacts renders the facts of an anime: type, status, episode count and
// year.
func metaFacts(meta *MetadataPanel) []string {
	rows := make([]string, 0, 4)
	if meta.Type != "" {
		rows = append(rows, ui.InfoRow("Type", meta.Type))
	}
	if meta.Status != "" {
		rows = append(rows, ui.InfoRow("Status", meta.Status))
	}
	if meta.Episodes > 0 {
		rows = append(rows, ui.InfoRow("Episodes", strconv.Itoa(meta.Episodes)))
	}
	if meta.Year > 0 {
		rows = append(rows, ui.InfoRow("Year", strconv.Itoa(meta.Year)))
	}
	return rows
}

// metaStats renders the rank and the popularity of an anime.
func metaStats(meta *MetadataPanel) []string {
	rows := make([]string, 0, 2)
	if meta.Rank > 0 {
		rows = append(rows, ui.InfoRow("Rank", fmt.Sprintf("#%d", meta.Rank)))
	}
	if meta.Popularity > 0 {
		rows = append(rows, ui.InfoRow("Popularity", fmt.Sprintf("#%d", meta.Popularity)))
	}
	return rows
}

func (m *SearchModel) viewDetailState() string {
	meta := m.searchState.Metadata
	if meta == nil {
		meta = &MetadataPanel{}
	}

	leftW, rightW, _, isNarrow := detailLayout(m.contentWidth, m.contentHeight)

	if isNarrow {
		return m.renderDetailSingleColumn(meta, rightW)
	}

	leftPanel := m.renderDetailLeftPanel(meta, leftW, m.contentHeight)
	rightPanel := m.renderDetailRightPanel(meta, rightW, m.contentHeight)
	gap := strings.Repeat(" ", 2)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, gap, rightPanel)
}

// detailLayout computes two-column dimensions for the detail view.
//
// The left column holds the cover art and the facts, the right one the
// episode list. When the terminal cannot hold both, the screen falls back
// to a single column instead of squeezing the episode list.
func detailLayout(totalWidth, totalHeight int) (leftW, rightW, availableH int, isNarrow bool) {
	availableH = totalHeight - 4 // reserve chrome space
	if availableH < 5 {
		availableH = 5
	}

	if !ui.BreakpointOf(totalWidth).IsTwoColumn() {
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
		// There is no room for both columns: the episodes take the screen.
		w := totalWidth - 4
		if w < 20 {
			w = 20
		}
		return 0, w, availableH, true
	}
	// Cap right panel at ~100 for readability on wide terminals
	if rightW > 100 {
		rightW = 100
	}

	return leftW, rightW, availableH, false
}

// detailArtworkRows returns the number of rows the cover of the episode
// screen takes.
//
// The panel below the cover holds the score, the facts and the tags: the
// cover gets what is left.
func (m *SearchModel) detailArtworkRows(width, height int) int {
	rows := height - 14
	if max := (width - 2) * 3 / 2; rows > max {
		rows = max
	}
	if rows < 8 {
		rows = 8
	}
	return rows
}

// renderDetailLeftPanel renders the cover art and the facts of an anime.
func (m *SearchModel) renderDetailLeftPanel(meta *MetadataPanel, width int, termHeight int) string {
	var sections []string

	// The cover, framed. The frame keeps the shape of the panel when the
	// image cannot be rendered at all.
	if art := m.renderArtwork(meta, width, m.detailArtworkRows(width, termHeight)); art != "" {
		sections = append(sections, art)
	}

	// Score
	if meta.Score > 0 {
		sections = append(sections, "")
		sections = append(sections, ui.Rating(meta.Score))
	}

	// Stats
	stats := metaStats(meta)
	if len(stats) > 0 {
		sections = append(sections, "")
		sections = append(sections, stats...)
	}

	// Facts
	facts := metaFacts(meta)
	if len(facts) > 0 {
		sections = append(sections, "")
		sections = append(sections, facts...)
	}

	// AniList tracking, when the entry has it.
	if row := m.anilistFactRow(); row != "" {
		sections = append(sections, "")
		sections = append(sections, row)
	}

	// Genre tags — wrap to fit within panel width
	if len(meta.Genres) > 0 {
		sections = append(sections, "")
		sections = append(sections, ui.ChipRow(meta.Genres, width, theme.SecondaryContainer)...)
	}

	// Source
	if meta.Source != "" {
		sections = append(sections, "")
		sections = append(sections, theme.Caption.Render("Source: "+meta.Source))
	}

	content := strings.Join(sections, "\n")
	// Width sets minimum, MaxWidth enforces maximum — prevents overflow
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(content)
}

// renderDetailRightPanel renders the title, the synopsis and the episode
// list of an anime.
func (m *SearchModel) renderDetailRightPanel(meta *MetadataPanel, width, height int) string {
	var lines []string

	// Title section
	lines = append(lines, wrapText(meta.Title, width, theme.Title.In(theme.Primary)))
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, wrapText(meta.TitleEnglish, width, theme.Label.Style()))
	}
	if meta.TitleNative != "" {
		lines = append(lines, wrapText(meta.TitleNative, width, theme.Label.Style()))
	}

	// Divider
	lines = append(lines, "")
	lines = append(lines, ui.Divider(false, width))

	// Synopsis
	synopsisLines := 0
	if meta.Synopsis != "" {
		synopsis := stripHTML(meta.Synopsis)
		maxLen := width * 5 // ~5 lines
		if maxLen > 0 {
			synopsis = truncateSynopsis(synopsis, maxLen)
		}
		rendered := wrapText(synopsis, width, theme.Body.Style())
		lines = append(lines, "")
		lines = append(lines, rendered)
		synopsisLines = strings.Count(rendered, "\n") + 1
	}

	// Second divider
	lines = append(lines, "")
	lines = append(lines, ui.Divider(false, width))
	lines = append(lines, "")

	// Calculate episode list height
	fixedLines := 4 // title lines (at least 1)
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		fixedLines++
	}
	if meta.TitleNative != "" {
		fixedLines++
	}
	fixedLines += 3 // dividers + blank lines
	fixedLines += synopsisLines
	fixedLines += 3                          // second divider + blanks
	fixedLines += 2                          // episode header: blank line + section title
	episodeHeight := height - fixedLines - 8 // chrome reservation
	if episodeHeight < 5 {
		episodeHeight = 5
	}

	// Set episode list size for the right panel
	m.episodeList.SetSize(width, episodeHeight)

	lines = append(lines, m.renderTrackingStatus()...)
	lines = append(lines, episodeSectionHeader(m.searchState.TranslationType, width)...)
	lines = append(lines, m.renderEpisodeArea()...)

	// Width + MaxWidth on each line to prevent overflow
	joined := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(joined)
}

// episodeSectionHeader returns the header of the episode list: the name of
// the section and the translation the servers were fetched with.
//
// The badge is the same switch the app bar shows, so the list and the
// toggle always agree about which translation is on.
func episodeSectionHeader(translation string, width int) []string {
	if translation == "" {
		translation = "sub"
	}
	return []string{
		"",
		ui.SectionHeader("Episodes", renderSubDubSwitch(translation), width),
	}
}

// renderTrackingStatus renders the AniList tracking state of the anime.
func (m *SearchModel) renderTrackingStatus() []string {
	if !m.trackingEnabled {
		return nil
	}
	if m.episodeState.TrackingStatus != "" {
		return []string{statusLine(formatTrackingStatus(m.episodeState.TrackingStatus, m.episodeState.TrackingProgress))}
	}
	return []string{mutedLine("Not in your AniList")}
}

// renderEpisodeArea renders the episode list, or the state of the request
// that fills it.
func (m *SearchModel) renderEpisodeArea() []string {
	switch {
	case m.episodeState.Loading:
		return []string{m.loadingLine("Loading episodes...")}

	case m.episodeState.Playing:
		selectedAnime := m.searchState.Results[m.searchState.Selected]
		epNum := ""
		if m.episodeState.Selected < len(m.episodeState.Episodes) {
			epNum = m.episodeState.Episodes[m.episodeState.Selected]
		}
		headline := theme.Title.In(theme.Primary).
			Render(fmt.Sprintf("Playing: %s - Episode %s", selectedAnime.Name, epNum))
		return []string{headline, mutedLine("Launching player...")}

	case len(m.episodeState.Episodes) > 0:
		return []string{m.episodeList.View()}

	case m.episodeState.Err != nil:
		return []string{errorLine(fmt.Sprintf("Error: %v", m.episodeState.Err))}

	default:
		return []string{mutedLine("No episodes found")}
	}
}

// renderDetailSingleColumn renders the detail view as a single column for narrow terminals.
func (m *SearchModel) renderDetailSingleColumn(meta *MetadataPanel, width int) string {
	var lines []string

	// Title
	lines = append(lines, wrapText(meta.Title, width, theme.Title.In(theme.Primary)))
	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, wrapText(meta.TitleEnglish, width, theme.Label.Style()))
	}
	if meta.TitleNative != "" {
		lines = append(lines, wrapText(meta.TitleNative, width, theme.Label.Style()))
	}

	// Info line
	var infoParts []string
	if meta.Year > 0 {
		infoParts = append(infoParts, strconv.Itoa(meta.Year))
	}
	if meta.Type != "" {
		infoParts = append(infoParts, meta.Type)
	}
	if meta.Episodes > 0 {
		infoParts = append(infoParts, fmt.Sprintf("%d episodes", meta.Episodes))
	}
	if meta.Status != "" {
		infoParts = append(infoParts, meta.Status)
	}
	if len(infoParts) > 0 {
		lines = append(lines, wrapText(mutedLine(strings.Join(infoParts, "  ·  ")), width, lipgloss.NewStyle()))
	}

	// Genres
	if len(meta.Genres) > 0 {
		lines = append(lines, mutedLine(strings.Join(meta.Genres, ", ")))
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
		lines = append(lines, wrapText(mutedLine(strings.Join(scoreParts, "  ·  ")), width, lipgloss.NewStyle()))
	}

	// Source
	if meta.Source != "" {
		lines = append(lines, theme.Caption.Render("Source: "+meta.Source))
	}

	// Synopsis
	if meta.Synopsis != "" {
		synopsis := stripHTML(meta.Synopsis)
		maxLen := width * 3
		if maxLen > 0 {
			synopsis = truncateSynopsis(synopsis, maxLen)
		}
		lines = append(lines, "")
		lines = append(lines, wrapText(synopsis, width, theme.Body.Style()))
	}

	// Divider
	sepWidth := width
	if sepWidth < 10 {
		sepWidth = 10
	}
	lines = append(lines, "")
	lines = append(lines, ui.Divider(false, sepWidth))

	lines = append(lines, m.renderTrackingStatus()...)
	lines = append(lines, episodeSectionHeader(m.searchState.TranslationType, width)...)
	lines = append(lines, m.renderEpisodeArea()...)

	return strings.Join(lines, "\n")
}

func (m *SearchModel) viewConfirmQuit() string {
	yes := ui.Button{Label: "Yes", Variant: ui.ButtonFilled}
	no := ui.Button{Label: "No", Variant: ui.ButtonText}
	if m.confirmSelect == 0 {
		yes.State = theme.StateFocused
		no.State = theme.StateHover
	} else {
		yes.State = theme.StateHover
		no.State = theme.StateFocused
	}

	dialog := ui.Dialog{
		Title: "Quit Anilix?",
		Body:  "Are you sure you want to quit?",
		Actions: []string{
			yes.Render(),
			no.Render(),
		},
		Width:     ui.OverlayWidth(44, m.contentWidth),
		Elevation: theme.Elevation3,
	}.Render()

	return lipgloss.Place(m.contentWidth, m.contentHeight, lipgloss.Center, lipgloss.Center, dialog)
}

// The grouped rows of the settings sheet.
//
// Every row carries the section it belongs to, its label, and the word
// the code uses to know which behaviour the cursor is on. The groups are
// the order a person looks for a setting in: what plays, what it sounds
// like, how the app looks, what it draws with, what it plays on, the
// account, and the app itself.
type settingsRowDef struct {
	section string
	label   string
	key     string
}

const (
	settingsRowQuality = iota
	settingsRowAniSkip
	settingsRowSubDub
	settingsRowSidebar
	settingsRowRenderer
	settingsRowPlayer
	settingsRowAniList
	settingsRowUpdate
	settingsRowCount
)

// settingsRows is the content of the settings sheet.
var settingsRows = []settingsRowDef{
	{"Playback", "Quality", "quality"},
	{"Playback", "AniSkip", "aniskip"},
	{"Audio", "Sub/Dub", "subdub"},
	{"Appearance", "Sidebar", "sidebar"},
	{"Images", "Renderer", "renderer"},
	{"Player", "Player", "player"},
	{"AniList", "Account", "anilist"},
	{"Application", "Update", "update"},
}

// The renderers the settings sheet offers. The chain validates the
// choice anyway: kitty is picked by auto when the terminal has it, and
// the in-band protocols are edited in the file by the people who use
// terminals that do them.
var rendererChoices = []string{"auto", "halfblock", "chafa", "ascii"}

// viewSettings renders the settings sheet.
//
// The sheet is grouped, and it scrolls: a phone terminal has no room for
// eight rows and their section headers at once, so the body is cut to
// the height the content region has, with the cursor row kept on screen.
func (m *SearchModel) viewSettings() string {
	body := m.settingsBody(m.contentHeight - 7)

	dialog := ui.Dialog{
		Title:     "Settings",
		Body:      body,
		Width:     ui.OverlayWidth(52, m.contentWidth),
		Elevation: theme.Elevation3,
	}.Render()

	return lipgloss.Place(m.contentWidth, m.contentHeight, lipgloss.Center, lipgloss.Center, dialog)
}

// settingsBody renders the rows of the settings sheet, cut to the given
// budget of lines.
//
// The budget counts every line the body draws: the section headers make
// the sheet taller than its rows suggest, and they are the first thing
// to scroll away on a small terminal. The cursor row always stays on
// screen.
func (m *SearchModel) settingsBody(budget int) string {
	if budget < 5 {
		budget = 5
	}

	type line struct {
		text string
		row  int // the settings row the line belongs to, -1 for chrome
	}

	lines := make([]line, 0, len(settingsRows)*2)
	for i := range settingsRows {
		if i == 0 || settingsRows[i].section != settingsRows[i-1].section {
			// A section header on its own line: the caption style carries
			// the separation, and blank lines would double the height of
			// a sheet a phone terminal has to fit.
			lines = append(lines, line{text: theme.Caption.In(theme.Muted).Render(settingsRows[i].section), row: -1})
		}
		lines = append(lines, line{text: m.settingsRowValue(i, m.settingsState.Cursor == i), row: i})
	}

	start := 0
	if len(lines) > budget {
		// The line of the cursor row, then the window that keeps it.
		cursorLine := 0
		for i, l := range lines {
			if l.row == m.settingsState.Cursor {
				cursorLine = i
				break
			}
		}
		start = cursorLine - budget + 1
		if start < 0 {
			start = 0
		}
		if start > len(lines)-budget {
			start = len(lines) - budget
		}
		m.settingsState.Scroll = start
		lines = lines[start : start+budget]
	}

	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.text)
	}
	return strings.Join(out, "\n")
}

// settingsRowValue returns the value of one settings row, as it is
// stored right now.
func (m *SearchModel) settingsRowValue(i int, selected bool) string {
	var value string
	switch settingsRows[i].key {
	case "quality":
		value = m.settingsState.Quality
	case "aniskip":
		value = "OFF"
		if m.settingsState.AniskipEnabled {
			value = "ON"
		}
	case "subdub":
		value = strings.ToUpper(m.translation())
	case "sidebar":
		value = "Collapsed"
		if m.nav.expanded {
			value = "Expanded"
		}
	case "renderer":
		value = strings.ToUpper(rendererChoice())
	case "player":
		value = settingsPlayer()
	case "anilist":
		if auth.IsLoggedIn() {
			name := auth.GetUsername()
			if name == "" {
				name = "connected"
			}
			value = name
		} else {
			value = "Not connected"
		}
	case "update":
		value = m.settingsUpdateValue()
	}

	pill := ui.Chip{Label: value, Container: theme.SurfaceVariant}.Render()
	return m.settingsRow(settingsRows[i].label, pill, selected)
}

// translation returns the active translation of the streams.
func (m *SearchModel) translation() string {
	if m.searchState.TranslationType == "" {
		return "sub"
	}
	return m.searchState.TranslationType
}

// rendererChoice returns the configured artwork renderer.
func rendererChoice() string {
	name := config.GetString("artwork.renderer")
	if name == "" {
		return "auto"
	}
	return name
}

// settingsPlayer returns the player the configuration asks for.
func settingsPlayer() string {
	current := config.GetString("player")
	if current == "" {
		current = "mpv"
	}
	return current
}

// settingsPlayerChoices returns the values the player row can cycle
// through: the configured player first, the installed ones after it.
func settingsPlayerChoices() []string {
	current := settingsPlayer()
	choices := []string{current}
	var d player.Detector
	for _, name := range d.Detect() {
		if name != current {
			choices = append(choices, name)
		}
	}
	return choices
}

// settingsAdjust changes the value of the row the cursor is on.
//
// Only the rows with a value the app can actually change respond: the
// account row and the update row act on enter instead.
func (m *SearchModel) settingsAdjust(decrease bool) {
	step := 1
	if decrease {
		step = -1
	}
	switch m.settingsState.Cursor {
	case settingsRowQuality:
		m.settingsState.Quality = cycleValue(m.settingsState.Quality,
			[]string{"best", "1080p", "720p", "480p", "360p", "auto"}, step)
		config.Set("quality", m.settingsState.Quality)

	case settingsRowAniSkip:
		m.settingsState.AniskipEnabled = !m.settingsState.AniskipEnabled
		config.Set("aniskip.enabled", m.settingsState.AniskipEnabled)

	case settingsRowSubDub:
		value := "sub"
		if m.translation() == "sub" {
			value = "dub"
		}
		m.searchState.TranslationType = value
		config.Set("translation_type", value)
		m.hianimeProvider.SetTranslation(value)

	case settingsRowSidebar:
		m.nav.expanded = !m.nav.expanded
		value := "collapsed"
		if m.nav.expanded {
			value = "expanded"
		}
		config.Set("ui.sidebar", value)
		m.updateLayout()

	case settingsRowRenderer:
		config.Set("artwork.renderer", cycleValue(rendererChoice(), rendererChoices, step))

	case settingsRowPlayer:
		config.Set("player", cycleValue(settingsPlayer(), settingsPlayerChoices(), step))
	}
}

// cycleValue moves a value one step around its choices.
func cycleValue(value string, choices []string, step int) string {
	idx := 0
	for i, c := range choices {
		if c == value {
			idx = i
			break
		}
	}
	return choices[(idx+step+len(choices))%len(choices)]
}

// settingsUpdateValue returns the value of the update row.
func (m *SearchModel) settingsUpdateValue() string {
	switch m.settingsState.UpdateStatus {
	case "checking":
		return m.loading.View() + " Checking..."
	case "available":
		return m.settingsState.UpdateVersion + " available"
	case "up_to_date":
		return "Up to date"
	case "updating":
		return m.loading.View() + " Downloading..."
	case "updated":
		return "Restart to apply"
	case "error":
		return "Error"
	}
	return version.Version
}

// settingsRow renders one row of the settings sheet.
//
// The cursor row carries the accent bar of the design system, so the row
// that is about to change is obvious without a second colour.
func (m *SearchModel) settingsRow(label, value string, selected bool) string {
	marker := "  "
	role := theme.Text
	if selected {
		marker = theme.Icon(theme.IconSelectionBar) + " "
		role = theme.Primary
	}
	return marker + theme.Label.In(role).Render(label) + "  " + value
}

func (m *SearchModel) viewAniListLogin() string {
	msg := mutedLine("Waiting for browser authorization...")
	content := m.loading.View() + " " + msg

	dialog := ui.Dialog{
		Title:     "AniList Login",
		Body:      content,
		Width:     ui.OverlayWidth(44, m.contentWidth),
		Elevation: theme.Elevation3,
	}.Render()

	return lipgloss.Place(m.contentWidth, m.contentHeight, lipgloss.Center, lipgloss.Center, dialog)
}

// viewAniListState renders the AniList account screen.
//
// The screen says who is connected, what the tracking does, and carries
// the actions that exist: log in, log out, and the tracking switch. It
// does not promise a sync it does not have: the progress of the watched
// episodes is pushed to AniList after playback, and the screen says so.
func (m *SearchModel) viewAniListState() string {
	lines := make([]string, 0, 8)

	// Account
	var account string
	if auth.IsLoggedIn() {
		name := auth.GetUsername()
		if name == "" {
			name = "connected"
		}
		account = ui.Chip{Label: name, Container: theme.Success}.Render()
	} else {
		account = ui.Chip{Label: "Not connected", Container: theme.Error}.Render()
	}
	lines = append(lines,
		ui.SectionHeader("Account", "", m.contentWidth),
		ui.InfoRow("Status", account),
	)
	if auth.IsLoggedIn() {
		lines = append(lines, ui.InfoRow("Tracking", trackingPill(m.trackingEnabled)))
	}

	// Synchronisation
	lines = append(lines,
		"",
		ui.SectionHeader("Synchronisation", "", m.contentWidth),
		wrapText(mutedLine("Progress syncs to AniList after each episode you play."), m.contentWidth, lipgloss.NewStyle()),
	)

	// Actions
	lines = append(lines, "")
	lines = append(lines, ui.SectionHeader("Actions", "", m.contentWidth))
	if auth.IsLoggedIn() {
		lines = append(lines, m.settingsRow("Log out", "", m.anilistCursor == 1))
	} else {
		lines = append(lines, m.settingsRow("Log in", "", m.anilistCursor == 1))
	}

	return strings.Join(lines, "\n")
}

// trackingPill renders the state of the tracking switch.
func trackingPill(on bool) string {
	return ui.Chip{Label: map[bool]string{true: "ON", false: "OFF"}[on], Container: theme.SurfaceVariant}.Render()
}

// viewConfirmLogout asks before the AniList credentials are dropped.
func (m *SearchModel) viewConfirmLogout() string {
	yes := ui.Button{Label: "Yes", Variant: ui.ButtonFilled}
	no := ui.Button{Label: "No", Variant: ui.ButtonText}
	if m.confirmSelect == 0 {
		yes.State = theme.StateFocused
		no.State = theme.StateHover
	} else {
		yes.State = theme.StateHover
		no.State = theme.StateFocused
	}

	dialog := ui.Dialog{
		Title: "Log out of AniList?",
		Body:  "Your tracking settings stay, the token is removed.",
		Actions: []string{
			yes.Render(),
			no.Render(),
		},
		Width:     ui.OverlayWidth(52, m.contentWidth),
		Elevation: theme.Elevation3,
	}.Render()

	return lipgloss.Place(m.contentWidth, m.contentHeight, lipgloss.Center, lipgloss.Center, dialog)
}

// renderRecentSearches renders the list of the queries the user ran before.
func (m *SearchModel) renderRecentSearches() string {
	lines := []string{
		ui.SectionHeader("Recent searches", fmt.Sprintf("%d", len(m.recentSearches)), m.contentWidth),
		"",
	}

	for i, s := range m.recentSearches {
		lines = append(lines, ui.ListItem{
			Title:    s,
			Selected: i == m.recentCursor,
			Focused:  true,
		}.Render(m.contentWidth))
	}

	return strings.Join(lines, "\n")
}

// trackingLabel returns the human wording of a tracking status, e.g.
// "Watching (Ep 5)".
func trackingLabel(status string, progress int) string {
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
	return fmt.Sprintf("%s (Ep %d)", label, progress)
}

func formatTrackingStatus(status string, progress int) string {
	return "AniList: " + trackingLabel(status, progress)
}

// anilistFactRow returns the AniList tracking of the entry as a fact row.
//
// It exists only when the tracking is known: a status that was never
// fetched is not invented, and a model without tracking shows no row.
func (m *SearchModel) anilistFactRow() string {
	if !m.trackingEnabled || m.episodeState.TrackingStatus == "" {
		return ""
	}
	return ui.InfoRow("AniList", trackingLabel(m.episodeState.TrackingStatus, m.episodeState.TrackingProgress))
}
