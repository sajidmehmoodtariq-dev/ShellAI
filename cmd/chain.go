package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"shellai/internal/config"
	"shellai/internal/executor"
	"shellai/internal/impact"
	"shellai/internal/parser"
	"shellai/internal/search"
)

func runChain(workflow string) error {
	steps := parser.SplitWorkflow(workflow)
	if len(steps) == 0 {
		return fmt.Errorf("workflow is empty")
	}

	engine, err := search.NewEngineFromDatabases(config.CommandsPath(), config.UserCommandsPath())
	if err != nil {
		return err
	}

	tmpl := executor.NewTemplateEngine()
	runner := executor.NewRunner()
	contextValues := map[string]string{}

	fmt.Printf("Workflow has %d step(s).\n", len(steps))
	for i, stepText := range steps {
		stepNum := i + 1
		intent := parser.ParseIntent(stepText)
		matches := engine.Search(intent, 3)
		if len(matches) == 0 {
			return fmt.Errorf("step %d: no command match for %q", stepNum, stepText)
		}

		selected := matches[0]
		build := tmpl.Build(selected.Entry.CommandTemplate, intent)
		finalCmd, missing := resolveStepCommand(build.Command, build.MissingPlaceholders, contextValues)
		if len(missing) > 0 {
			resolved, err := promptMissingPlaceholders(stepNum, finalCmd, missing)
			if err != nil {
				return err
			}
			finalCmd = resolved
		}

		fmt.Println("")
		fmt.Printf("Step %d/%d\n", stepNum, len(steps))
		fmt.Printf("Request: %s\n", stepText)
		fmt.Printf("Match:   %s\n", selected.Entry.Intent)
		fmt.Printf("Cmd:     %s\n", finalCmd)
		fmt.Println("Impact preview:")
		for _, line := range impact.PreviewLines(finalCmd, 8) {
			fmt.Printf("  %s\n", line)
		}

		if cfg.DryRun {
			fmt.Println("Dry-run enabled; skipping execution for this step.")
			continue
		}

		if !confirmStep(stepNum) {
			return fmt.Errorf("workflow stopped by user at step %d", stepNum)
		}

		res, runErr := runner.Run(
			context.Background(),
			finalCmd,
			func(ev executor.StreamEvent) { fmt.Println(ev.Data) },
			func(ev executor.StreamEvent) { fmt.Fprintln(os.Stderr, ev.Data) },
		)
		if runErr != nil {
			return fmt.Errorf("step %d failed: %w; workflow stopped", stepNum, runErr)
		}

		contextValues[fmt.Sprintf("step%d_stdout", stepNum)] = strings.TrimSpace(res.Stdout)
		contextValues["previous_output"] = strings.TrimSpace(res.Stdout)
	}

	fmt.Println("Workflow complete.")
	return nil
}

func confirmStep(step int) bool {
	fmt.Printf("Run step %d? [y/N]: ", step)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func resolveStepCommand(command string, missing []string, contextValues map[string]string) (string, []string) {
	resolved := command
	stillMissing := make([]string, 0, len(missing))
	for _, key := range missing {
		value := strings.TrimSpace(contextValues[key])
		if value == "" && (key == "source" || key == "input" || key == "files") {
			value = strings.TrimSpace(contextValues["previous_output"])
		}
		if value == "" {
			stillMissing = append(stillMissing, key)
			continue
		}
		resolved = strings.ReplaceAll(resolved, "{"+key+"}", value)
	}
	return resolved, stillMissing
}

func promptMissingPlaceholders(step int, command string, missing []string) (string, error) {
	resolved := command
	reader := bufio.NewReader(os.Stdin)
	for _, key := range missing {
		fmt.Printf("Step %d needs {%s}: ", step, key)
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read placeholder value for %s: %w", key, err)
		}
		value := strings.TrimSpace(line)
		if value == "" {
			return "", fmt.Errorf("value for {%s} cannot be empty", key)
		}
		resolved = strings.ReplaceAll(resolved, "{"+key+"}", value)
	}
	return resolved, nil
}
