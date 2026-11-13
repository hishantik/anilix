package theme

// State is the interaction state of a component.
//
// A terminal has no hover, no ripple and no shadow, so the state model is
// deliberately small: the states below are the ones that can be expressed
// with colour, weight, case and box drawing characters.
type State int

const (
	// StateDefault is the resting state.
	StateDefault State = iota
	// StateHover is the pointer-over state. In a keyboard-only terminal it
	// doubles as the "pointed at" affordance of a mouse-capable terminal.
	StateHover
	// StatePressed is the active (held) state.
	StatePressed
	// StateFocused is the keyboard-focused state.
	StateFocused
	// StateSelected is the currently selected row of a list.
	StateSelected
	// StateDisabled is the unavailable state.
	StateDisabled
)

// stateName gives every state a name for the showcase and the docs.
var stateName = map[State]string{
	StateDefault:  "default",
	StateHover:    "hover",
	StatePressed:  "pressed",
	StateFocused:  "focused",
	StateSelected: "selected",
	StateDisabled: "disabled",
}

// String returns the state's name.
func (s State) String() string {
	if n, ok := stateName[s]; ok {
		return n
	}
	return "unknown"
}

// AllStates returns every state in order.
func AllStates() []State {
	return []State{StateDefault, StateHover, StatePressed, StateFocused, StateSelected, StateDisabled}
}

// Enabled reports whether the state allows interaction.
func (s State) Enabled() bool {
	return s != StateDisabled
}

// Interactive reports whether the state is a transient interaction state
// (hover, pressed, focused) rather than a resting or terminal state.
func (s State) Interactive() bool {
	switch s {
	case StateHover, StatePressed, StateFocused:
		return true
	}
	return false
}
