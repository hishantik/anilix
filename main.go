package main

import (
	"fmt"
	"os"

	"github.com/hishantik/anilix/cmd"
	"github.com/hishantik/anilix/config"
)

func main() {
	if err := config.Setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
