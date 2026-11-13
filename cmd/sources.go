package cmd

import (
	"fmt"

	"github.com/hishantik/anilix/provider"
	"github.com/spf13/cobra"
)

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List available anime sources",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available sources:")
		for _, p := range provider.All() {
			fmt.Printf("  - %s\n", p.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(sourcesCmd)
}
