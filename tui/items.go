package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hishantik/anilix/source"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/tui/theme"
	"github.com/hishantik/anilix/tui/ui"
)

// listRow is a list item that knows how to render itself.
//
// The list handles the scrolling and the pagination, the row decides how it
// looks: both belong to the design system, not to the widget.
type listRow interface {
	list.Item

	// render draws the row at the given width.
	render(width int, selected, focused bool) string
}

// animeItem is one search result.
type animeItem struct {
	anime *source.Anime
	// meta is the metadata of the entry, when it is already known.
	//
	// The search provider only knows the name it lists the entry under; the
	// metadata provider knows the alternative titles and the airing status.
	// The row shows both, so the item carries the metadata it was built
	// with.
	meta *MetadataPanel
}

// Title returns the name of the anime.
func (i animeItem) Title() string {
	return i.anime.Name
}

// Description returns the facts of the anime shown under its name.
//
// The format, the year, the number of episodes and the status are the four
// facts that tell two entries apart.
func (i animeItem) Description() string {
	parts := make([]string, 0, 4)
	if i.anime.Type != "" {
		parts = append(parts, i.anime.Type)
	}
	if i.anime.Year > 0 {
		parts = append(parts, strconv.Itoa(i.anime.Year))
	}
	if i.anime.EpisodeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d eps", i.anime.EpisodeCount))
	}
	if status := i.status(); status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, " · ")
}

// status returns the airing status of the anime.
func (i animeItem) status() string {
	if i.meta != nil && i.meta.Status != "" {
		return i.meta.Status
	}
	return i.anime.Status
}

// score returns the score of the anime.
func (i animeItem) score() float64 {
	if i.meta != nil && i.meta.Score > 0 {
		return i.meta.Score
	}
	return i.anime.Score
}

// AltTitle returns the alternative title of the anime.
//
// The native title comes first: it is the one the series is known by in its
// own country, and the one that tells two entries with the same name apart.
func (i animeItem) AltTitle() string {
	if i.meta == nil {
		return ""
	}
	for _, candidate := range []string{i.meta.TitleNative, i.meta.TitleEnglish, i.meta.Title} {
		if candidate != "" && candidate != i.anime.Name {
			return candidate
		}
	}
	return ""
}

// FilterValue returns the text the list filters on.
func (i animeItem) FilterValue() string { return i.anime.Name }

// render draws the row as a design system list item.
func (i animeItem) render(width int, selected, focused bool) string {
	return ui.ListItem{
		Title:        i.anime.Name,
		Subtitle:     i.AltTitle(),
		Description:  i.Description(),
		Trailing:     ui.RatingText(i.score()),
		TrailingRole: theme.Tertiary,
		Selected:     selected,
		Focused:      focused,
	}.Render(width)
}

// episodeItem is one episode of an anime.
type episodeItem struct {
	number string
	title  string
	// watched and nextUp come from the AniList tracking of the entry.
	// An episode the user has already seen gets the check, the first one
	// after the progress gets the play glyph: the list answers "where
	// was I?" at a glance.
	watched bool
	nextUp  bool
}

// Title returns the number of the episode.
func (i episodeItem) Title() string {
	return "Episode " + i.number
}

// Description returns the title of the episode.
func (i episodeItem) Description() string { return i.title }

// FilterValue returns the text the list filters on.
func (i episodeItem) FilterValue() string { return i.number }

// statusBadge returns the trailing badge of the row: the play glyph for
// the episode that is next up, the check for a watched one. An episode
// with no tracking gets no badge.
func (i episodeItem) statusBadge() (string, theme.Role) {
	switch {
	case i.nextUp:
		return theme.Icon(theme.IconPlay), theme.Primary
	case i.watched:
		return theme.Icon(theme.IconCheck), theme.Muted
	}
	return "", theme.RoleUnset
}

// render draws the row as a design system list item.
func (i episodeItem) render(width int, selected, focused bool) string {
	item := ui.ListItem{
		Title:       i.Title(),
		Description: i.title,
		Selected:    selected,
		Focused:     focused,
	}
	if badge, role := i.statusBadge(); badge != "" {
		item.Trailing = badge
		item.TrailingRole = role
	}
	return item.Render(width)
}

// resultDelegate renders the rows of a result list.
//
// The delegate is deliberately thin: it hands the row to the design system
// and leaves the scrolling to the list.
type resultDelegate struct {
	// focused reports whether the list holds the keyboard. A list that
	// lost the focus keeps its cursor, but mutes it: the cursor still
	// shows what is selected, the keyboard is somewhere else.
	focused bool
}

// Height returns the height of a row: the title and the description.
func (d *resultDelegate) Height() int { return 2 }

// Spacing returns the gap between two rows.
func (d *resultDelegate) Spacing() int { return 0 }

// Update handles the messages of the rows. Nothing in a row is stateful.
func (d *resultDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

// Render draws one row.
func (d *resultDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	row, ok := item.(listRow)
	if !ok {
		return
	}

	width := m.Width()
	if width < 1 {
		width = 1
	}

	fmt.Fprint(w, row.render(width, index == m.Index(), d.focused))
}

// makeSearchList builds the list of the search results.
func makeSearchList(km keymap, delegate *resultDelegate) list.Model {
	return makeResultList(km, delegate)
}

// makeEpisodeList builds the list of the episodes.
func makeEpisodeList(km keymap, delegate *resultDelegate) list.Model {
	return makeResultList(km, delegate)
}

// makeResultList builds a result list.
//
// The list has no title, no status bar and no filter: the shell shows the
// screen title in the app bar, and the footer shows the key hints.
func makeResultList(km keymap, delegate *resultDelegate) list.Model {
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	// The shell shows the key hints in its footer: the help of the list
	// would repeat them.
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.CursorUp = km.Up
	l.KeyMap.CursorDown = km.Down
	// Override quit from default "q" to "ctrl+c"
	l.KeyMap.Quit = km.Quit
	// Add custom keys to the built-in help system
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings, km.Theme}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings, km.Theme}
	}
	return l
}
