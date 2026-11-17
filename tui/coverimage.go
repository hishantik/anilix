package tui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Protocol represents a terminal image rendering protocol.
type Protocol int

const (
	ProtocolNone      Protocol = iota // No image: the framed text placeholder
	ProtocolKitty                     // Kitty graphics protocol via Unicode placeholders
	ProtocolITerm2                    // iTerm2 inline images (OSC 1337)
	ProtocolHalfBlock                 // ANSI half-block characters (SGR, works everywhere)
	ProtocolChafa                     // chafa unicode art (universal fallback)
)

// DetectKittySupport checks if the terminal supports Kitty graphics protocol.
//
// WezTerm implements the Kitty graphics protocol together with the Unicode
// placeholders this renderer needs, so it belongs on the list too.
func DetectKittySupport() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	term, prog := termAndProgram()
	if prog == "kitty" || prog == "ghostty" || prog == "wezterm" {
		return true
	}
	for _, marker := range []string{"kitty", "ghostty", "wezterm"} {
		if strings.Contains(term, marker) {
			return true
		}
	}
	return false
}

// DetectTrueColorSupport checks if the terminal supports 24-bit color.
func DetectTrueColorSupport() bool {
	ct := os.Getenv("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	term, _ := termAndProgram()
	switch term {
	case "vt100", "vt220", "linux", "dumb", "":
		return false
	}
	return true
}

// termAndProgram returns the lower-cased TERM and TERM_PROGRAM values.
func termAndProgram() (term, prog string) {
	return strings.ToLower(os.Getenv("TERM")), strings.ToLower(os.Getenv("TERM_PROGRAM"))
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// coverCachePath returns the cache file path for a cover image URL.
func coverCachePath(url string) string {
	hash := sha256.Sum256([]byte(url))
	ext := ".png"
	if strings.Contains(url, ".jpg") || strings.Contains(url, ".jpeg") {
		ext = ".jpg"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".anilix", "cache", "covers")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, hex.EncodeToString(hash[:8])+ext)
}

// DownloadCoverImage downloads an image from a URL and returns both the decoded image
// and the local cache file path.
func DownloadCoverImage(url string) (image.Image, string, error) {
	cachePath := coverCachePath(url)

	// Try cache first
	if data, err := os.ReadFile(cachePath); err == nil {
		img, _, decErr := image.Decode(bytes.NewReader(data))
		if decErr == nil {
			return img, cachePath, nil
		}
	}

	data, err := curlGetBytes(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download cover: %w", err)
	}

	// Save to cache
	os.WriteFile(cachePath, data, 0o644)

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode cover image: %w", err)
	}

	return img, cachePath, nil
}

// renderKittyCover renders an image using Kitty graphics protocol with
// Unicode placeholders. The image is transmitted at high pixel resolution
// and the terminal's GPU scales it to fit the cell area, giving a crystal
// clear result.
func renderKittyCover(img image.Image, maxWidth int, maxRows int) KittyRenderResult {
	// Cell area: how many terminal cells the image occupies
	cols := maxWidth / 3
	if cols < 20 {
		cols = 20
	}
	if cols > 50 {
		cols = 50
	}
	rows := cols * 3 / 2 // ~2:3 aspect ratio for cover art
	if maxRows > 0 && rows > maxRows {
		rows = maxRows
	}

	// Pixel resolution: transmit at high res, terminal GPU scales to cell area.
	// Use the original image resolution (capped) for maximum clarity.
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	pixelW := srcW
	pixelH := srcH
	const maxPixel = 1200
	if srcW > maxPixel || srcH > maxPixel {
		ratio := float64(maxPixel) / float64(max(srcW, srcH))
		pixelW = int(float64(srcW) * ratio)
		pixelH = int(float64(srcH) * ratio)
	}
	if pixelW < 1 {
		pixelW = 1
	}
	if pixelH < 1 {
		pixelH = 1
	}
	resized := ResizeImage(img, pixelW, pixelH)

	return RenderKittyCover(resized, cols, rows)
}

// renderChafaCover renders an image as unicode art using the chafa binary.
func renderChafaCover(path string, width int) (string, error) {
	height := width * 2
	size := fmt.Sprintf("%dx%d", width, height)

	cmd := exec.Command("chafa",
		"--size="+size,
		"--format=symbols",
		"--color-space=rgb",
		"--work=9",
		"--align=left",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := strings.TrimRight(string(out), "\n")
	lines := strings.Split(result, "\n")
	if len(lines) > height {
		lines = lines[:height]
		result = strings.Join(lines, "\n")
	}

	return result, nil
}

// renderHalfBlock renders an image as ANSI-colored half-block characters.
// Each terminal row uses the ▀ character to display two vertical pixels,
// producing pure SGR text that survives Bubbletea v2's ultraviolet cell buffer.
func renderHalfBlock(img image.Image, maxWidth int, maxRows int) string {
	if maxRows <= 0 {
		maxRows = maxWidth * 2
	}
	if maxRows > 30 {
		maxRows = 30
	}
	resized := ResizeImage(img, maxWidth, maxRows)

	bounds := resized.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var sb strings.Builder
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := resized.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			var r2, g2, b2 uint32
			if y+1 < h {
				r2, g2, b2, _ = resized.At(bounds.Min.X+x, bounds.Min.Y+y+1).RGBA()
			}
			fg := fmt.Sprintf("\033[38;2;%d;%d;%dm", r1>>8, g1>>8, b1>>8)
			bg := fmt.Sprintf("\033[48;2;%d;%d;%dm", r2>>8, g2>>8, b2>>8)
			sb.WriteString(fg)
			sb.WriteString(bg)
			sb.WriteRune('\u2580')
		}
		sb.WriteString("\033[0m")
		if y+2 < h {
			sb.WriteRune('\n')
		}
	}
	sb.WriteString("\033[0m")
	return sb.String()
}

// ResizeImage resizes an image to fit within maxWidth x maxHeight while
// preserving aspect ratio, using high-quality CatmullRom scaling.
func ResizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	ratioW := float64(maxWidth) / float64(srcW)
	ratioH := float64(maxHeight) / float64(srcH)
	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	targetW := int(float64(srcW) * ratio)
	targetH := int(float64(srcH) * ratio)
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

// curlGetBytes downloads a URL and returns raw bytes using curl.
func curlGetBytes(url string) ([]byte, error) {
	cmd := exec.Command("curl", "-s", "-L",
		"-H", "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		url)
	return cmd.Output()
}
