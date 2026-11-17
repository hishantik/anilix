package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
)

// The in-band image protocols.
//
// Both renderers here produce text: escape sequences the terminal
// interprets where the interface prints them. They are opt-in - the
// detected chain never picks them - because terminals that keep images
// outside of the text grid draw them over the interface on repaint.
// See renderer.go for the chain and the fallback.

// cellPixelWidth and cellPixelHeight are the pixels one terminal cell is
// assumed to have. Terminals differ, and the protocols have no way to ask
// for the true size: the picture is sized by the pixel count and a
// terminal with another cell size shows it slightly smaller or larger.
const (
	cellPixelWidth  = 8
	cellPixelHeight = 16
)

// DetectSixelSupport reports whether the terminal advertises SIXEL.
//
// The check is deliberately narrow. A converter on the PATH is not a
// terminal capability: the name of the terminal is the only static
// evidence here, and an unspecific name claims nothing. The strongest
// evidence is dynamic: the terminal's own answer to the device attributes
// query, where attribute 4 is the SIXEL bit - the way Termux advertises
// its graphics support in the builds that carry it.
func DetectSixelSupport() bool {
	if sixelAdvertised() {
		return true
	}
	term, prog := termAndProgram()
	for _, marker := range []string{"sixel", "foot", "mlterm", "wezterm", "contour", "rio"} {
		if strings.Contains(term, marker) || strings.Contains(prog, marker) {
			return true
		}
	}
	// xterm claims SIXEL in its terminfo entry only when it was built
	// with it; the entry is part of the name then.
	return strings.Contains(term, "xterm") && strings.Contains(term, "sixel")
}

// DetectITerm2Support reports whether the terminal speaks the OSC 1337
// inline-image protocol of iTerm2.
func DetectITerm2Support() bool {
	_, prog := termAndProgram()
	return prog == "iterm.app" || prog == "wezterm"
}

// renderSixelCover renders a cover as a SIXEL image for the given cell
// size. The result is in-band text: the terminal draws the pixels where
// the interface prints the sequence.
//
// It first tries chafa, which keeps the full 24-bit palette and dithers,
// falling back to the native 3-3-2 encoder when chafa is missing or
// fails. Both paths draw the same pixel grid (cols*8 rows*16), so the
// raster covers exactly the same cells either way.
func renderSixelCover(img image.Image, cols, rows int) string {
	if img == nil || cols < 1 || rows < 1 {
		return ""
	}
	if hasBinary("chafa") {
		if out, err := runChafaSixel(img, cols, rows); err == nil && out != "" {
			return out
		}
	}
	// Fallback: native encoder
	pixelW := cols * cellPixelWidth
	pixelH := rows * cellPixelHeight
	return encodeSixel(ResizeImage(img, pixelW, pixelH))
}

// runChafaSixel renders an image as SIXEL through the chafa binary.
//
// The image is resized to the same pixel grid the native encoder paints,
// then piped to chafa on stdin. "--exact-size=on" keeps the raster at
// those source pixels and "--polite=on" suppresses the cursor hide/show
// that would otherwise bookend the stream, so the sixel stays clean
// in-band text from "ESC P" to "ESC \".
func runChafaSixel(img image.Image, cols, rows int) (string, error) {
	pixelW := cols * cellPixelWidth
	pixelH := rows * cellPixelHeight
	resized := ResizeImage(img, pixelW, pixelH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", err
	}

	// chafa reads the image from stdin when the path is "-". The sixel
	// format is sixels (sixel is accepted as an alias).
	cmd := exec.Command("chafa",
		"--polite=on",
		"--probe=off",
		"-f", "sixels",
		"--exact-size=on",
		"--work=9",
		"-",
	)
	cmd.Stdin = &buf
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// encodeSixel encodes an image as SIXEL.
//
// The picture is cut into bands of six rows; a colour paints its pixels
// of a band column by column, each column being one byte of six bits.
// Runs of equal columns are compressed with the repeat introducer. The
// colours come from a fixed 3-3-2 palette: two bits of blue are lost,
// which is invisible at the size of a cover.
func encodeSixel(img image.Image) string {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1 || h < 1 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\x1bPq")

	used := map[uint8]bool{}
	pixels := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if a < 0x8000 {
				continue // transparent: the band background stays
			}
			idx := sixelColor(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			pixels[y*w+x] = idx + 1 // 0 means transparent
			used[idx] = true
		}
	}

	// The palette, in the order the colours first appear.
	colors := make([]uint8, 0, len(used))
	for idx := range used {
		colors = append(colors, idx)
	}
	for _, idx := range colors {
		r, g, b := sixelPalette(idx)
		fmt.Fprintf(&sb, "#%d;2;%d;%d;%d", idx, r, g, b)
	}

	for band := 0; band < h; band += 6 {
		bandH := 6
		if band+bandH > h {
			bandH = h - band
		}
		for i, idx := range colors {
			if i > 0 {
				sb.WriteByte('$') // carriage return: back to the band start
			}
			fmt.Fprintf(&sb, "#%d", idx)
			run := byte(0)
			runLen := 0
			flush := func() {
				if runLen == 0 {
					return
				}
				char := byte(63 + run)
				if runLen >= 3 {
					fmt.Fprintf(&sb, "!%d%c", runLen, char)
				} else {
					for j := 0; j < runLen; j++ {
						sb.WriteByte(char)
					}
				}
				run = 0
				runLen = 0
			}
			for x := 0; x < w; x++ {
				var bits byte
				for k := 0; k < bandH; k++ {
					if pixels[(band+k)*w+x] == idx+1 {
						bits |= 1 << k
					}
				}
				if bits == run && runLen > 0 {
					runLen++
					continue
				}
				flush()
				run = bits
				runLen = 1
			}
			flush()
		}
		if band+6 < h {
			sb.WriteByte('-') // next band
		}
	}

	sb.WriteString("\x1b\\")
	return sb.String()
}

// sixelColor reduces a colour to the 3-3-2 palette index: five bits of
// red, three of green, two of blue.
func sixelColor(r, g, b uint8) uint8 {
	return (r>>5)<<5 | (g>>5)<<2 | b>>6
}

// sixelPalette returns the RGB values of a 3-3-2 palette index.
func sixelPalette(idx uint8) (r, g, b uint8) {
	return (idx >> 5) << 5, ((idx >> 2) & 0x07) << 5, (idx & 0x03) << 6
}

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
