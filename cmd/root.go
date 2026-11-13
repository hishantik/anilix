package cmd

import (
	"fmt"
	"os"

	"github.com/hishantik/anilix/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "anilix",
	Short: "Anime streaming CLI",
	Long:  `Anilix - Stream anime from your terminal`,
	Run:   runTUI,
	Args:  cobra.RangeArgs(0, 1),
}

func Execute() error {
	return rootCmd.Execute()
}

// tuiCmd represents the TUI search command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI search",
	Long:  `Launch the interactive TUI to search and select anime`,
	Run:   runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) {
	result, err := tui.RunSearch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if result == nil || result.Anime == nil {
		os.Exit(0)
	}
	fmt.Printf("Selected: %s (MAL ID: %d, Episode: %s)\n", result.Anime.Name, result.Anime.MALID, result.Episode)
}
