package tui

import "charm.land/bubbles/v2/key"

type keymap struct {
	Up         key.Binding
	Down       key.Binding
	Select     key.Binding
	Back       key.Binding
	Quit       key.Binding
	Toggle     key.Binding
	Search     key.Binding
	Settings   key.Binding
	Open       key.Binding
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
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in Miruro"),
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
	return []key.Binding{k.Up, k.Down, k.Select, k.Settings, k.Back, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select},
		{k.Back, k.Quit, k.Toggle, k.Settings, k.Open},
	}
}

type confirmKeymap struct {
	Yes key.Binding
	No  key.Binding
}

func (k confirmKeymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Yes, k.No}
}

func (k confirmKeymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Yes, k.No}}
}

type settingsKeymap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Enter key.Binding
	Close key.Binding
}

func (k settingsKeymap) ShortHelp() []key.Binding {
	bindings := []key.Binding{k.Up, k.Down}
	if k.Left.Keys() != nil {
		bindings = append(bindings, k.Left)
	}
	if k.Right.Keys() != nil {
		bindings = append(bindings, k.Right)
	}
	if k.Enter.Keys() != nil {
		bindings = append(bindings, k.Enter)
	}
	bindings = append(bindings, k.Close)
	return bindings
}

func (k settingsKeymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
