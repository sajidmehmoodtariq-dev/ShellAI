package main

import (
	"fmt"
	"os"

	"shellai/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "add":
			if err := runAdd(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "shellai add: %v\n", err)
				os.Exit(1)
			}
			return
		case "share":
			if err := runShare(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "shellai share: %v\n", err)
				os.Exit(1)
			}
			return
		case "import":
			if err := runImport(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "shellai import: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "shellai: %v\n", err)
		os.Exit(1)
	}
}
