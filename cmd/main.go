package main

import (
	"fmt"
	"os"

	"shellai/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "shellai: %v\n", err)
		os.Exit(1)
	}
}
