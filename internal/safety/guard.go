package safety

import (
	"regexp"
	"sort"
	"strings"
)

type SafetyLevel string

const (
	LevelSafe      SafetyLevel = "safe"
	LevelWarning   SafetyLevel = "warning"
	LevelDangerous SafetyLevel = "dangerous"
)

type ConfirmationMode string

const (
	ConfirmSimple    ConfirmationMode = "simple_confirm"
	ConfirmExplicit  ConfirmationMode = "explicit_confirm"
	ConfirmTypedYes  ConfirmationMode = "typed_yes"
)

type Assessment struct {
	Level           SafetyLevel
	Confirmation    ConfirmationMode
	Reasons         []string
	AffectedPaths   []string
	WhatCouldGoWrong []string
}

var pathPattern = regexp.MustCompile(`/[a-zA-Z0-9._/\-]+`)

var systemPathPrefixes = []string{
	"/etc", "/usr", "/bin", "/sbin", "/lib", "/lib64", "/boot", "/var",
}

func Analyze(command string) Assessment {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return Assessment{
			Level:        LevelWarning,
			Confirmation: ConfirmExplicit,
			Reasons:      []string{"Empty command cannot be safely evaluated."},
		}
	}

	lower := strings.ToLower(cmd)
	reasons := make([]string, 0)
	impacts := make([]string, 0)
	level := LevelSafe

	affected := extractPaths(cmd)
	hasSystemPath := false
	for _, p := range affected {
		if isSystemPath(p) {
			hasSystemPath = true
			break
		}
	}

	if containsDownloadPipeExec(lower) {
		level = escalate(level, LevelDangerous)
		reasons = append(reasons, "Command executes remote script directly from download stream.")
		impacts = append(impacts, "Remote code may run with full user privileges.")
	}

	if containsRM(lower) {
		reasons = append(reasons, "Command includes rm which can permanently delete files.")
		impacts = append(impacts, "Files may be permanently removed and not recoverable.")
		if containsRMRecursiveForce(lower) {
			level = escalate(level, LevelDangerous)
			reasons = append(reasons, "Command includes rm -rf style recursive force deletion.")
			impacts = append(impacts, "Entire directory trees can be erased instantly.")
		} else {
			level = escalate(level, LevelWarning)
		}
	}

	if hasSystemPath && containsWriteOperation(lower) {
		level = escalate(level, LevelDangerous)
		reasons = append(reasons, "Command writes to protected system directories.")
		impacts = append(impacts, "System configuration or binaries may be overwritten.")
	}

	if hasSystemPath && containsOwnershipOrPermissionChange(lower) {
		level = escalate(level, LevelDangerous)
		reasons = append(reasons, "Command changes permissions or ownership on system paths.")
		impacts = append(impacts, "System files can become inaccessible or insecure.")
	}

	if hasSystemPath && containsDeleteOperation(lower) {
		level = escalate(level, LevelDangerous)
		reasons = append(reasons, "Command deletes content under system directories.")
		impacts = append(impacts, "Critical OS files may be removed, causing boot or service failures.")
	}

	if len(reasons) == 0 {
		return Assessment{
			Level:           LevelSafe,
			Confirmation:    ConfirmSimple,
			Reasons:         []string{"No high-risk pattern detected by safety guard."},
			AffectedPaths:   affected,
			WhatCouldGoWrong: nil,
		}
	}

	reasons = uniqueStrings(reasons)
	impacts = uniqueStrings(impacts)

	return Assessment{
		Level:           level,
		Confirmation:    confirmationFor(level),
		Reasons:         reasons,
		AffectedPaths:   affected,
		WhatCouldGoWrong: impacts,
	}
}

func confirmationFor(level SafetyLevel) ConfirmationMode {
	switch level {
	case LevelDangerous:
		return ConfirmTypedYes
	case LevelWarning:
		return ConfirmExplicit
	default:
		return ConfirmSimple
	}
}

func containsDownloadPipeExec(command string) bool {
	hasDownloader := strings.Contains(command, "curl ") || strings.Contains(command, "wget ")
	hasPipeExec := strings.Contains(command, "| sh") || strings.Contains(command, "|bash") || strings.Contains(command, "| bash") || strings.Contains(command, "| /bin/sh") || strings.Contains(command, "| /bin/bash")
	return hasDownloader && hasPipeExec
}

func containsRM(command string) bool {
	return strings.Contains(command, "rm ") || strings.HasPrefix(command, "rm")
}

func containsRMRecursiveForce(command string) bool {
	patterns := []string{"rm -rf", "rm -fr", "rm -r -f", "rm -f -r", "rm -rfv", "rm -r"}
	for _, p := range patterns {
		if strings.Contains(command, p) {
			if p == "rm -r" {
				return false
			}
			return true
		}
	}
	return false
}

func containsWriteOperation(command string) bool {
	return strings.Contains(command, ">") ||
		strings.Contains(command, "tee ") ||
		strings.Contains(command, "cp ") ||
		strings.Contains(command, "mv ") ||
		strings.Contains(command, "install ") ||
		strings.Contains(command, "echo ")
}

func containsDeleteOperation(command string) bool {
	return containsRM(command) || strings.Contains(command, "rmdir ") || strings.Contains(command, "unlink ")
}

func containsOwnershipOrPermissionChange(command string) bool {
	return strings.Contains(command, "chmod ") || strings.Contains(command, "chown ") || strings.Contains(command, "chgrp ")
}

func extractPaths(command string) []string {
	matches := pathPattern.FindAllString(command, -1)
	if len(matches) == 0 {
		return nil
	}
	return uniqueStrings(matches)
}

func isSystemPath(path string) bool {
	normalized := strings.TrimSpace(path)
	for _, prefix := range systemPathPrefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
}

func escalate(current, next SafetyLevel) SafetyLevel {
	if severity(next) > severity(current) {
		return next
	}
	return current
}

func severity(level SafetyLevel) int {
	switch level {
	case LevelDangerous:
		return 3
	case LevelWarning:
		return 2
	default:
		return 1
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
