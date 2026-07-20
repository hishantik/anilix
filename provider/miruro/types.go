package miruro

import "encoding/json"

var defaultProviderOrder = []string{"bonk", "kiwi", "hop", "ally", "pewe", "bee", "moo"}

type EpisodeCandidate struct {
	AniListID int
	ID        string
	Provider  string
	Category  string
	Number    string
	Title     string
}

type Catalog struct {
	entries map[string]map[string][]EpisodeCandidate
}

type configResponse struct {
	ProviderOrder []string                       `json:"providerOrder"`
	Streaming     map[string]streamingConfigWire `json:"streaming"`
}

type streamingConfigWire struct {
	Player  string `json:"player"`
	Visible bool   `json:"visible"`
}

func (c *Catalog) Candidates(category, number string) []EpisodeCandidate {
	if c == nil || c.entries[category] == nil {
		return nil
	}
	return append([]EpisodeCandidate(nil), c.entries[category][number]...)
}

type catalogResponse struct {
	Providers map[string]providerWire `json:"providers"`
}

type providerWire struct {
	Episodes map[string][]episodeWire `json:"episodes"`
}

type episodeWire struct {
	ID     string          `json:"id"`
	Number json.RawMessage `json:"number"`
	Title  string          `json:"title"`
}

type sourcesResponse struct {
	Streams   []streamWire   `json:"streams"`
	Subtitles []subtitleWire `json:"subtitles"`
	Captions  []subtitleWire `json:"captions"`
}

type streamWire struct {
	URL     string            `json:"url"`
	Quality string            `json:"quality"`
	Label   string            `json:"label"`
	Headers map[string]string `json:"headers"`
}

type subtitleWire struct {
	URL      string `json:"url"`
	File     string `json:"file"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Lang     string `json:"lang"`
}
