package parser

import (
	"regexp"
	"strings"
)

var chainDelimiterPattern = regexp.MustCompile(`(?i)\s*(?:,\s*)?(?:and then|after that|then)\s+`)

// SplitWorkflow breaks a natural-language workflow into ordered steps.
// It handles conjunctions like "then", "and then", and "after that".
func SplitWorkflow(input string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}

	parts := chainDelimiterPattern.Split(trimmed, -1)
	steps := normalizeSteps(parts)
	if len(steps) > 1 {
		return steps
	}

	commaParts := strings.Split(trimmed, ",")
	return normalizeSteps(commaParts)
}

func normalizeSteps(parts []string) []string {
	steps := make([]string, 0, len(parts))
	for _, part := range parts {
		step := strings.TrimSpace(part)
		if step == "" {
			continue
		}
		steps = append(steps, step)
	}
	return steps
}
