package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"shellai/internal/config"
	"shellai/internal/search"

	"gopkg.in/yaml.v3"
)

const validationHeaderMarker = "# SHELLAI_VALIDATION_ERRORS"

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	generatePrompt := fs.String("generate-prompt", "", "Generate an LLM prompt for creating a ShellAI YAML entry")
	fromStdin := fs.Bool("from-stdin", false, "Read YAML entry from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*generatePrompt) != "" {
		fmt.Println(buildGeneratePrompt(strings.TrimSpace(*generatePrompt)))
		return nil
	}

	if *fromStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin yaml: %w", err)
		}
		entry, problems := parseAndValidateEntry(data)
		if len(problems) > 0 {
			return fmt.Errorf("validation failed:\n- %s", strings.Join(problems, "\n- "))
		}
		path, err := saveUserEntry(entry)
		if err != nil {
			return err
		}
		fmt.Printf("Saved command entry to %s\n", path)
		return nil
	}

	return runAddInteractive()
}

func runAddInteractive() error {
	tmp, err := os.CreateTemp("", "shellai_add_*.yaml")
	if err != nil {
		return fmt.Errorf("create temp yaml: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := os.WriteFile(tmpPath, []byte(defaultYAMLTemplate()), 0o644); err != nil {
		return fmt.Errorf("write yaml template: %w", err)
	}

	for {
		if err := openEditor(tmpPath); err != nil {
			return err
		}

		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("read edited yaml: %w", err)
		}

		entry, problems := parseAndValidateEntry(data)
		if len(problems) == 0 {
			path, err := saveUserEntry(entry)
			if err != nil {
				return err
			}
			fmt.Printf("Saved command entry to %s\n", path)
			return nil
		}

		fmt.Fprintln(os.Stderr, "Validation failed. Reopening editor with details.")
		annotated := addValidationBanner(string(data), problems)
		if err := os.WriteFile(tmpPath, []byte(annotated), 0o644); err != nil {
			return fmt.Errorf("write annotated yaml: %w", err)
		}
	}
}

func parseAndValidateEntry(data []byte) (search.CommandEntry, []string) {
	var entry search.CommandEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return search.CommandEntry{}, []string{fmt.Sprintf("invalid YAML: %v", err)}
	}
	entry = normalizeEntry(entry)
	problems := validateEntry(entry)
	return entry, problems
}

func normalizeEntry(entry search.CommandEntry) search.CommandEntry {
	entry.Intent = strings.TrimSpace(entry.Intent)
	entry.CommandTemplate = strings.TrimSpace(entry.CommandTemplate)
	entry.Explanation = strings.TrimSpace(entry.Explanation)
	entry.Platform = strings.TrimSpace(entry.Platform)

	cleanKeywords := make([]string, 0, len(entry.Keywords))
	for _, k := range entry.Keywords {
		k = strings.TrimSpace(k)
		if k != "" {
			cleanKeywords = append(cleanKeywords, k)
		}
	}
	entry.Keywords = cleanKeywords

	for i := range entry.Flags {
		entry.Flags[i].Flag = strings.TrimSpace(entry.Flags[i].Flag)
		entry.Flags[i].Description = strings.TrimSpace(entry.Flags[i].Description)
	}

	return entry
}

func validateEntry(entry search.CommandEntry) []string {
	problems := make([]string, 0)
	if entry.Intent == "" {
		problems = append(problems, "intent is required")
	}
	if len(entry.Keywords) == 0 {
		problems = append(problems, "keywords must contain at least one phrase")
	}
	if entry.CommandTemplate == "" {
		problems = append(problems, "command_template is required")
	}
	if entry.Explanation == "" {
		problems = append(problems, "explanation is required")
	}
	if entry.Platform == "" {
		problems = append(problems, "platform is required")
	}
	for i, f := range entry.Flags {
		if f.Flag == "" {
			problems = append(problems, fmt.Sprintf("flags[%d].flag is required", i))
		}
		if f.Description == "" {
			problems = append(problems, fmt.Sprintf("flags[%d].description is required", i))
		}
	}
	return problems
}

func saveUserEntry(entry search.CommandEntry) (string, error) {
	path := userCommandsPath()
	entries, err := search.LoadEntriesFromJSON(path)
	if err != nil {
		return "", err
	}
	entries = upsertUserEntry(entries, entry)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	data, err := jsonMarshal(entries)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write user command database: %w", err)
	}
	return path, nil
}

func upsertUserEntry(entries []search.CommandEntry, entry search.CommandEntry) []search.CommandEntry {
	key := strings.ToLower(strings.TrimSpace(entry.Intent))
	for i := range entries {
		if strings.ToLower(strings.TrimSpace(entries[i].Intent)) == key {
			entries[i] = entry
			return entries
		}
	}
	return append(entries, entry)
}

func openEditor(filePath string) error {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "nano"
		}
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("invalid EDITOR value")
	}

	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

func userCommandsPath() string {
	return config.UserCommandsPath()
}

func defaultYAMLTemplate() string {
	return strings.TrimSpace(`
# Fill this template, save, and close the editor.
# Required fields: intent, keywords, command_template, explanation, danger, platform.

intent: "Describe what the user wants in plain English"

keywords:
  - "how users might ask this"
  - "another common variation"

command_template: "command with placeholders like {source} {destination}"

explanation: "One sentence explaining what this command does"

flags:
  - flag: "-i"
    description: "Prompt before overwrite"
  - flag: "-v"
    description: "Verbose output"

danger: false

platform: "linux"
`) + "\n"
}

func addValidationBanner(content string, problems []string) string {
	clean := removeValidationBanner(content)
	var b strings.Builder
	b.WriteString(validationHeaderMarker)
	b.WriteString("\n")
	b.WriteString("# Fix the errors below, save, and close the editor.\n")
	for _, p := range problems {
		b.WriteString("# - ")
		b.WriteString(p)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(clean)
	return b.String()
}

func removeValidationBanner(content string) string {
	if !strings.Contains(content, validationHeaderMarker) {
		return content
	}
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == validationHeaderMarker {
			start = i
			break
		}
	}
	if start == -1 {
		return content
	}

	i := start
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			break
		}
		if strings.HasPrefix(line, "#") || line == validationHeaderMarker {
			i++
			continue
		}
		break
	}
	return strings.Join(lines[i:], "\n")
}

func buildGeneratePrompt(userGoal string) string {
	return fmt.Sprintf(`You are generating one ShellAI command entry.
Return YAML only.
Do not include markdown fences.
Do not include explanations outside YAML.
Use this exact schema:
intent: string
keywords: string[]
command_template: string
explanation: string
flags:
  - flag: string
    description: string
danger: boolean
platform: string

Requirements:
- intent must be a plain English goal
- keywords must include diverse real user phrasings
- command_template should use placeholders like {source} and {destination} when needed
- explanation must be one sentence
- platform should usually be linux unless clearly different

User goal:
%s`, userGoal)
}

func jsonMarshal(entries []search.CommandEntry) ([]byte, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal user command database: %w", err)
	}
	return data, nil
}
