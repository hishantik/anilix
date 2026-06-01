package cmd

import (
	"fmt"

	"github.com/hishantik/anilix/version"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("anilix version", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
