package main

import (
	"fmt"

	"shellai/internal/config"
	"shellai/internal/feedback"
	"shellai/internal/search"
)

func runStats(top int) error {
	if top <= 0 {
		top = 10
	}

	core, err := search.LoadEntriesFromJSON(config.CommandsPath())
	if err != nil {
		return err
	}
	user, err := search.LoadEntriesFromJSON(config.UserCommandsPath())
	if err != nil {
		return err
	}
	entries := search.MergeEntriesByIntent(core, user)

	store := feedback.NewStore()
	topHits, err := store.TopHits(top)
	if err != nil {
		return err
	}
	neverMatched, err := store.NeverMatched(entries)
	if err != nil {
		return err
	}

	fmt.Printf("Feedback files:\n")
	fmt.Printf("- misses: %s\n", store.MissesPath())
	fmt.Printf("- hits:   %s\n\n", store.HitsPath())

	fmt.Printf("Total command entries: %d\n", len(entries))
	fmt.Printf("Never matched: %d\n\n", len(neverMatched))

	fmt.Printf("Top matched commands:\n")
	if len(topHits) == 0 {
		fmt.Println("- No matches recorded yet")
	} else {
		for i, item := range topHits {
			fmt.Printf("%d. %s (%d)\n", i+1, item.Intent, item.Hits)
		}
	}

	fmt.Printf("\nNever matched commands:\n")
	if len(neverMatched) == 0 {
		fmt.Println("- None")
	} else {
		limit := 20
		if limit > len(neverMatched) {
			limit = len(neverMatched)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("- %s\n", neverMatched[i])
		}
		if len(neverMatched) > limit {
			fmt.Printf("... and %d more\n", len(neverMatched)-limit)
		}
	}

	return nil
}
