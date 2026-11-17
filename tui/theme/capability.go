package theme

import (
	"image/color"
	"math"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

var (
	capOnce sync.Once
	profile colorprofile.Profile
	unicode bool

	convertCache sync.Map // hex string -> color.Color
)

// SetProfile overrides the detected color profile.
//
// It exists for two cases: tests, which run without a terminal, and the
// theme showcase, which is often piped to a file or a pager and would
// otherwise lose its colour.
func SetProfile(p colorprofile.Profile) {
	// Run detection first so the unicode capability is still resolved.
	capOnce.Do(detect)
	profile = p
	convertCache = sync.Map{}
}

// ForceTrueColor switches the design system to 24-bit colour regardless of
// what the terminal reports.
func ForceTrueColor() {
	SetProfile(colorprofile.TrueColor)
}

// Profile returns the detected color profile of the terminal.
//
// Detection is done once per process: the profile is a property of the
// terminal, not of the screen being rendered.
func Profile() colorprofile.Profile {
	capOnce.Do(detect)
	return profile
}

// detect resolves the terminal capabilities.
func detect() {
	profile = colorprofile.Detect(os.Stdout, os.Environ())
	if profile == colorprofile.Unknown || profile == colorprofile.NoTTY {
		// Not a terminal (tests, pipes): fall back to environment detection so
		// the palette still renders with colour in captured output.
		if p := colorprofile.Env(os.Environ()); p != colorprofile.Unknown {
			profile = p
		}
	}
	unicode = detectUnicode(os.Environ())
}

// detectUnicode reports whether the terminal can render box drawing and
// symbol characters.
func detectUnicode(env []string) bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		for _, kv := range env {
			if !strings.HasPrefix(kv, key+"=") {
				continue
			}
			v := strings.ToUpper(kv[len(key)+1:])
			if strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8") {
				return true
			}
		}
	}
	// No locale set: assume UTF-8, which is the norm for modern terminals.
	return true
}

// HasUnicode reports whether unicode box drawing and symbols are safe to use.
func HasUnicode() bool {
	Profile()
	return unicode
}

// SetUnicode overrides the detected unicode capability.
//
// It exists for the same reason as SetProfile: tests and the showcase run
// without a terminal, and the fallback path of a terminal that cannot draw
// box characters has to be reproducible.
func SetUnicode(ok bool) {
	Profile()
	unicode = ok
}

// HasColor reports whether the terminal can display colour at all.
func HasColor() bool {
	return Profile() > colorprofile.ASCII
}

// HasTrueColor reports whether the terminal supports 24-bit colour.
func HasTrueColor() bool {
	return Profile() == colorprofile.TrueColor
}

// colorFor converts a hex colour to one the terminal can display.
//
// The result is nil when the terminal has no colour support, or when the
// colour cannot be represented.
func colorFor(hex string) color.Color {
	Profile()
	if !HasColor() {
		return nil
	}
	if cached, ok := convertCache.Load(hex); ok {
		c, _ := cached.(color.Color)
		return c
	}
	c := profile.Convert(lipgloss.Color(hex))
	if c == nil {
		return nil
	}
	convertCache.Store(hex, c)
	return c
}

// contrast returns the WCAG contrast ratio between two colours, used by the
// tests to keep the palette readable.
func contrast(a, b color.Color) float64 {
	if a == nil || b == nil {
		return 0
	}
	ra, ga, ba := linearize(a)
	rb, gb, bb := linearize(b)
	l1 := 0.2126*ra + 0.7152*ga + 0.0722*ba
	l2 := 0.2126*rb + 0.7152*gb + 0.0722*bb
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func linearize(c color.Color) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return channel(r), channel(g), channel(b)
}

func channel(v uint32) float64 {
	f := float64(v) / 65535.0
	if f <= 0.03928 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}
