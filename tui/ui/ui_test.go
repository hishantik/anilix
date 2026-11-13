package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/hishantik/anilix/tui/theme"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// strip removes the escape sequences so the visible text can be measured.
func strip(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// visibleWidth returns the number of cells the string occupies.
func visibleWidth(s string) int {
	return lipgloss.Width(strip(s))
}

func TestComponentsRenderWithoutPanic(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	_ = Button{Label: "Filled", Variant: ButtonFilled}.Render()
	_ = Button{Label: "Tonal", Variant: ButtonTonal}.Render()
	_ = Button{Label: "Outline", Variant: ButtonOutlined}.Render()
	_ = Button{Label: "Text", Variant: ButtonText}.Render()
	_ = Button{Label: "Disabled", Variant: ButtonFilled, State: theme.StateDisabled}.Render()
	_ = Button{Label: "With hint", Variant: ButtonOutlined, Shortcut: "ctrl+s", Icon: theme.Icon(theme.IconSettings)}.Render()
	_ = Chip{Label: "TV"}.Render()
	_ = Chip{Label: "TV", Container: theme.SecondaryContainer}.Render()
	_ = Badge{Label: "3"}.Render()
	_ = Card{Title: "Title", Subtitle: "Subtitle", Body: "Body", Width: 40}.Render()
	_ = MenuItem{Label: "Item", Selected: true}.Render(30)
	_ = MenuItem{Label: "Item", Disabled: true}.Render(30)
	_ = Tab{Label: "Tab", Selected: true}.Render()
	_ = TabBar([]Tab{{Label: "One"}, {Label: "Two", Selected: true}})
	_ = ListItem{Title: "Item", Description: "Desc", Selected: true}.Render(40)
	_ = Dialog{Title: "Title", Body: "Body", Width: 40}.Render()
	_ = Divider(false, 20)
	_ = SectionHeader("Section", "3", 40)
	_ = KeyHint("enter", "play")
	_ = InputField{Placeholder: "Search anime...", Width: 40}.Render()
	_ = InputField{Content: "one piece", Focused: true, Width: 40}.Render()
	_ = InfoRow("Type", "TV")
	_ = ChipRow([]string{"Action", "Drama"}, 40, theme.SecondaryContainer)
	_ = EmptyState{Title: "No results"}.Render(40)
	_ = ErrorState{Title: "Failed", Message: "boom"}.Render(40)
	_ = ProgressBar(50, 20)
	_ = LoadingIndicator("Searching", 1)
	_ = Row([]string{"a", "b"}, theme.Space2)
}

func TestComponentsRenderWithoutColorSupport(t *testing.T) {
	theme.SetProfile(colorprofile.ASCII)
	defer theme.SetProfile(colorprofile.TrueColor)

	cases := []struct {
		name string
		out  string
		want string
	}{
		{"button", Button{Label: "Filled", Variant: ButtonFilled}.Render(), "Filled"},
		{"list item", ListItem{Title: "Item", Description: "Desc"}.Render(40), "Item"},
		{"chip", Chip{Label: "TV"}.Render(), "TV"},
		{"badge", Badge{Label: "3"}.Render(), "3"},
		{"menu item", MenuItem{Label: "Item"}.Render(30), "Item"},
		{"empty state", EmptyState{Title: "Nothing"}.Render(40), "Nothing"},
		{"error state", ErrorState{Title: "Failed"}.Render(40), "Failed"},
	}
	for _, c := range cases {
		if !strings.Contains(strip(c.out), c.want) {
			t.Errorf("%s lost its text without color: %q", c.name, c.out)
		}
	}

	// The progress bar has no text, but it must still be exactly as wide as
	// it was asked to be.
	if got := visibleWidth(ProgressBar(50, 10)); got != 10 {
		t.Errorf("progress bar is %d cells, want 10", got)
	}
}

func TestCardRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 44
	out := Card{
		Title:     "Demon Slayer",
		Subtitle:  "TV · 2019",
		Body:      "A long body that has to wrap inside the card.",
		Elevation: theme.Elevation1,
		Width:     width,
	}.Render()

	for i, line := range strings.Split(strip(out), "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Errorf("card line %d is %d cells, want %d: %q", i, got, width, line)
		}
	}
}

func TestDialogRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 44
	out := Dialog{
		Title: "Quit Anilix?",
		Body:  "Are you sure?",
		Actions: []string{
			Button{Label: "Yes", Variant: ButtonFilled}.Render(),
			Button{Label: "No", Variant: ButtonText}.Render(),
		},
		Width:     width,
		Elevation: theme.Elevation3,
	}.Render()

	for i, line := range strings.Split(strip(out), "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Errorf("dialog line %d is %d cells, want %d: %q", i, got, width, line)
		}
	}
}

func TestListItemRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 40
	items := []ListItem{
		{Title: "One Piece", Description: "TV", Trailing: "8.73"},
		{Title: "One Piece", Description: "TV", Trailing: "8.73", Selected: true, Focused: true},
		{Title: "One Piece", Description: "TV", Trailing: "8.73", Selected: true},
		{Title: "One Piece", Description: "TV", Disabled: true},
	}
	for _, item := range items {
		out := item.Render(width)
		for i, line := range strings.Split(strip(out), "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("list item line %d is %d cells, want %d: %q", i, got, width, line)
			}
		}
	}
}

func TestMenuItemRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 30
	items := []MenuItem{
		{Label: "Search", Hint: "10"},
		{Label: "Search", Hint: "10", Selected: true, Focused: true},
		{Label: "Search", Disabled: true},
	}
	for _, item := range items {
		out := item.Render(width)
		if got := visibleWidth(out); got != width {
			t.Errorf("menu item is %d cells, want %d: %q", got, width, out)
		}
	}
}

func TestFilledRowKeepsItsBackground(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	// Every segment of a filled row has to carry the fill itself: Lip Gloss
	// closes a styled run with a reset, and a reset would clear the fill of
	// everything that follows it.
	out := ListItem{
		Title:       "One Piece",
		Description: "TV",
		Trailing:    "8.73",
		Selected:    true,
		Focused:     true,
	}.Render(40)

	bg := theme.SurfaceVariant.Color()
	if bg == nil {
		t.Fatal("expected a surface colour in truecolor mode")
	}

	// The fill is part of the escape sequence of every segment, whether it
	// carries a colour of its own or not.
	r, g, b, _ := bg.RGBA()
	code := fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)
	if got := strings.Count(out, code); got < 5 {
		t.Errorf("expected the fill on every segment, found %d occurrences of %q", got, code)
	}
}

func TestButtonVariantsUseDistinctContainers(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	filled := Button{Label: "A", Variant: ButtonFilled}.Render()
	tonal := Button{Label: "A", Variant: ButtonTonal}.Render()
	outlined := Button{Label: "A", Variant: ButtonOutlined}.Render()
	text := Button{Label: "A", Variant: ButtonText}.Render()

	if filled == tonal || tonal == outlined || outlined == text {
		t.Error("button variants must not render identically")
	}
}

func TestDisabledButtonIsNotFilled(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	enabled := Button{Label: "A", Variant: ButtonFilled}.Render()
	disabled := Button{Label: "A", Variant: ButtonFilled, State: theme.StateDisabled}.Render()

	if enabled == disabled {
		t.Error("a disabled button must differ from an enabled one")
	}
	if !strings.Contains(strip(disabled), "A") {
		t.Errorf("disabled button lost its label: %q", disabled)
	}
}

func TestBreakpointsFollowTheWidth(t *testing.T) {
	cases := []struct {
		width int
		want  Breakpoint
	}{
		{40, Compact},
		{59, Compact},
		{60, Medium},
		{99, Medium},
		{100, Expanded},
		{200, Expanded},
	}
	for _, c := range cases {
		if got := BreakpointOf(c.width); got != c.want {
			t.Errorf("width %d: got %s, want %s", c.width, got, c.want)
		}
	}
}

func TestCompactLayoutHasNoPreview(t *testing.T) {
	if PreviewWidth(40) != 0 {
		t.Error("a compact terminal must not reserve a preview column")
	}
	if PreviewWidth(120) == 0 {
		t.Error("an expanded terminal must reserve a preview column")
	}
	if got := ListWidth(120); got+PreviewWidth(120)+PanelGutter() != 120 {
		t.Errorf("list and preview do not fill the terminal: %d + %d", got, PreviewWidth(120))
	}
}

func TestOverlayWidthIsClampedToTheTerminal(t *testing.T) {
	// A 44 column dialog on a phone must fit.
	if got := OverlayWidth(44, 40); got > 36 {
		t.Errorf("overlay width %d overflows a 40 column terminal", got)
	}
	if got := OverlayWidth(44, 120); got != 44 {
		t.Errorf("overlay width %d, want 44", got)
	}
	if got := OverlayWidth(44, 20); got < 20 {
		t.Errorf("overlay width %d is unusably narrow", got)
	}
}

