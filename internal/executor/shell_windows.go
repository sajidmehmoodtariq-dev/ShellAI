//go:build windows

package executor

import (
	"os"
	"path/filepath"
	"strings"
)

func shellCommandArgs(command string) (string, []string) {
	// Windows always prefers PowerShell code path for command execution.
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell != "" {
		base := strings.ToLower(filepath.Base(shell))
		if strings.Contains(base, "powershell") || strings.HasPrefix(base, "pwsh") {
			return shell, []string{"-NoLogo", "-NoProfile", "-Command", command}
		}
	}

	if pwsh, err := execLookPath("pwsh.exe"); err == nil {
		return pwsh, []string{"-NoLogo", "-NoProfile", "-Command", command}
	}
	if powershell, err := execLookPath("powershell.exe"); err == nil {
		return powershell, []string{"-NoLogo", "-NoProfile", "-Command", command}
	}

	return "powershell.exe", []string{"-NoLogo", "-NoProfile", "-Command", command}
}

func execLookPath(file string) (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, file)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}
