package aniskip

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hishantik/anilix/curl"
)

// SkipInterval represents a single intro/outro skip segment.
type SkipInterval struct {
	StartTime float64
	EndTime   float64
	Type      string // "op" or "ed"
}

type apiResponse struct {
	Found   bool          `json:"found"`
	Results []apiSkipTime `json:"results"`
}

type apiSkipTime struct {
	Interval apiInterval `json:"interval"`
	SkipType string      `json:"skipType"`
}

type apiInterval struct {
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
}

// GetSkipTimes fetches intro/outro skip times for an anime episode from the
// AniSkip API. Returns nil, nil when malID is 0 or no skip times are found.
func GetSkipTimes(malID int, episodeNum int) ([]SkipInterval, error) {
	if malID == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf(
		"https://api.aniskip.com/v2/skip-times/%d/%d?types=op&types=ed&episodeLength=0",
		malID, episodeNum,
	)

	body, err := curl.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("aniskip request failed: %w", err)
	}

	var resp apiResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("aniskip parse error: %w", err)
	}

	if !resp.Found {
		return nil, nil
	}

	var intervals []SkipInterval
	for _, r := range resp.Results {
		if r.SkipType != "op" && r.SkipType != "ed" {
			continue
		}
		intervals = append(intervals, SkipInterval{
			StartTime: r.Interval.StartTime,
			EndTime:   r.Interval.EndTime,
			Type:      r.SkipType,
		})
	}

	return intervals, nil
}

// FormatForScriptOpts encodes skip times into the mpv script-opts format:
// "op:87.5-118.2,ed:1340.0-1370.5"
func FormatForScriptOpts(intervals []SkipInterval) string {
	var parts []string
	for _, iv := range intervals {
		parts = append(parts, fmt.Sprintf("%s:%s-%s",
			iv.Type,
			strconv.FormatFloat(iv.StartTime, 'f', 1, 64),
			strconv.FormatFloat(iv.EndTime, 'f', 1, 64),
		))
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ","
		}
		result += p
	}
	return result
}
