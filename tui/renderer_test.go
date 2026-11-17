package tui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/hishantik/anilix/config"
)

// clearArtEnv resets every environment variable the detection reads, so a
// test sees only the terminal it sets itself.
func clearArtEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TERM", "TERM_PROGRAM", "COLORTERM", "KITTY_WINDOW_ID"} {
		t.Setenv(key, "")
	}
}

// chainHas reports whether the chain contains a protocol.
func chainHas(chain []Protocol, p Protocol) bool {
	for _, c := range chain {
		if c == p {
			return true
		}
	}
	return false
}

// TestDetectionClaimsNothingWithoutEvidence checks the Termux rule: a
// terminal with an unspecific name gets half-blocks at best, never the
// in-band image protocols, and a terminal without colour gets the
// placeholder only.
func TestDetectionClaimsNothingWithoutEvidence(t *testing.T) {
	clearArtEnv(t)

	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	chain := detectedChain()
	if chainHas(chain, ProtocolKitty) {
		t.Error("a plain xterm-256color claimed the kitty protocol")
	}
	if !chainHas(chain, ProtocolHalfBlock) {
		t.Error("a truecolor terminal was denied half-blocks")
	}

	clearArtEnv(t)
	chain = detectedChain()
	if !chainHas(chain, ProtocolNone) {
		t.Error("the chain lost the text placeholder")
	}
	for _, p := range chain {
		if p != ProtocolNone && p != ProtocolChafa {
			t.Errorf("a terminal without colour claimed protocol %d", p)
		}
	}
}

// TestDetectionReadsTheTerminal checks that the markers of a terminal are
// honoured: WezTerm speaks the kitty protocol, iTerm.app speaks OSC 1337.
func TestDetectionReadsTheTerminal(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM_PROGRAM", "WezTerm")
	if !DetectKittySupport() {
		t.Error("WezTerm was denied the kitty protocol")
	}
	if !DetectITerm2Support() {
		t.Error("WezTerm was denied the iTerm2 protocol")
	}

	clearArtEnv(t)
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if !DetectITerm2Support() {
		t.Error("iTerm.app was denied the iTerm2 protocol")
	}
	if DetectKittySupport() {
		t.Error("iTerm.app claimed the kitty protocol")
	}

	clearArtEnv(t)
	t.Setenv("TERM", "foot")
	if DetectKittySupport() {
		t.Error("foot claimed the kitty protocol")
	}
}

// TestTheAutoChainNeverPicksInBandProtocols checks that the automatic
// chain keeps iTerm2 out: it draws over the interface on terminals that
// keep images outside the text grid.
func TestTheAutoChainNeverPicksInBandProtocols(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "foot")

	chain := rendererChain()
	if chainHas(chain, ProtocolITerm2) {
		t.Error("the automatic chain picked the iTerm2 protocol")
	}
}

// TestTheConfigurationChoosesTheRenderer checks the artwork.renderer
// option: a configured renderer leads the chain, "none" switches the
// artwork off.
func TestTheConfigurationChoosesTheRenderer(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("ARTWORK_RENDERER", "")
	config.Set("artwork.renderer", "halfblock")
	defer config.Set("artwork.renderer", "auto")

	chain := rendererChain()
	if len(chain) == 0 || chain[0] != ProtocolHalfBlock {
		t.Errorf("the configured renderer did not lead the chain: %v", chain)
	}

	config.Set("artwork.renderer", "none")
	if chain := rendererChain(); len(chain) != 0 {
		t.Errorf("renderer none still returned a chain: %v", chain)
	}
}

// TestTheConfigurationIsVerified checks the Termux priority: a renderer
// the terminal does not advertise is ignored rather than half-working.
func TestTheConfigurationIsVerified(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	config.Set("artwork.renderer", "kitty")
	defer config.Set("artwork.renderer", "auto")

	chain := rendererChain()
	if chainHas(chain, ProtocolKitty) {
		t.Error("a terminal without the kitty protocol was given the configured kitty renderer")
	}
}