func TestSectionHeaderAlignsTrailingValue(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	out := SectionHeader("Section", "3 items", 40)
	if got := visibleWidth(out); got != 40 {
		t.Errorf("section header is %d cells, want 40: %q", got, out)
	}
}

func TestInputFieldRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 40
	fields := []InputField{
		{Placeholder: "Search anime...", Width: width},
		{Content: "one piece", Focused: true, Width: width},
		{Content: "one piece", Width: width},
	}

	for _, field := range fields {
		out := field.Render()
		for i, line := range strings.Split(strip(out), "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("input field line %d is %d cells, want %d: %q", i, got, width, line)
			}
		}
	}
}

func TestInputFieldShowsItsPlaceholder(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	out := strip(InputField{Placeholder: "Search anime...", Width: 40}.Render())
	if !strings.Contains(out, "Search anime...") {
		t.Errorf("the field lost its placeholder: %q", out)
	}

	out = strip(InputField{Content: "one piece", Placeholder: "Search anime...", Width: 40}.Render())
	if strings.Contains(out, "Search anime...") {
		t.Errorf("the field shows the placeholder next to its value: %q", out)
	}
	if !strings.Contains(out, "one piece") {
		t.Errorf("the field lost its value: %q", out)
	}
}

func TestInfoRowAlignsTheValues(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	short := strip(InfoRow("Type", "TV"))
	long := strip(InfoRow("Episodes", "28"))

	if !strings.HasPrefix(short, "Type") || !strings.HasPrefix(long, "Episodes") {
		t.Fatalf("the rows lost their labels: %q / %q", short, long)
	}
	if strings.Index(short, "TV") != strings.Index(long, "28") {
		t.Errorf("the values are not aligned: %q / %q", short, long)
	}
}

func TestChipRowWrapsToTheWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	labels := []string{"Action", "Adventure", "Drama", "Fantasy", "Magic"}
	rows := ChipRow(labels, 30, theme.SecondaryContainer)

	if len(rows) < 2 {
		t.Fatalf("the chips did not wrap: %v", rows)
	}
	for i, row := range rows {
		if got := visibleWidth(row); got > 30 {
			t.Errorf("row %d is %d cells wide, want at most 30: %q", i, got, row)
		}
	}

	joined := strip(strings.Join(rows, ""))
	for _, label := range labels {
		if !strings.Contains(joined, label) {
			t.Errorf("the row lost the chip %q: %q", label, joined)
		}
	}
}

func TestArtworkRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 30
	artworks := []Artwork{
		{Title: "One Piece", Width: width, Height: 12},
		{Image: "▀▀▀▀▀\n▀▀▀▀▀", Title: "One Piece", Width: width, Height: 4},
		{Image: strings.Repeat("▀", 60), Title: "One Piece", Width: width, Height: 6},
	}

	for _, art := range artworks {
		out := art.Render()
		lines := strings.Split(strip(out), "\n")
		if len(lines) != art.Height {
			t.Errorf("the artwork is %d rows, want %d: %q", len(lines), art.Height, out)
		}
		for i, line := range lines {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("artwork line %d is %d cells, want %d: %q", i, got, width, line)
			}
		}
	}
}

func TestArtworkFallsBackWithoutAnImage(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	out := strip(Artwork{Title: "One Piece", Width: 30, Height: 10}.Render())

	if !strings.Contains(out, "One Piece") {
		t.Errorf("the placeholder lost the title: %q", out)
	}
	if !strings.Contains(out, "No cover art") {
		t.Errorf("the placeholder does not say what it is: %q", out)
	}
}

func TestListItemWithSubtitleRendersExactWidth(t *testing.T) {
	theme.SetProfile(colorprofile.TrueColor)

	const width = 40
	items := []ListItem{
		{Title: "One Piece", Subtitle: "ワンピース", Description: "TV · 1999 · 1180 eps"},
		{Title: "One Piece", Subtitle: "ワンピース", Description: "TV", Selected: true, Focused: true},
		// A row that is too narrow for the alternative title keeps the
		// headline: the headline identifies the entry.
		{Title: "One Piece", Subtitle: "ワンピース", Description: "TV"},
	}

	for i, item := range items {
		w := width
		if i == len(items)-1 {
			w = 14
		}
		out := item.Render(w)
		for j, line := range strings.Split(strip(out), "\n") {
			if got := lipgloss.Width(line); got != w {
				t.Errorf("list item line %d is %d cells, want %d: %q", j, got, w, line)
			}
		}
	}
}
