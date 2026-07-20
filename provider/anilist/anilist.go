package anilist

import (
	"context"

	"github.com/hishantik/anilix/source"
)

type AniListProvider struct {
	client *Client
}

func NewAniListProvider() *AniListProvider {
	return &AniListProvider{
		client: NewClient(),
	}
}

func (a *AniListProvider) Name() string {
	return "AniList"
}

func (a *AniListProvider) ID() string {
	return "anilist"
}

func (a *AniListProvider) Search(query string) ([]*source.Anime, error) {
	ctx := context.Background()
	results, err := a.client.SearchAnime(ctx, query, 20)
	if err != nil {
		return nil, err
	}

	animes := make([]*source.Anime, 0, len(results))
	for _, media := range results {
		anime := a.mapToAnime(&media)
		animes = append(animes, anime)
	}

	return animes, nil
}

func (a *AniListProvider) mapToAnime(data *MediaData) *source.Anime {
	anime := &source.Anime{
		Name:         data.Title.English,
		URL:          data.SiteURL,
		AniListID:    data.ID,
		MALID:        data.IDMal,
		Status:       data.Status,
		EpisodeCount: data.Episodes,
		Type:         data.Format,
		Score:        float64(data.AverageScore) / 10,
		Popularity:   data.Popularity,
		Genres:       data.Genres,
	}

	if anime.Name == "" {
		anime.Name = data.Title.Romaji
	}
	if anime.Name == "" {
		anime.Name = data.Title.Native
	}

	if data.CoverImage.Large != "" {
		anime.Cover = data.CoverImage.Large
	} else if data.CoverImage.Medium != "" {
		anime.Cover = data.CoverImage.Medium
	} else if data.CoverImage.ExtraLarge != "" {
		anime.Cover = data.CoverImage.ExtraLarge
	}

	if data.StartDate.Year != 0 {
		anime.Year = data.StartDate.Year
	} else if data.SeasonYear != 0 {
		anime.Year = data.SeasonYear
	}

	return anime
}

func (a *AniListProvider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	return []source.Season{}, nil
}

func (a *AniListProvider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	return []*source.Episode{}, nil
}

func (a *AniListProvider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	return []*source.Stream{}, nil
}
