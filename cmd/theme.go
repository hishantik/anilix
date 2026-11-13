package cmd

import (
	"fmt"
	"os"

	"github.com/hishantik/anilix/tui"
	"github.com/hishantik/anilix/tui/theme"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Print the design system showcase",
	Long: "Print Anilix's design system: colour tokens, typography scale, spacing rules " +
		"and the component styles the interface is built from.",
	Run: func(cmd *cobra.Command, args []string) {
		// The showcase is usually piped or redirected, where colour
		// detection falls back to "no colour". Keep the palette visible
		// unless the user explicitly asked for plain output.
		if !theme.HasColor() && !noColorRequested() {
			theme.ForceTrueColor()
		}
		fmt.Println(tui.Showcase())
	},
}

func init() {
	rootCmd.AddCommand(themeCmd)
}

// noColorRequested reports whether the user asked for colourless output.
func noColorRequested() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return false
}
