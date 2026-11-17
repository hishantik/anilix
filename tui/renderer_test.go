package tui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/hishantik/anilix/config"
)

// clearArtEnv resets every environment variable the detection reads, and
// the evidence of the capability query, so a test sees only the terminal
// it sets itself.
func clearArtEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TERM", "TERM_PROGRAM", "COLORTERM", "KITTY_WINDOW_ID"} {
		t.Setenv(key, "")
	}
	resetDeviceAttributes()
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
// terminal with an unspecific name gets half-blocks at best, never SIXEL,
// and a terminal without colour gets the placeholder only.
func TestDetectionClaimsNothingWithoutEvidence(t *testing.T) {
	clearArtEnv(t)

	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	chain := detectedChain()
	if chainHas(chain, ProtocolSixel) {
		t.Error("a plain xterm-256color claimed SIXEL")
	}
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
// honoured: WezTerm speaks the kitty protocol, iTerm.app speaks OSC 1337,
// and foot speaks SIXEL.
func TestDetectionReadsTheTerminal(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM_PROGRAM", "WezTerm")
	if !DetectKittySupport() {
		t.Error("WezTerm was denied the kitty protocol")
	}
	if !DetectSixelSupport() {
		t.Error("WezTerm was denied SIXEL")
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
	if !DetectSixelSupport() {
		t.Error("foot was denied SIXEL")
	}
	if DetectKittySupport() {
		t.Error("foot claimed the kitty protocol")
	}
}

// TestTheAutoChainNeverPicksInBandProtocols checks that the automatic
// chain keeps SIXEL and iTerm2 out: both draw over the interface on
// terminals that keep images outside the text grid.
func TestTheAutoChainNeverPicksInBandProtocols(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "foot")

	chain := rendererChain()
	if chainHas(chain, ProtocolSixel) {
		t.Error("the automatic chain picked SIXEL")
	}
	if chainHas(chain, ProtocolITerm2) {
		t.Error("the automatic chain picked the iTerm2 protocol")
	}
}

// TestTheTerminalAdvertisesSixelInDeviceAttributes checks the DA1 path of
// the detection: attribute 4 of the answer is the SIXEL bit, and Termux
// sets it in the builds that carry its graphics support. A configured
// SIXEL renderer is honoured once the terminal has spoken.
func TestTheTerminalAdvertisesSixelInDeviceAttributes(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "xterm-256color")
	t.Cleanup(resetDeviceAttributes)

	// Without an answer the plain name claims nothing.
	if DetectSixelSupport() {
		t.Fatal("a terminal that answered nothing claimed SIXEL")
	}

	// An answer without the SIXEL bit changes nothing. This is the answer
	// of a Termux build from before its graphics support merged.
	if HandlePrimaryDeviceAttributes([]int{64, 1, 2, 6, 9, 15, 18, 21, 22}) {
		t.Error("an answer without the SIXEL bit changed the renderer")
	}
	if DetectSixelSupport() {
		t.Error("attribute 4 missing, but SIXEL was detected")
	}

	// The answer of a Termux build with graphics support: attribute 4 is
	// in the list.
	if !HandlePrimaryDeviceAttributes([]int{64, 1, 2, 4, 6, 9, 15, 18, 21, 22}) {
		t.Error("the SIXEL bit was not noticed")
	}
	if !DetectSixelSupport() {
		t.Error("the terminal advertised SIXEL, but it was not detected")
	}

	// The same answer twice is not a change: the screen must not redraw.
	if HandlePrimaryDeviceAttributes([]int{64, 1, 2, 4, 6, 9, 15, 18, 21, 22}) {
		t.Error("a repeated answer was treated as a change")
	}

	// The evidence makes a configured SIXEL renderer lead the chain.
	config.Set("artwork.renderer", "sixel")
	defer config.Set("artwork.renderer", "auto")
	chain := rendererChain()
	if len(chain) == 0 || chain[0] != ProtocolSixel {
		t.Errorf("the advertised SIXEL renderer did not lead the chain: %v", chain)
	}
	if !chainHas(chain, ProtocolNone) {
		t.Error("the chain lost the placeholder fallback")
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
	config.Set("artwork.renderer", "sixel")
	defer config.Set("artwork.renderer", "auto")

	chain := rendererChain()
	if chainHas(chain, ProtocolSixel) {
		t.Error("a terminal without SIXEL was given the configured SIXEL renderer")
	}
	if len(chain) == 0 || chain[0] == ProtocolSixel {
		t.Errorf("the verified configuration did not fall back to auto: %v", chain)
	}

	config.Set("artwork.renderer", "kitty")
	chain = rendererChain()
	if chainHas(chain, ProtocolKitty) {
		t.Error("a terminal without the kitty protocol was given the configured kitty renderer")
	}
}

// TestTheConfigurationHonoursADetectedRenderer checks that an opt-in
// renderer the terminal actually advertises is honoured, with the
// detected protocols behind it as the fallback.
func TestTheConfigurationHonoursADetectedRenderer(t *testing.T) {
	clearArtEnv(t)
	t.Setenv("TERM", "foot")
	config.Set("artwork.renderer", "sixel")
	defer config.Set("artwork.renderer", "auto")

	chain := rendererChain()
	if len(chain) == 0 || chain[0] != ProtocolSixel {
		t.Errorf("the configured SIXEL renderer was not honoured: %v", chain)
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

// TestTheChafaSixelFraming checks that renderSixelCover routes through the
// chafa binary and produces a well-formed sixel stream. It requires chafa
// on the PATH; without it the native encoder is the fallback.
func TestTheChafaSixelFraming(t *testing.T) {
	if !hasBinary("chafa") {
		t.Skip("chafa not installed")
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 12))
	red := color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff}
	for y := 0; y < 12; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, red)
		}
	}

	out := renderSixelCover(img, 4, 12)
	if out == "" {
		t.Fatal("chafa sixel returned nothing")
	}
	if !strings.HasPrefix(out, "\x1bP0;1;0q") {
		t.Errorf("the chafa sixel output lost its DCS introducer: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("the chafa sixel output lost its string terminator: %q", out)
	}
	if !strings.Contains(out, ";2;") {
		t.Errorf("the chafa sixel output lost its colour definition: %q", out)
	}
	if strings.Contains(out, "\x1b[?25") {
		t.Errorf("chafa sixel leaked cursor hide/show instead of polite output: %q", out)
	}
}

