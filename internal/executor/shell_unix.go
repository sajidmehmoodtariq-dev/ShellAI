//go:build !windows

package executor

import (
	"os"
	"path/filepath"
	"strings"
)

func shellCommandArgs(command string) (string, []string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	base := strings.ToLower(filepath.Base(shell))
	if base == "bash" || base == "zsh" || base == "fish" {
		return shell, []string{"-ic", command}
	}

	return shell, []string{"-c", command}
}
