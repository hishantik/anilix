package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hishantik/anilix/history"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"

	Allanime "github.com/hishantik/anilix/provider/allanime"

	tea "charm.land/bubbletea/v2"
)

// fetchHomeHistoryCmd loads recently played from local history file.
func fetchHomeHistoryCmd(hist *history.History) tea.Cmd {
	return func() tea.Msg {
		entries := hist.RecentUnique(20)
		items := make([]HomeItem, 0, len(entries))
		for _, e := range entries {
			items = append(items, HomeItem{
				Name:         e.AnimeName,
				MALID:        e.MALID,
				AniListID:    e.AniListID,
				AllAnimeID:   e.AllAnimeID,
				Cover:        e.CoverURL,
				Score:        e.Score,
				Type:         e.Type,
				Genres:       e.Genres,
				Year:         e.Year,
				Episode:      e.Episode,
				EpisodeCount: e.EpisodeCount,
				Source:       "history",
			})
		}
		return HomeHistoryLoadedMsg{Items: items}
	}
}

// fetchHomeTrendingCmd fetches trending anime from AniList.
func fetchHomeTrendingCmd(client *anilist.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := client.GetTrendingAnime(ctx, 20)
		if err != nil {
			return HomeTrendingLoadedMsg{Err: err}
		}

		items := make([]HomeItem, 0, len(data))
		for _, m := range data {
			name := m.Title.English
			if name == "" {
				name = m.Title.Romaji
			}
			items = append(items, HomeItem{
				Name:         name,
				AniListID:    m.ID,
				Cover:        m.CoverImage.Large,
				Score:        float64(m.AverageScore) / 10,
				Type:         m.Format,
				Genres:       m.Genres,
				Year:         m.StartDate.Year,
				EpisodeCount: m.Episodes,
				Source:       "trending",
			})
		}
		return HomeTrendingLoadedMsg{Items: items}
	}
}

// fetchHomePopularCmd fetches popular anime from Jikan.
func fetchHomePopularCmd(client *jikan.JikanClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := client.GetTopAnime(ctx, "bypopularity", 20)
		if err != nil {
			return HomePopularLoadedMsg{Err: err}
		}

		items := make([]HomeItem, 0, len(data))
		for _, m := range data {
			name := m.TitleEnglish
			if name == "" {
				name = m.Title
			}
			genres := make([]string, len(m.Genres))
			for i, g := range m.Genres {
				genres[i] = g.Name
			}
			year := 0
			if y, ok := m.Year.(float64); ok {
				year = int(y)
			}
			items = append(items, HomeItem{
				Name:         name,
				MALID:        m.MalID,
				Cover:        m.Images.JPG.LargeImageURL,
				Score:        m.Score,
				Type:         m.Type,
				Genres:       genres,
				Year:         year,
				EpisodeCount: m.Episodes,
				Source:       "popular",
			})
		}
		return HomePopularLoadedMsg{Items: items}
	}
}

// fetchHomeGenreCmd fetches anime for a specific genre from AniList.
func fetchHomeGenreCmd(client *anilist.Client, genre string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := client.GetAnimeByGenre(ctx, genre, 15)
		if err != nil {
			return HomeGenreLoadedMsg{Genre: genre, Err: err}
		}

		items := make([]HomeItem, 0, len(data))
		for _, m := range data {
			name := m.Title.English
			if name == "" {
				name = m.Title.Romaji
			}
			items = append(items, HomeItem{
				Name:         name,
				AniListID:    m.ID,
				Cover:        m.CoverImage.Large,
				Score:        float64(m.AverageScore) / 10,
				Type:         m.Format,
				Genres:       m.Genres,
				Year:         m.StartDate.Year,
				EpisodeCount: m.Episodes,
				Source:       genre,
			})
		}
		return HomeGenreLoadedMsg{Genre: genre, Items: items}
	}
}


// resolveHomeItemAllAnimeID searches AllAnime for an anime and resolves its AllAnimeID.
func resolveHomeItemAllAnimeID(anime *source.Anime, client *Allanime.AllanimeClient, translationType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if translationType == "" {
			translationType = "sub"
		}

		shows, err := client.SearchShows(ctx, anime.Name, 5, 1, translationType)
		if err != nil {
			return SearchErrorMsg{Err: err}
		}

		// Try to match by MAL ID first
		for _, show := range shows {
			if anime.MALID > 0 && show.MalID == strconv.Itoa(anime.MALID) {
				anime.AllAnimeID = show.ID
				return HomeItemResolvedMsg{Anime: anime}
			}
		}

		// Fallback: use first result
		if len(shows) > 0 {
			mapped := client.MapToAnime(&shows[0])
			anime.AllAnimeID = mapped.AllAnimeID
			if anime.Cover == "" {
				anime.Cover = mapped.Cover
			}
			return HomeItemResolvedMsg{Anime: anime}
		}

		return SearchErrorMsg{Err: fmt.Errorf("no AllAnime result found for %s", anime.Name)}
	}
}

// searchByNameForHome searches AllAnime by name to find a match.
func searchByNameForHome(name string, client *Allanime.AllanimeClient, translationType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if translationType == "" {
			translationType = "sub"
		}

		shows, err := client.SearchShows(ctx, name, 5, 1, translationType)
		if err != nil {
			return SearchErrorMsg{Err: err}
		}

		if len(shows) == 0 {
			return SearchErrorMsg{Err: fmt.Errorf("no results found for %s", name)}
		}

		// Return first result as a source.Anime
		anime := client.MapToAnime(&shows[0])
		return SearchResultsMsg{Results: []*source.Anime{anime}}
	}
}

// trimAnimeName truncates an anime name to fit within maxLen characters.
// Uses a proper ellipsis character (…) when truncating.
func trimAnimeName(name string, maxLen int) string {
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) <= maxLen {
		return name
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "\u2026"
}
