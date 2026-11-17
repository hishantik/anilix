package source

// Stream represents a single playable video source for an episode.
// Each stream has a quality level, direct URL, and the provider it came from.
// The Referer field is required by some CDN providers to authorize playback.
type Stream struct {
	Quality       string
	URL           string
	Provider      string
	Referer       string // Required for some providers
	Subtitles     []Subtitle
	NeedsReferrer bool // True if stream requires Referer header to play
}

// Subtitle represents a subtitle track associated with a stream.
type Subtitle struct {
	Language string
	URL      string
}
