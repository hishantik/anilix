package tui

import (
	"fmt"
	"image"
	"strings"
	"sync"

	"github.com/hishantik/anilix/config"
)

// The adaptive artwork renderer.
//
// A terminal can show a picture in several ways, and no two terminals
// agree on which ones they support. The renderer therefore keeps an
// ordered chain of the protocols the running terminal can do, honours
// the renderer the user asked for in the configuration, and drops a
// protocol from the chain for the rest of the session the first time it
// fails to produce a picture.
//
// The chain ends in ProtocolNone, which is not a renderer at all: it is
// the framed text placeholder of ui.Artwork. A terminal that cannot show
// a picture gets that placeholder, never a blank hole.
//
// iTerm2 inline images are deliberately absent from the detected chain.
// They draw over the text grid of terminals that keep images outside of
// it, which breaks a full-screen interface on repaint. iTerm2 is opt-in
// through the configuration, and the configuration is only honoured when
// the terminal actually advertises the protocol: a converter on the PATH
// is not a terminal capability. The advertisement the renderer trusts is
// the terminal's program name (iTerm.app, WezTerm), or a terminal name
// that is specific about the protocol.

// rendererNames maps the words the configuration uses to the protocols.
var rendererNames = map[string]Protocol{
	"kitty":     ProtocolKitty,
	"iterm2":    ProtocolITerm2,
	"halfblock": ProtocolHalfBlock,
	"chafa":     ProtocolChafa,
	"ascii":     ProtocolNone,
	"none":      ProtocolNone,
	"off":       ProtocolNone,
}

// configuredRenderer returns the protocol the user asked for.
//
// The second result is false when the choice is "auto" or something the
// renderer does not know: the chain then comes from the detection alone.
func configuredRenderer() (Protocol, bool) {
	name := strings.ToLower(strings.TrimSpace(config.GetString("artwork.renderer")))
	if name == "" || name == "auto" {
		return ProtocolNone, false
	}
	p, ok := rendererNames[name]
	return p, ok
}

// The renderer that draws a picture when the configuration asks for a
// specific one.
//
// protocolDetected reports whether the terminal advertises a protocol.
//
// This is the "verify before enabling" step: a configured renderer is
// only honoured when the terminal actually has it.
func protocolDetected(p Protocol) bool {
	switch p {
	case ProtocolKitty:
		return DetectKittySupport()
	case ProtocolITerm2:
		return DetectITerm2Support()
	case ProtocolHalfBlock:
		return DetectTrueColorSupport()
	case ProtocolChafa:
		return hasBinary("chafa")
	}
	return true
}

// HandlePrimaryDeviceAttributes records what the terminal reported in its
// answer to the DA1 query. The in-band protocols the answer used to
// reveal are gone, so this is a no-op kept for its call site in Init.
func HandlePrimaryDeviceAttributes(_ []int) bool {
	return false
}

// detectedChain returns the protocols the terminal can do, best first.
//
// The order is the quality order: a transmitted picture beats coloured
// blocks, and coloured blocks beat the text placeholder.
func detectedChain() []Protocol {
	var chain []Protocol
	if DetectKittySupport() {
		chain = append(chain, ProtocolKitty)
	}
	if DetectTrueColorSupport() {
		chain = append(chain, ProtocolHalfBlock)
	}
	if hasBinary("chafa") {
		chain = append(chain, ProtocolChafa)
	}
	chain = append(chain, ProtocolNone)
	return chain
}

// rendererChain returns the chain to render with.
//
// A configured renderer comes first, when the terminal has it; the
// detected protocols follow as the fallback. "none" switches the
// artwork off: the chain is empty and no cover is ever downloaded.
func rendererChain() []Protocol {
	pref, set := configuredRenderer()
	if !set {
		return detectedChain()
	}
	if pref == ProtocolNone {
		return nil
	}
	if !protocolDetected(pref) {
		// The terminal does not have what the user asked for: the
		// preference is ignored rather than half-working.
		return detectedChain()
	}
	chain := []Protocol{pref}
	for _, p := range detectedChain() {
		if p != pref {
			chain = append(chain, p)
		}
	}
	return chain
}

// The protocols that failed to render, for the rest of the session.
//
// A protocol that produced nothing once is not asked again: every cover
// would pay for the same failure.
var (
	renderFailuresMu sync.Mutex
	renderFailures   = map[Protocol]bool{}
)

// markProtocolFailed drops a protocol from the chain for this session.
func markProtocolFailed(p Protocol) {
	renderFailuresMu.Lock()
	defer renderFailuresMu.Unlock()
	renderFailures[p] = true
}

