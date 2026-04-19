package impact

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileImpact struct {
	Path  string
	Size  int64
	IsDir bool
}

type Report struct {
	Kind      string
	Items     []FileImpact
	TotalSize int64
	Note      string
}

func Preview(command string) Report {
	fields := splitCommand(command)
	if len(fields) == 0 {
		return Report{Note: "No impact preview available for empty command."}
	}

	switch strings.ToLower(fields[0]) {
	case "cp", "copy", "mv", "move":
		return previewCopyMove(fields)
	case "rm", "del", "remove":
		return previewDelete(fields)
	case "chmod":
		return previewChmod(fields)
	case "find":
		return previewFind(fields)
	default:
		return Report{Note: "No structured impact preview available for this command."}
	}
}

func PreviewLines(command string, limit int) []string {
	report := Preview(command)
	if limit <= 0 {
		limit = 10
	}

	lines := make([]string, 0)
	if report.Kind != "" {
		lines = append(lines, fmt.Sprintf("Type: %s", report.Kind))
	}

	if len(report.Items) > 0 {
		lines = append(lines, fmt.Sprintf("Affected entries: %d", len(report.Items)))
		if report.TotalSize > 0 {
			lines = append(lines, fmt.Sprintf("Total size: %s", humanSize(report.TotalSize)))
		}
		show := limit
		if show > len(report.Items) {
			show = len(report.Items)
		}
		for i := 0; i < show; i++ {
			item := report.Items[i]
			size := "dir"
			if !item.IsDir {
				size = humanSize(item.Size)
			}
			lines = append(lines, fmt.Sprintf("- %s (%s)", item.Path, size))
		}
		if len(report.Items) > show {
			lines = append(lines, fmt.Sprintf("... and %d more", len(report.Items)-show))
		}
	}

	if report.Note != "" {
		lines = append(lines, report.Note)
	}
	if len(lines) == 0 {
		lines = append(lines, "No impact preview available.")
	}
	return lines
}

func previewCopyMove(fields []string) Report {
	args := stripFlags(fields[1:])
	if len(args) < 2 {
		return Report{Kind: fields[0], Note: "Not enough arguments to determine source and destination."}
	}
	sources := args[:len(args)-1]
	items := collectTargets(sources)
	total := int64(0)
	for _, item := range items {
		total += item.Size
	}
	return Report{Kind: strings.ToLower(fields[0]), Items: items, TotalSize: total, Note: fmt.Sprintf("Destination: %s", args[len(args)-1])}
}

func previewDelete(fields []string) Report {
	args := stripFlags(fields[1:])
	if len(args) == 0 {
		return Report{Kind: strings.ToLower(fields[0]), Note: "No delete targets detected."}
	}
	items := collectTargets(args)
	total := int64(0)
	for _, item := range items {
		total += item.Size
	}
	return Report{Kind: "delete", Items: items, TotalSize: total}
}

func previewChmod(fields []string) Report {
	if len(fields) < 3 {
		return Report{Kind: "chmod", Note: "No chmod targets detected."}
	}
	args := stripFlags(fields[1:])
	if len(args) < 2 {
		return Report{Kind: "chmod", Note: "No chmod targets detected."}
	}
	targets := args[1:]
	items := collectTargets(targets)
	return Report{Kind: "chmod", Items: items, Note: fmt.Sprintf("Mode: %s", args[0])}
}

func previewFind(fields []string) Report {
	root := "."
	namePattern := ""
	olderThanDays := -1

	for i := 1; i < len(fields); i++ {
		tok := fields[i]
		switch tok {
		case "-name":
			if i+1 < len(fields) {
				namePattern = trimQuotes(fields[i+1])
				i++
			}
		case "-mtime":
			if i+1 < len(fields) {
				v := strings.TrimPrefix(fields[i+1], "+")
				if n, err := strconv.Atoi(v); err == nil {
					olderThanDays = n
				}
				i++
			}
		default:
			if !strings.HasPrefix(tok, "-") && root == "." {
				root = trimQuotes(tok)
			}
		}
	}

	items := make([]FileImpact, 0)
	cutoff := time.Now().Add(-time.Duration(olderThanDays) * 24 * time.Hour)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if namePattern != "" {
			ok, matchErr := filepath.Match(namePattern, info.Name())
			if matchErr != nil || !ok {
				return nil
			}
		}
		if olderThanDays >= 0 && info.ModTime().After(cutoff) {
			return nil
		}
		items = append(items, FileImpact{Path: path, Size: info.Size(), IsDir: false})
		return nil
	})

	total := int64(0)
	for _, item := range items {
		total += item.Size
	}
	note := fmt.Sprintf("Search root: %s", root)
	if namePattern != "" {
		note += fmt.Sprintf(", pattern: %s", namePattern)
	}
	if olderThanDays >= 0 {
		note += fmt.Sprintf(", older than: %d days", olderThanDays)
	}
	return Report{Kind: "find", Items: items, TotalSize: total, Note: note}
}

func collectTargets(patterns []string) []FileImpact {
	out := make([]FileImpact, 0)
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		expanded := expandPattern(pattern)
		for _, p := range expanded {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			if info.IsDir() {
				_ = filepath.Walk(p, func(path string, fi os.FileInfo, walkErr error) error {
					if walkErr != nil {
						return nil
					}
					if _, exists := seen[path]; exists {
						return nil
					}
					seen[path] = struct{}{}
					out = append(out, FileImpact{Path: path, Size: fi.Size(), IsDir: fi.IsDir()})
					return nil
				})
				continue
			}
			out = append(out, FileImpact{Path: p, Size: info.Size(), IsDir: false})
		}
	}
	return out
}

func expandPattern(pattern string) []string {
	p := trimQuotes(pattern)
	if strings.ContainsAny(p, "*?[") {
		matches, err := filepath.Glob(p)
		if err == nil && len(matches) > 0 {
			return matches
		}
	}
	return []string{p}
}

func stripFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		t := trimQuotes(a)
		if strings.HasPrefix(t, "-") {
			continue
		}
		out = append(out, t)
	}
	return out
}

func splitCommand(command string) []string {
	parts := strings.Fields(command)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, trimQuotes(p))
	}
	return out
}

func trimQuotes(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "\"")
	v = strings.Trim(v, "'")
	return v
}

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