// TestTheConfigurationHonoursADetectedRenderer checks that an opt-in
// renderer the terminal actually advertises is honoured, with the
// detected protocols behind it as the fallback.
func TestTheConfigurationHonoursADetectedRenderer(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	config.Set("artwork.renderer", "iterm2")
	defer config.Set("artwork.renderer", "auto")

	chain := rendererChain()
	if len(chain) == 0 || chain[0] != ProtocolITerm2 {
		t.Errorf("the configured iTerm2 renderer was not honoured: %v", chain)
	}
	if !chainHas(chain, ProtocolNone) {
		t.Error("the configured chain lost the placeholder fallback")
	}
}

// TestAFailedProtocolFallsThrough checks the runtime fallback: a protocol
// that failed once is skipped, and the next one renders instead.
func TestAFailedProtocolFallsThrough(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	config.Set("artwork.renderer", "halfblock")
	defer func() {
		config.Set("artwork.renderer", "auto")
		resetRenderFailures()
	}()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}

	// A render that fails once is never asked again.
	markProtocolFailed(ProtocolHalfBlock)
	result := RenderAdaptive(img, 10, 6, "", "test://red")
	if result.Placeholder != "" {
		t.Errorf("a failed protocol still rendered: %q", result.Placeholder)
	}
	if !protocolFailed(ProtocolHalfBlock) {
		t.Error("the failure was not remembered")
	}

	// The placeholder is the last link of the chain: it renders nothing
	// and the caller shows the framed fallback.
	resetRenderFailures()
	result = RenderAdaptive(img, 10, 6, "", "test://red")
	if result.Placeholder == "" {
		t.Error("a healthy protocol failed to render")
	}
	if result.Protocol != ProtocolHalfBlock {
		t.Errorf("the render came from protocol %d, want half-block", result.Protocol)
	}
}

// TestTheITerm2Sequence checks the framing of the OSC 1337 output.
func TestTheITerm2Sequence(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	out := renderITerm2Cover(img, 2, 1)
	if !strings.HasPrefix(out, "\x1b]1337;File=inline=1;") {
		t.Errorf("the iTerm2 output lost its introducer: %q", out)
	}
	if !strings.HasSuffix(out, "\x07") {
		t.Errorf("the iTerm2 output lost its terminator: %q", out)
	}
	if !strings.Contains(out, "width=16px;height=16px") {
		t.Errorf("the iTerm2 output lost the pixel size: %q", out)
	}
	if renderITerm2Cover(nil, 2, 1) != "" {
		t.Error("a nil image rendered")
	}
}

// TestTheProcessedArtworkCache checks that a rendered cover is reused and
// that the cache stays bounded.
func TestTheProcessedArtworkCache(t *testing.T) {
	artworkCacheMu.Lock()
	artworkCache = map[string]ArtworkRender{}
	artworkKeys = nil
	artworkCacheMu.Unlock()

	key := "test://cover|1|10x6"
	cacheArtwork(key, ArtworkRender{Placeholder: "blocks"})
	if hit, ok := cachedArtwork(key); !ok || hit.Placeholder != "blocks" {
		t.Errorf("the cache lost the cover: %v %v", hit, ok)
	}

	for i := 0; i < artworkCacheLimit+8; i++ {
		cacheArtwork(fmt.Sprintf("key-%d", i), ArtworkRender{Placeholder: "x"})
	}
	artworkCacheMu.Lock()
	size := len(artworkCache)
	oldest := artworkKeys[0]
	artworkCacheMu.Unlock()
	if size > artworkCacheLimit {
		t.Errorf("the cache grew to %d entries", size)
	}
	if _, ok := cachedArtwork("key-0"); ok {
		t.Errorf("the oldest entry %q survived the eviction", oldest)
	}
}

// TestClearCoverCmd checks the cleanup of the images in terminal memory:
// a model with an image drops it, a model without one asks for nothing.
func TestClearCoverCmd(t *testing.T) {
	m := NewSearchModel()
	if m.clearCoverCmd() != nil {
		t.Error("a model without an image asked for a cleanup")
	}

	m.kittyImageID = 42
	cmd := m.clearCoverCmd()
	if cmd == nil {
		t.Fatal("a model with an image asked for no cleanup")
	}
	if m.kittyImageID != 0 {
		t.Errorf("the image id survived the cleanup: %d", m.kittyImageID)
	}
}
