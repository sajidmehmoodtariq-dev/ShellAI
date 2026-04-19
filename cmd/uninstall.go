package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"shellai/internal/config"
)

type uninstallOptions struct {
	Yes        bool
	KeepConfig bool
}

func runUninstall(opts uninstallOptions) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	cfgDir := config.ConfigDir()

	if !opts.Yes {
		fmt.Println("This will remove ShellAI from this machine.")
		fmt.Printf("- binary: %s\n", exePath)
		if opts.KeepConfig {
			fmt.Println("- config/data: preserved")
		} else {
			fmt.Printf("- config/data: %s\n", cfgDir)
		}
		fmt.Print("Continue? type yes: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "yes" {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	switch runtime.GOOS {
	case "windows":
		if err := uninstallWindows(exePath, cfgDir, opts.KeepConfig); err != nil {
			return err
		}
		fmt.Println("Uninstall scheduled. ShellAI binary will be removed in a few seconds.")
		return nil
	default:
		if err := uninstallUnix(exePath, cfgDir, opts.KeepConfig); err != nil {
			return err
		}
		fmt.Println("ShellAI removed successfully.")
		return nil
	}
}

func uninstallUnix(exePath, cfgDir string, keepConfig bool) error {
	if !keepConfig {
		if err := os.RemoveAll(cfgDir); err != nil {
			return fmt.Errorf("remove config directory: %w", err)
		}
	}

	if err := os.Remove(exePath); err != nil {
		return fmt.Errorf("remove executable %s: %w", exePath, err)
	}
	return nil
}

func uninstallWindows(exePath, cfgDir string, keepConfig bool) error {
	scriptPath := filepath.Join(os.TempDir(), "shellai-uninstall.cmd")

	cfgCmd := ""
	if !keepConfig {
		cfgCmd = fmt.Sprintf("rmdir /s /q \"%s\"\r\n", cfgDir)
	}

	script := fmt.Sprintf("@echo off\r\n"+
		"ping 127.0.0.1 -n 3 > nul\r\n"+
		"del /f /q \"%s\"\r\n"+
		"%s"+
		"del /f /q %%~f0\r\n", exePath, cfgCmd)

	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write uninstall script: %w", err)
	}

	cmd := exec.Command("cmd", "/C", "start", "", "/B", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schedule uninstall script: %w", err)
	}
	return nil
}
