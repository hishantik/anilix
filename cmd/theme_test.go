package cmd

import "testing"

// TestThemeCommandIsRegistered guards the design system showcase command.
//
// The command lives in its own file and registers itself in init(): if that
// registration is lost the root command silently launches the TUI instead of
// printing the showcase.
func TestThemeCommandIsRegistered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "theme" {
			return
		}
	}
	t.Fatal("the theme command is not registered on the root command")
}

// TestNoColorRequested checks the NO_COLOR convention.
func TestNoColorRequested(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !noColorRequested() {
		t.Error("NO_COLOR must be honoured")
	}
	t.Setenv("NO_COLOR", "")
	if noColorRequested() {
		t.Error("an empty NO_COLOR must not disable colour")
	}
}
