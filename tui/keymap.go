package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// styledHelpStyles returns the canonical help styles used by every help bar
// in the TUI (including the help bar embedded in list.Model). Keeping them in
// one place ensures the home helpbar matches the search/detail helpbar.
func styledHelpStyles() help.Styles {
	hs := help.DefaultStyles(true)
	hs.ShortKey = lipgloss.NewStyle().Foreground(Theme.Primary)
	hs.ShortDesc = lipgloss.NewStyle().Foreground(Theme.Faint)
	hs.FullKey = lipgloss.NewStyle().Foreground(Theme.Primary)
	hs.FullDesc = lipgloss.NewStyle().Foreground(Theme.Faint)
	hs.ShortSeparator = lipgloss.NewStyle().Foreground(Theme.Faint)
	hs.FullSeparator = lipgloss.NewStyle().Foreground(Theme.Faint)
	return hs
}

type keymap struct {
	Up         key.Binding
	Down       key.Binding
	Select     key.Binding
	Back       key.Binding
	Quit       key.Binding
	Toggle     key.Binding
	Search     key.Binding
	Settings   key.Binding
	Resume     key.Binding
	NextRegion key.Binding
	PrevRegion key.Binding
	ConfirmYes key.Binding
	ConfirmNo  key.Binding
}

func newKeymap() keymap {
	return keymap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "search/select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "toggle sub/dub"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Settings: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "settings"),
		),
		Resume: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "resume watching"),
		),
		NextRegion: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next section"),
		),
		PrevRegion: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev section"),
		),
		ConfirmYes: key.NewBinding(
			key.WithKeys("y", "enter"),
			key.WithHelp("y/enter", "yes"),
		),
		ConfirmNo: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "no"),
		),
	}
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Settings, k.Resume, k.Back, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Search},
		{k.Back, k.Quit, k.Toggle, k.Settings, k.Resume},
		{k.NextRegion, k.PrevRegion},
	}
}
