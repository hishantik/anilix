package theme

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func TestEveryRoleHasHexAndName(t *testing.T) {
	for _, r := range AllRoles() {
		if !hexRe.MatchString(r.Hex()) {
			t.Errorf("role %d (%s) has invalid hex %q", r, r, r.Hex())
		}
		if r.String() == "unknown" {
			t.Errorf("role %d has no name", r)
		}
	}
}

func TestAllRolesCoversEveryRole(t *testing.T) {
	roles := AllRoles()
	if len(roles) != int(roleCount)-1 {
		t.Errorf("AllRoles returned %d roles, want %d", len(roles), roleCount-1)
	}
	seen := make(map[Role]bool, len(roles))
	for _, r := range roles {
		if r == RoleUnset {
			t.Error("the unset role must not be part of AllRoles")
		}
		if seen[r] {
			t.Errorf("role %d returned twice", r)
		}
		seen[r] = true
	}
}

func TestColorRespectsProfile(t *testing.T) {
	// Truecolor keeps the exact value.
	SetProfile(colorprofile.TrueColor)
	c := Primary.Color()
	if c == nil {
		t.Fatal("primary must have a color in truecolor mode")
	}
	r, g, b, _ := c.RGBA()
	if hexToRGB(Primary.Hex()) != [3]uint32{r >> 8, g >> 8, b >> 8} {
		t.Errorf("truecolor value not preserved: %v", c)
	}

	// ANSI256 downsamples but stays a colour.
	SetProfile(colorprofile.ANSI256)
	if c := Primary.Color(); c == nil {
		t.Error("primary must have a color in 256 color mode")
	}

	// ANSI keeps the 16 colour palette.
	SetProfile(colorprofile.ANSI)
	if c := Primary.Color(); c == nil {
		t.Error("primary must have a color in 16 color mode")
	}

	// No colour support means no colour at all.
	SetProfile(colorprofile.ASCII)
	if c := Primary.Color(); c != nil {
		t.Errorf("expected no color without color support, got %v", c)
	}
	if HasColor() {
		t.Error("HasColor must be false without color support")
	}
	SetProfile(colorprofile.TrueColor)
}

func TestContrastRatiosAreReadable(t *testing.T) {
	SetProfile(colorprofile.TrueColor)

	pairs := []struct {
		name string
		fg   Role
		bg   Role
		min  float64
	}{
		{"text on background", Text, Background, 7.0},
		{"text on surface", Text, Surface, 7.0},
		{"muted on surface", Muted, Surface, 4.5},
		{"primary on background", Primary, Background, 4.5},
		{"on-primary on primary", OnPrimary, Primary, 7.0},
		{"on-primary-container on primary-container", OnPrimaryContainer, PrimaryContainer, 4.5},
		{"on-secondary-container on secondary-container", OnSecondaryContainer, SecondaryContainer, 4.5},
		{"on-tertiary-container on tertiary-container", OnTertiaryContainer, TertiaryContainer, 4.5},
		{"on-error-container on error-container", OnErrorContainer, ErrorContainer, 4.5},
		{"error on surface", Error, Surface, 4.5},
		{"disabled on surface", Disabled, Surface, 2.0},
	}

	for _, p := range pairs {
		ratio := contrast(p.fg.Color(), p.bg.Color())
		if ratio < p.min {
			t.Errorf("%s: contrast %.2f below minimum %.2f", p.name, ratio, p.min)
		}
	}
}

func TestSpacingScaleIsMonotonic(t *testing.T) {
	spaces := AllSpaces()
	if len(spaces) != 9 {
		t.Fatalf("expected 9 spacing steps, got %d", len(spaces))
	}
	prev := -1
	for _, s := range spaces {
		if s.Cells() <= prev && s != Space0 {
			t.Errorf("spacing %s (%d) is not greater than the previous step (%d)", s, s.Cells(), prev)
		}
		prev = s.Cells()
	}
	if Space3.Cells() != 4 || Space4.Cells() != 8 || Space6.Cells() != 16 {
		t.Errorf("spacing scale lost its 4dp rhythm: %d %d %d", Space3.Cells(), Space4.Cells(), Space6.Cells())
	}
}

func TestIconFallbacks(t *testing.T) {
	// Every icon must have an ASCII fallback of the same width, so the
	// fallback never shifts the layout.
	for icon, fallback := range asciiFallbacks {
		want := IconWidth(icon)
		if got := len([]rune(icon)); got != want {
			t.Errorf("icon %q is %d cells, want %d", icon, got, want)
		}
		if got := len([]rune(fallback)); got != want {
			t.Errorf("fallback %q for icon %q is %d cells, want %d", fallback, icon, got, want)
		}
		for _, r := range fallback {
			if r > 127 {
				t.Errorf("fallback %q for icon %q is not ASCII", fallback, icon)
			}
		}
	}

	// Every unicode icon must have a fallback entry.
	for _, icon := range []string{
		IconMenu, IconMenuItem, IconSearch, IconChevronRight, IconChevronDown,
		IconCheck, IconClose, IconSettings, IconPlay, IconPause, IconEpisode,
		IconStar, IconDot, IconWarning, IconInfo, IconExternal, IconLock,
		IconSelectionBar, IconDivider, IconTabActive,
	} {
		if _, ok := asciiFallbacks[icon]; !ok {
			t.Errorf("icon %q has no ascii fallback", icon)
		}
	}
}

func TestTypeScaleRenders(t *testing.T) {
	SetProfile(colorprofile.TrueColor)
	for _, typ := range AllTypes() {
		if typ.String() == "unknown" {
			t.Errorf("type %d has no name", typ)
		}
		out := typ.Render("Sample")
		if !strings.Contains(stripANSI(out), "Sample") {
			t.Errorf("type %s did not render its text: %q", typ, out)
		}
	}
}

func TestRoleUnsetHasNoColor(t *testing.T) {
	SetProfile(colorprofile.TrueColor)

	if RoleUnset.Hex() != "" {
		t.Errorf("the unset role must have no hex value, got %q", RoleUnset.Hex())
	}
	if RoleUnset.Color() != nil {
		t.Error("the unset role must not resolve to a colour")
	}
	if RoleUnset.String() != "unset" {
		t.Errorf("the unset role must be named unset, got %q", RoleUnset.String())
	}
}

func TestStatesAreNamed(t *testing.T) {
	for _, s := range AllStates() {
		if s.String() == "unknown" {
			t.Errorf("state %d has no name", s)
		}
	}
	if StateDisabled.Enabled() {
		t.Error("the disabled state must not be enabled")
	}
	if !StateDefault.Enabled() {
		t.Error("the default state must be enabled")
	}
	if !StateHover.Interactive() || StateSelected.Interactive() {
		t.Error("only hover, pressed and focused are interactive states")
	}
}

// hexToRGB converts "#rrggbb" to components.
func hexToRGB(hex string) [3]uint32 {
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		return [3]uint32{}
	}
	return [3]uint32{uint32((v >> 16) & 0xff), uint32((v >> 8) & 0xff), uint32(v & 0xff)}
}

// stripANSI removes escape sequences so tests can assert on plain text.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}
