package source

import "testing"

func TestEpisodeString(t *testing.T) {
	tests := []struct {
		name     string
		episode  Episode
		expected string
	}{
		{"integer episode", Episode{Number: 1, Title: "", Anime: nil}, "1"},
		{"half episode", Episode{Number: 1.5, Title: "", Anime: nil}, "1.5"},
		{"with title", Episode{Number: 5, Title: "The Beginning", Anime: nil}, "The Beginning"},
		{"large number", Episode{Number: 24, Title: "", Anime: nil}, "24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.episode.String()
			if result != tt.expected {
				t.Errorf("Episode.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
