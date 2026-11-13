package tui

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hishantik/anilix/config"
)

// The watch history.
//
// Every episode that starts playing is written to a small store on disk:
// the anime, its streaming id, the episode and the time. The history
// screen reads it; nothing else does. The store honours
// history.enabled: a user who switched the history off gets no writes,
// and an empty screen.
type historyEntry struct {
	Name    string    `json:"name"`
	ShowID  string    `json:"show_id"`
	Episode string    `json:"episode"`
	At      time.Time `json:"at"`
}

const watchHistoryLimit = 20

func watchHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".anilix", "history.json")
}

// loadWatchHistory returns the episodes that were played, newest first.
func loadWatchHistory() []historyEntry {
	data, err := os.ReadFile(watchHistoryPath())
	if err != nil {
		return nil
	}
	var entries []historyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// recordWatch adds one played episode to the history.
//
// Replaying an episode moves it to the top instead of duplicating it,
// and the store keeps only the most recent entries: the history answers
// "what was I watching?", not "everything I ever saw".
func recordWatch(name, showID, episode string) {
	if !config.GetBool("history.enabled") {
		return
	}
	if name == "" || episode == "" {
		return
	}

	entries := loadWatchHistory()

	kept := make([]historyEntry, 0, len(entries)+1)
	kept = append(kept, historyEntry{Name: name, ShowID: showID, Episode: episode, At: time.Now()})
	for _, e := range entries {
		if e.Name == name && e.Episode == episode {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) > watchHistoryLimit {
		kept = kept[:watchHistoryLimit]
	}

	path := watchHistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[anilix] failed to create config dir: %v\n", err)
		return
	}
	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		log.Printf("[anilix] failed to marshal watch history: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("[anilix] failed to save watch history: %v\n", err)
	}
}

// agoLabel renders a timestamp as a rough age: the unit a person uses
// when they remember what they watched.
func agoLabel(at time.Time) string {
	d := time.Since(at)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return plural(int(d.Hours()/24), "day")
	default:
		return at.Format("Jan 2")
	}
}

// plural renders "3 minutes" or "1 minute".
func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	if n >= 100 {
		return "a while ago"
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
