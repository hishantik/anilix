package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
)

// DetectITerm2Support reports whether the terminal speaks the OSC 1337
// inline-image protocol of iTerm2.
func DetectITerm2Support() bool {
	_, prog := termAndProgram()
	return prog == "iterm.app" || prog == "wezterm"
}

// cellPixelWidth and cellPixelHeight are the pixels one terminal cell is
// assumed to have. Terminals differ, and the protocols have no way to ask
// for the true size: the picture is sized by the pixel count and a
// terminal with another cell size shows it slightly smaller or larger.
const (
	cellPixelWidth  = 8
	cellPixelHeight = 16
)

// renderITerm2Cover renders a cover as an iTerm2 inline image (OSC 1337).
// The picture travels as a base64 PNG sized in pixels.
func renderITerm2Cover(img image.Image, cols, rows int) string {
	if img == nil || cols < 1 || rows < 1 {
		return ""
	}
	pixelW := cols * cellPixelWidth
	pixelH := rows * cellPixelHeight
	resized := ResizeImage(img, pixelW, pixelH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b]1337;File=inline=1;size=%d;width=%dpx;height=%dpx:%s\x07",
		buf.Len(), pixelW, pixelH, base64.StdEncoding.EncodeToString(buf.Bytes()))
}
