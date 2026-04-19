package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerCapturesStdoutAndStderr(t *testing.T) {
	runner := NewRunner()
	tmp := t.TempDir()
	fileName := "sample.txt"
	filePath := filepath.Join(tmp, fileName)
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	command := buildListCommand(tmp)

	var streamedStdout []string
	var streamedStderr []string
	result, err := runner.Run(
		context.Background(),
		command,
		func(ev StreamEvent) { streamedStdout = append(streamedStdout, ev.Data) },
		func(ev StreamEvent) { streamedStderr = append(streamedStderr, ev.Data) },
	)
	if err != nil {
		t.Fatalf("expected command to succeed, got error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(strings.ToLower(result.Stdout), strings.ToLower(fileName)) {
		t.Fatalf("expected stdout to contain %q, got: %q", fileName, result.Stdout)
	}

	if strings.TrimSpace(result.Stderr) != "" {
		t.Fatalf("expected empty stderr, got: %q", result.Stderr)
	}

	if len(streamedStdout) == 0 {
		t.Fatalf("expected streamed stdout events")
	}

	if len(streamedStderr) != 0 {
		t.Fatalf("expected no streamed stderr events, got: %v", streamedStderr)
	}
}

func TestRunnerReportsExitCodeOnFailure(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background(), buildFailCommand(), nil, nil)
	if err == nil {
		t.Fatalf("expected error for failing command")
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
}

func buildListCommand(path string) string {
	cleanPath := filepath.ToSlash(path)
	if strings.Contains(strings.ToLower(os.Getenv("SHELL")), "cmd") {
		return "dir " + cleanPath
	}
	return "ls " + cleanPath
}

func buildFailCommand() string {
	if strings.Contains(strings.ToLower(os.Getenv("SHELL")), "cmd") {
		return "exit /b 2"
	}
	return "false"
}
