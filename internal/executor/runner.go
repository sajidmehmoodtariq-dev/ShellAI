package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type StreamEvent struct {
	Data string
}

type RunResult struct {
	Command  string
	Shell    string
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context, command string, onStdout func(StreamEvent), onStderr func(StreamEvent)) (RunResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return RunResult{}, fmt.Errorf("command is empty")
	}

	shell, args := shellCommandArgs(command)
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = os.Environ()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{}, fmt.Errorf("create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return RunResult{}, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return RunResult{}, fmt.Errorf("start command: %w", err)
	}

	var stdoutBuilder strings.Builder
	var stderrBuilder strings.Builder

	var wg sync.WaitGroup
	var readErrMu sync.Mutex
	var readErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := streamPipe(stdoutPipe, &stdoutBuilder, onStdout); err != nil {
			readErrMu.Lock()
			if readErr == nil {
				readErr = err
			}
			readErrMu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if err := streamPipe(stderrPipe, &stderrBuilder, onStderr); err != nil {
			readErrMu.Lock()
			if readErr == nil {
				readErr = err
			}
			readErrMu.Unlock()
		}
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	if readErr != nil {
		return RunResult{}, fmt.Errorf("stream command output: %w", readErr)
	}

	result := RunResult{
		Command:  command,
		Shell:    shell,
		Stdout:   stdoutBuilder.String(),
		Stderr:   stderrBuilder.String(),
		ExitCode: exitCodeFromError(waitErr),
	}

	if waitErr != nil {
		return result, fmt.Errorf("command failed with exit code %d: %w", result.ExitCode, waitErr)
	}

	return result, nil
}

func streamPipe(pipe io.Reader, builder *strings.Builder, callback func(StreamEvent)) error {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		builder.WriteString(line)
		builder.WriteString("\n")
		if callback != nil {
			callback(StreamEvent{Data: line})
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
