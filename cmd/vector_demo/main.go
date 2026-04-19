package main

import (
	"fmt"
	"log"

	"shellai/internal/parser"
	"shellai/internal/search"
)

func main() {
	engine, err := search.NewEngineFromJSON("db/commands.json")
	if err != nil {
		log.Fatalf("failed to initialize vector search engine: %v", err)
	}

	input := "find log files older than 7d in /var/log"
	intent := parser.ParseIntent(input)
	matches := engine.Search(intent, 3)

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Parsed: action=%s target=%s destination=%s filters=%v\n\n", intent.Action, intent.Target, intent.Destination, intent.Filters)
	fmt.Println("Top 3 matches:")
	for i, m := range matches {
		fmt.Printf("%d. score=%.4f | intent=%s | command=%s\n", i+1, m.Score, m.Entry.Intent, m.Entry.CommandTemplate)
	}
}