// protocolFailed reports whether a protocol failed earlier.
func protocolFailed(p Protocol) bool {
	renderFailuresMu.Lock()
	defer renderFailuresMu.Unlock()
	return renderFailures[p]
}

// resetRenderFailures clears the failure memory. It exists for the tests.
func resetRenderFailures() {
	renderFailuresMu.Lock()
	defer renderFailuresMu.Unlock()
	renderFailures = map[Protocol]bool{}
}

// ArtworkRender is the output of the renderer: the text the view carries
// and, for the Kitty protocol, the transmit sequence that puts the image
// into the memory of the terminal.
type ArtworkRender struct {
	Placeholder  string // the text of the view
	TransmitSeq  string // the APC sequence for tea.Raw (Kitty only)
	KittyImageID uint32
	Protocol     Protocol
}

// The artwork rendered during this session.
//
// The half-block, chafa and iTerm2 output is self-contained text:
// the same cover at the same size always renders to the same string, so
// it is kept instead of re-encoded. The Kitty output is not cached: it
// references an image in the memory of the terminal, and the screen
// changes delete that image.
var (
	artworkCacheMu sync.Mutex
	artworkCache   = map[string]ArtworkRender{}
	artworkKeys    []string
)

const artworkCacheLimit = 48

// artworkCacheKey identifies one rendered cover.
func artworkCacheKey(coverURL string, protocol Protocol, width, rows int) string {
	return fmt.Sprintf("%s|%d|%dx%d", coverURL, protocol, width, rows)
}

// cachedArtwork returns the rendered cover, when it is known.
func cachedArtwork(key string) (ArtworkRender, bool) {
	artworkCacheMu.Lock()
	defer artworkCacheMu.Unlock()
	r, ok := artworkCache[key]
	return r, ok
}

// cacheArtwork remembers a rendered cover.
//
// The cache is bounded: the oldest entry goes when it is full, so a long
// session cannot grow it without limit.
func cacheArtwork(key string, r ArtworkRender) {
	artworkCacheMu.Lock()
	defer artworkCacheMu.Unlock()
	if _, ok := artworkCache[key]; !ok {
		artworkKeys = append(artworkKeys, key)
	}
	artworkCache[key] = r
	for len(artworkKeys) > artworkCacheLimit {
		delete(artworkCache, artworkKeys[0])
		artworkKeys = artworkKeys[1:]
	}
}

// RenderAdaptive renders the cover with the first protocol of the chain
// that produces a picture, and remembers the ones that did not.
//
// cols and rows are the size of the picture in cells, without the frame
// ui.Artwork draws around it.
func RenderAdaptive(img image.Image, cols, rows int, cachePath, coverURL string) ArtworkRender {
	chain := rendererChain()
	for _, p := range chain {
		if protocolFailed(p) {
			continue
		}

		if p != ProtocolKitty {
			key := artworkCacheKey(coverURL, p, cols, rows)
			if hit, ok := cachedArtwork(key); ok {
				return hit
			}
			result := renderWithProtocol(img, p, cols, rows, cachePath)
			if result.Placeholder == "" {
				markProtocolFailed(p)
				continue
			}
			cacheArtwork(key, result)
			return result
		}

		result := renderWithProtocol(img, p, cols, rows, cachePath)
		if result.Placeholder == "" || result.TransmitSeq == "" {
			markProtocolFailed(p)
			continue
		}
		return result
	}
	return ArtworkRender{}
}

// renderWithProtocol renders the cover with one protocol.
func renderWithProtocol(img image.Image, p Protocol, cols, rows int, cachePath string) ArtworkRender {
	switch p {
	case ProtocolKitty:
		r := renderKittyCover(img, cols, rows)
		return ArtworkRender{
			Placeholder:  r.Placeholder,
			TransmitSeq:  r.TransmitSeq,
			KittyImageID: r.ImageID,
			Protocol:     p,
		}
	case ProtocolHalfBlock:
		return ArtworkRender{Placeholder: renderHalfBlock(img, cols, rows), Protocol: p}
	case ProtocolChafa:
		placeholder := ""
		if cachePath != "" {
			if result, err := renderChafaCover(cachePath, cols); err == nil {
				placeholder = result
			}
		}
		// A failed chafa returns nothing: the chain moves on to the next
		// protocol. Half-block sits ahead of chafa in the chain whenever
		// the terminal has true colour, and the fallback for the rest is
		// the placeholder - never a renderer the chain did not choose.
		return ArtworkRender{Placeholder: placeholder, Protocol: p}
	case ProtocolITerm2:
		return ArtworkRender{Placeholder: renderITerm2Cover(img, cols, rows), Protocol: p}
	}
	return ArtworkRender{}
}
