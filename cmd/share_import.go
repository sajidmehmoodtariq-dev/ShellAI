package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shellai/internal/config"
	"shellai/internal/search"

	"gopkg.in/yaml.v3"
)

func runShare(args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	outFile := fs.String("file", "", "Write output to a file instead of stdout")
	format := fs.String("format", "yaml", "Output format: yaml or json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	entries, err := search.LoadEntriesFromJSON(userCommandsPath())
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No custom commands to share yet. Add one first with: shellai add")
		return nil
	}

	payload, err := marshalEntries(entries, strings.ToLower(strings.TrimSpace(*format)))
	if err != nil {
		return err
	}

	if strings.TrimSpace(*outFile) == "" {
		fmt.Print(string(payload))
		if len(payload) == 0 || payload[len(payload)-1] != '\n' {
			fmt.Println()
		}
		return nil
	}

	if err := os.WriteFile(*outFile, payload, 0o644); err != nil {
		return fmt.Errorf("write share file: %w", err)
	}
	fmt.Printf("Exported %d custom commands to %s\n", len(entries), *outFile)
	return nil
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: shellai import <file-path-or-url>")
	}

	source := fs.Arg(0)
	data, err := readImportSource(source)
	if err != nil {
		return err
	}

	incoming, validationProblems, err := parseAndValidateEntries(data)
	if err != nil {
		return err
	}
	if len(validationProblems) > 0 {
		return fmt.Errorf("import validation failed:\n- %s", strings.Join(validationProblems, "\n- "))
	}

	if len(incoming) == 0 {
		return fmt.Errorf("import file contains no command entries")
	}

	fmt.Printf("Import file contains %d command(s):\n", len(incoming))
	for _, entry := range incoming {
		fmt.Printf("- %s\n", entry.Intent)
	}
	if !askYesNo("Proceed with import? [y/N]: ") {
		fmt.Println("Import canceled.")
		return nil
	}

	existing, err := search.LoadEntriesFromJSON(userCommandsPath())
	if err != nil {
		return err
	}

	changes, overwrites, skipped := resolveImportConflicts(existing, incoming)
	if len(changes) == 0 {
		fmt.Println("No changes to apply.")
		return nil
	}

	updated := applyImportChanges(existing, changes)
	if err := saveUserEntries(updated); err != nil {
		return err
	}

	if _, err := search.NewEngineFromDatabases(config.CommandsPath(), userCommandsPath()); err != nil {
		return fmt.Errorf("import saved but re-index failed: %w", err)
	}

	fmt.Printf("Import complete. Added/updated %d command(s). Overwritten: %d. Skipped: %d.\n", len(changes), overwrites, skipped)
	return nil
}

func parseAndValidateEntries(data []byte) ([]search.CommandEntry, []string, error) {
	entries, err := decodeEntries(data)
	if err != nil {
		return nil, nil, err
	}

	problems := make([]string, 0)
	validated := make([]search.CommandEntry, 0, len(entries))
	for i, entry := range entries {
		norm := normalizeEntry(entry)
		errList := validateEntry(norm)
		if len(errList) > 0 {
			for _, p := range errList {
				problems = append(problems, fmt.Sprintf("entry %d (%q): %s", i+1, norm.Intent, p))
			}
			continue
		}
		validated = append(validated, norm)
	}

	return validated, problems, nil
}

func decodeEntries(data []byte) ([]search.CommandEntry, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	var list []search.CommandEntry
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	var single search.CommandEntry
	if err := yaml.Unmarshal(data, &single); err == nil {
		if strings.TrimSpace(single.Intent) != "" || strings.TrimSpace(single.CommandTemplate) != "" {
			return []search.CommandEntry{single}, nil
		}
	}

	return nil, fmt.Errorf("could not parse import content as YAML/JSON command entry or list")
}

func marshalEntries(entries []search.CommandEntry, format string) ([]byte, error) {
	switch format {
	case "yaml", "yml":
		data, err := yaml.Marshal(entries)
		if err != nil {
			return nil, fmt.Errorf("marshal yaml: %w", err)
		}
		return data, nil
	case "json":
		return jsonMarshal(entries)
	default:
		return nil, fmt.Errorf("unsupported format %q; use yaml or json", format)
	}
}

func readImportSource(source string) ([]byte, error) {
	if isHTTPURL(source) {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("fetch import URL: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch import URL returned status %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read import URL body: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("import file not found: %s", source)
		}
		return nil, fmt.Errorf("read import file: %w", err)
	}
	return data, nil
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u == nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func resolveImportConflicts(existing, incoming []search.CommandEntry) (changes []search.CommandEntry, overwrites, skipped int) {
	existingByIntent := make(map[string]search.CommandEntry, len(existing))
	for _, e := range existing {
		existingByIntent[strings.ToLower(strings.TrimSpace(e.Intent))] = e
	}

	changes = make([]search.CommandEntry, 0, len(incoming))
	reader := bufio.NewReader(os.Stdin)

	for _, in := range incoming {
		key := strings.ToLower(strings.TrimSpace(in.Intent))
		if _, exists := existingByIntent[key]; !exists {
			changes = append(changes, in)
			continue
		}

		for {
			fmt.Printf("Intent %q already exists in your custom commands. [s]kip/[o]verwrite: ", in.Intent)
			line, _ := reader.ReadString('\n')
			answer := strings.ToLower(strings.TrimSpace(line))
			switch answer {
			case "s", "skip":
				skipped++
				goto nextEntry
			case "o", "overwrite":
				overwrites++
				changes = append(changes, in)
				goto nextEntry
			default:
				fmt.Println("Please type s (skip) or o (overwrite).")
			}
		}

	nextEntry:
	}

	return changes, overwrites, skipped
}

func applyImportChanges(existing, changes []search.CommandEntry) []search.CommandEntry {
	updated := make([]search.CommandEntry, 0, len(existing)+len(changes))
	index := make(map[string]int)

	for _, e := range existing {
		key := strings.ToLower(strings.TrimSpace(e.Intent))
		index[key] = len(updated)
		updated = append(updated, e)
	}

	for _, c := range changes {
		key := strings.ToLower(strings.TrimSpace(c.Intent))
		if i, ok := index[key]; ok {
			updated[i] = c
			continue
		}
		index[key] = len(updated)
		updated = append(updated, c)
	}

	return updated
}

func saveUserEntries(entries []search.CommandEntry) error {
	path := userCommandsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := jsonMarshal(entries)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write user command database: %w", err)
	}
	return nil
}

func askYesNo(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
