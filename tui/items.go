package tui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/hishantik/anilix/source"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
)

type animeItem struct {
	anime *source.Anime
}

func (i animeItem) Title() string {
	var right []string
	if i.anime.Type != "" {
		right = append(right, fmt.Sprintf("[%s]", i.anime.Type))
	}
	if i.anime.Score > 0 {
		right = append(right, fmt.Sprintf("\u2605 %.2f", i.anime.Score))
	}
	if i.anime.EpisodeCount > 0 {
		right = append(right, fmt.Sprintf("%d eps", i.anime.EpisodeCount))
	}
	if len(right) > 0 {
		return fmt.Sprintf("%-40s %s", i.anime.Name, strings.Join(right, "  "))
	}
	return i.anime.Name
}

func (i animeItem) Description() string {
	var parts []string
	if len(i.anime.Genres) > 0 {
		genres := i.anime.Genres
		if len(genres) > 3 {
			genres = genres[:3]
		}
		parts = append(parts, strings.Join(genres, ", "))
	}
	if i.anime.Year > 0 {
		parts = append(parts, strconv.Itoa(i.anime.Year))
	}
	if i.anime.Status != "" {
		parts = append(parts, i.anime.Status)
	}
	return strings.Join(parts, "  \u00b7  ")
}

func (i animeItem) FilterValue() string { return i.anime.Name }

type episodeItem struct {
	number   string
	title    string
	progress episodeProgress // watched, current, or unwatched
}

type episodeProgress int

const (
	episodeUnwatched episodeProgress = iota
	episodeWatched
	episodeCurrent
)

func (i episodeItem) Title() string {
	var prefix string
	switch i.progress {
	case episodeWatched:
		prefix = "\u2713 "
	case episodeCurrent:
		prefix = "\u25b6 "
	}

	if i.title != "" {
		return fmt.Sprintf("%sEpisode %s: %s", prefix, i.number, i.title)
	}
	return fmt.Sprintf("%sEpisode %s", prefix, i.number)
}

func (i episodeItem) Description() string { return "" }
func (i episodeItem) FilterValue() string { return i.number }

// gradientBorderColor returns the middle color of the theme gradient for list item borders.
func gradientBorderColor() color.Color {
	if len(Theme.Gradient) == 0 {
		return lipgloss.Color("#9d4edd")
	}
	mid := len(Theme.Gradient) / 2
	return Theme.Gradient[mid]
}

func makeSearchList(km keymap) list.Model {
	delegate := list.NewDefaultDelegate()
	borderColor := gradientBorderColor()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(borderColor).
		Foreground(Theme.Primary).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(borderColor).
		Foreground(Theme.Faint).
		Padding(0, 0, 0, 1)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(Theme.Text).
		Padding(0, 0, 0, 1)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(Theme.Faint).
		Padding(0, 0, 0, 1)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.CursorUp = km.Up
	l.KeyMap.CursorDown = km.Down
	// Override quit from default "q" to "ctrl+c"
	l.KeyMap.Quit = km.Quit
	// Add custom keys to the built-in help system
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings}
	}
	return l
}

func makeEpisodeList(km keymap) list.Model {
	delegate := list.NewDefaultDelegate()
	borderColor := gradientBorderColor()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(borderColor).
		Foreground(Theme.Primary).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(Theme.Faint).
		Padding(0, 0, 0, 1)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(Theme.Text).
		Padding(0, 0, 0, 1)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(Theme.Faint).
		Padding(0, 0, 0, 1)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.CursorUp = km.Up
	l.KeyMap.CursorDown = km.Down
	l.KeyMap.Quit = km.Quit
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{km.Toggle, km.Settings}
	}
	return l
}
