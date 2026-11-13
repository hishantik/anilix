package jikan

import (
	"context"
	"fmt"

	"github.com/hishantik/anilix/source"
)

type JikanProvider struct {
	client *JikanClient
}

func NewJikanProvider() *JikanProvider {
	return &JikanProvider{
		client: NewClient("https://api.jikan.moe/v4"),
	}
}

func (j *JikanProvider) Name() string {
	return "Jikan"
}

func (j *JikanProvider) ID() string {
	return "jikan"
}

func (j *JikanProvider) Search(query string) ([]*source.Anime, error) {
	ctx := context.Background()
	resp, err := j.client.SearchAnime(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("Jikan search failed: %w", err)
	}

	animes := make([]*source.Anime, 0, len(resp.Data))
	for _, data := range resp.Data {
		anime := j.mapToAnime(&data)
		animes = append(animes, anime)
	}

	return animes, nil
}

func (j *JikanProvider) mapToAnime(data *AnimeData) *source.Anime {
	anime := &source.Anime{
		Name:         data.TitleEnglish,
		URL:          data.URL,
		MALID:        data.MalID,
		Status:       data.Status,
		EpisodeCount: data.Episodes,
		Type:         data.Type,
		Rating:       data.Rating,
		Score:        data.Score,
		Rank:         data.Rank,
		Popularity:   data.Popularity,
	}

	if anime.Name == "" {
		anime.Name = data.Title
	}

	if data.Images.JPG.LargeImageURL != "" {
		anime.Cover = data.Images.JPG.LargeImageURL
	} else if data.Images.JPG.ImageURL != "" {
		anime.Cover = data.Images.JPG.ImageURL
	}

	if year, ok := data.Year.(float64); ok {
		anime.Year = int(year)
	}

	if len(data.Genres) > 0 {
		genres := make([]string, len(data.Genres))
		for i, g := range data.Genres {
			genres[i] = g.Name
		}
		anime.Genres = genres
	}

	return anime
}

func (j *JikanProvider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	return []source.Season{}, nil
}

func (j *JikanProvider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	return []*source.Episode{}, nil
}

func (j *JikanProvider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	return []*source.Stream{}, nil
}