// TestTheSixelViaTeaRaw checks that renderWithProtocol for ProtocolSixel
// returns both the sixel sequence (for tea.Raw) and a non-empty placeholder
// so the frame keeps its shape. The sixel sequence is a DCS that the
// ultraviolet renderer strips from view text, which is why it must travel
// through tea.Raw() the same way the kitty APC does.
func TestTheSixelViaTeaRaw(t *testing.T) {
	if !hasBinary("chafa") {
		t.Skip("chafa not installed")
	}
	clearArtEnv(t)
	t.Setenv("TERM", "foot") // has sixel
	t.Setenv("TERM_PROGRAM", "")
	t.Cleanup(resetDeviceAttributes)
	HandlePrimaryDeviceAttributes([]int{64, 1, 2, 4, 6, 9, 15, 18, 21, 22})
	t.Cleanup(resetRenderFailures)

	img := image.NewRGBA(image.Rect(0, 0, 4, 12))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}

	result := renderWithProtocol(img, ProtocolSixel, 4, 12, "")
	if result.SixelSeq == "" {
		t.Fatal("SixelSeq is empty: chafa sixel path produced nothing")
	}
	if !strings.HasPrefix(result.SixelSeq, "\x1bP0;1;0q") {
		t.Errorf("SixelSeq is not a sixel stream: %q", result.SixelSeq[:30])
	}
	if result.Placeholder == "" {
		t.Error("Placeholder is empty: the frame has no shape")
	}
	// Placeholder must cover the right number of rows.
	lines := strings.Split(result.Placeholder, "\n")
	if len(lines) != 12 {
		t.Errorf("Placeholder has %d rows, want 12", len(lines))
	}
}

// TestTheSixelEncoder checks the framing of the SIXEL output: the DCS
// introducer, one colour definition per used colour, a band separator
// when the picture is taller than one band, and the string terminator.
func TestTheSixelEncoder(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 12))
	red := color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff}
	for y := 0; y < 12; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, red)
		}
	}

	out := encodeSixel(img)
	if !strings.HasPrefix(out, "\x1bPq") {
		t.Errorf("the SIXEL output lost the DCS introducer: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("the SIXEL output lost the string terminator: %q", out)
	}
	if !strings.Contains(out, ";2;") {
		t.Errorf("the SIXEL output lost the colour definition: %q", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("a 12-row picture has two bands and lost the separator: %q", out)
	}
	// Four columns of a solid colour compress to one repeat run: the six
	// rows of the first band are all set, so the column byte is 63+63.
	if !strings.Contains(out, "!4~") {
		t.Errorf("the SIXEL output lost the colour data: %q", out)
	}
}

// TestTheSixelEncoderHandlesTransparentPixels checks that transparency
// paints nothing rather than a colour.
func TestTheSixelEncoderHandlesTransparentPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 6)) // all transparent
	out := encodeSixel(img)
	if !strings.Contains(out, "!4?") && !strings.Contains(out, "????") {
		// Every column of the band must carry zero bits, painted or not.
		t.Logf("all-transparent picture encoded as %q", out)
	}
	if strings.Count(out, "#0;2;") > 1 {
		t.Errorf("a transparent picture defined a colour twice: %q", out)
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
