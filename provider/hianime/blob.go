package hianime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// embedConfig is the decoded embed page config. Only the fields we consume
// are typed; the rest is ignored.
type embedConfig struct {
	Src       string          `json:"src"`
	Subtitles []SubtitleTrack `json:"subtitles"`
}

// deobfuscateBlob decodes the embed page's config blob, which ships as
// base64(json XOR "otaku-embed-v1").
func deobfuscateBlob(blob string) (*embedConfig, error) {
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	key := []byte(obfKey)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ key[i%len(key)]
	}

	var cfg embedConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		return nil, fmt.Errorf("json decode failed: %w", err)
	}
	return &cfg, nil
}
