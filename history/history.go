package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/hishantik/anilix/config"
)

// Entry represents a single play event in the history.
type Entry struct {
	AnimeName    string    `json:"anime_name"`
	MALID        int       `json:"mal_id"`
	AniListID    int       `json:"anilist_id"`
	AllAnimeID   string    `json:"allanime_id"`
	Episode      string    `json:"episode"`
	Timestamp    time.Time `json:"timestamp"`
	CoverURL     string    `json:"cover_url"`
	Genres       []string  `json:"genres,omitempty"`
	Score        float64   `json:"score,omitempty"`
	Type         string    `json:"type,omitempty"`
	Year         int       `json:"year,omitempty"`
	EpisodeCount int       `json:"episode_count,omitempty"`
}

// History manages play history persistence.
type History struct {
	mu      sync.RWMutex
	entries []Entry
	path    string
}

// historyFile is the JSON structure written to disk.
type historyFile struct {
	Entries []Entry `json:"entries"`
}

// New creates a History instance using the default config path.
func New() *History {
	return &History{
		path: config.HistoryPath(),
	}
}

// Load reads the history file from disk.
func (h *History) Load() error {
	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var file historyFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	h.mu.Lock()
	h.entries = file.Entries
	h.mu.Unlock()
	return nil
}

// Save writes the history to disk atomically.
func (h *History) Save() error {
	h.mu.RLock()
	file := historyFile{Entries: h.entries}
	h.mu.RUnlock()

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(h.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

// Add prepends an entry, deduplicating by MALID+Episode.
func (h *History) Add(e Entry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove existing entry with same MALID+Episode
	filtered := h.entries[:0]
	for _, existing := range h.entries {
		if existing.MALID != e.MALID || existing.Episode != e.Episode {
			filtered = append(filtered, existing)
		}
	}
	h.entries = filtered

	// Prepend new entry
	h.entries = append([]Entry{e}, h.entries...)

	// Cap at 50 entries
	if len(h.entries) > 50 {
		h.entries = h.entries[:50]
	}
}

// Recent returns the n most recent entries.
func (h *History) Recent(n int) []Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n > len(h.entries) {
		n = len(h.entries)
	}

	result := make([]Entry, n)
	copy(result, h.entries[:n])
	return result
}

// RecentUnique returns the n most recent entries, deduplicated by MALID
// (keeping the most recent episode per anime).
func (h *History) RecentUnique(n int) []Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[int]bool)
	var result []Entry
	for _, e := range h.entries {
		if e.MALID == 0 {
			continue
		}
		if seen[e.MALID] {
			continue
		}
		seen[e.MALID] = true
		result = append(result, e)
		if len(result) >= n {
			break
		}
	}
	return result
}

// Clear removes all history entries.
func (h *History) Clear() error {
	h.mu.Lock()
	h.entries = nil
	h.mu.Unlock()
	return os.Remove(h.path)
}

// SortByScore sorts entries by score descending (for display purposes).
func SortByScore(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})
}
