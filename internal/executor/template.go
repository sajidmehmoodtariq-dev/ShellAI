package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"shellai/internal/parser"
)

type BuildResult struct {
	Command             string
	MissingPlaceholders []string
	PromptOptions       map[string][]string
}

type TemplateEngine struct {
	mediaRoot string
}

var placeholderPattern = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
var pathTokenPattern = regexp.MustCompile(`([a-zA-Z]:\\[^\s]+|/[^\s]+)`)
var allFilesExtensionPattern = regexp.MustCompile(`\ball\s+([a-z0-9]+)\s+files\b`)

func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{mediaRoot: "/media"}
}

func NewTemplateEngineWithMediaRoot(mediaRoot string) *TemplateEngine {
	return &TemplateEngine{mediaRoot: mediaRoot}
}

func (e *TemplateEngine) Build(template string, intent parser.ParsedIntent) BuildResult {
	placeholders := extractPlaceholders(template)
	if len(placeholders) == 0 {
		return BuildResult{Command: template}
	}

	values, promptOptions := e.resolveValues(intent)
	command := template
	missing := make([]string, 0)

	for _, name := range placeholders {
		value, ok := values[name]
		if !ok || strings.TrimSpace(value) == "" {
			missing = appendMissing(missing, name)
			continue
		}
		command = strings.ReplaceAll(command, "{"+name+"}", value)
	}

	return BuildResult{
		Command:             command,
		MissingPlaceholders: missing,
		PromptOptions:       promptOptions,
	}
}

func (e *TemplateEngine) resolveValues(intent parser.ParsedIntent) (map[string]string, map[string][]string) {
	values := map[string]string{}
	promptOptions := map[string][]string{}

	paths := extractPaths(intent.Raw)

	if source := resolveSource(intent, paths); source != "" {
		values["source"] = source
	}

	destination, options := e.resolveDestination(intent, paths)
	if destination != "" {
		values["destination"] = destination
	}
	if len(options) > 0 {
		promptOptions["destination"] = options
	}

	if target := normalizeTarget(intent.Target); target != "" {
		values["target"] = target
	}

	if path := resolvePath(intent, paths); path != "" {
		values["path"] = path
	}

	if pattern := resolvePattern(intent); pattern != "" {
		values["pattern"] = pattern
	}

	if timeFilter := resolveTimeFilter(intent); timeFilter != "" {
		values["time_filter"] = timeFilter
	}

	if ext := resolveExtension(intent); ext != "" {
		values["extension"] = ext
	}

	if containing := resolveContaining(intent); containing != "" {
		values["containing"] = containing
	}

	return values, promptOptions
}

func resolveSource(intent parser.ParsedIntent, paths []string) string {
	from := findTokenAfter(intent.Raw, "from")
	to := findTokenAfter(intent.Raw, "to")
	if from != "" && to != "" {
		return from
	}
	if len(paths) == 1 {
		return paths[0]
	}
	if len(paths) >= 2 {
		return paths[0]
	}
	return ""
}

func (e *TemplateEngine) resolveDestination(intent parser.ParsedIntent, paths []string) (string, []string) {
	dst := strings.TrimSpace(intent.Destination)
	if dst == "" && len(paths) >= 2 {
		dst = paths[len(paths)-1]
	}
	if dst == "" {
		return "", nil
	}

	expanded, options := e.expandDestination(dst)
	if len(options) > 0 && expanded == "" {
		return "", options
	}
	return expanded, options
}

func (e *TemplateEngine) expandDestination(token string) (string, []string) {
	lower := strings.ToLower(strings.TrimSpace(token))
	switch lower {
	case "desktop":
		return "~/Desktop", nil
	case "downloads":
		return "~/Downloads", nil
	case "home":
		return "~", nil
	case "usb":
		mounts := scanUSBDestinations(e.mediaRoot)
		if len(mounts) == 1 {
			return mounts[0], mounts
		}
		if len(mounts) > 1 {
			return "", mounts
		}
		return "", nil
	default:
		return token, nil
	}
}

func scanUSBDestinations(mediaRoot string) []string {
	entries, err := os.ReadDir(mediaRoot)
	if err != nil {
		return nil
	}

	mounts := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			mounts = append(mounts, filepath.ToSlash(filepath.Join(mediaRoot, entry.Name())))
		}
	}
	sort.Strings(mounts)
	return mounts
}

func resolvePath(intent parser.ParsedIntent, paths []string) string {
	if len(paths) > 0 {
		return paths[0]
	}
	dst := strings.TrimSpace(intent.Destination)
	if dst != "" {
		return dst
	}
	return ""
}

func resolvePattern(intent parser.ParsedIntent) string {
	if ext := resolveExtension(intent); ext != "" {
		return "*." + ext
	}

	raw := strings.ToLower(intent.Raw)
	if m := allFilesExtensionPattern.FindStringSubmatch(raw); len(m) == 2 {
		return "*." + strings.TrimPrefix(m[1], ".")
	}

	for _, filter := range intent.Filters {
		if filter.Type == "containing" && filter.Value != "" {
			return "*" + filter.Value + "*"
		}
	}

	return ""
}

func resolveTimeFilter(intent parser.ParsedIntent) string {
	for _, filter := range intent.Filters {
		if filter.Type != "older_than" {
			continue
		}
		days := extractLeadingInt(filter.Value)
		if days > 0 {
			return fmt.Sprintf("-mtime +%d", days)
		}
	}
	return ""
}

func resolveExtension(intent parser.ParsedIntent) string {
	for _, filter := range intent.Filters {
		if filter.Type == "extension" && filter.Value != "" {
			return strings.TrimPrefix(strings.ToLower(filter.Value), ".")
		}
	}

	target := strings.ToLower(intent.Target)
	if strings.HasPrefix(target, "extension:") {
		file := strings.TrimPrefix(target, "extension:")
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file)), ".")
		if ext != "" {
			return ext
		}
	}

	raw := strings.ToLower(intent.Raw)
	if m := allFilesExtensionPattern.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimPrefix(m[1], ".")
	}

	return ""
}

func resolveContaining(intent parser.ParsedIntent) string {
	for _, filter := range intent.Filters {
		if filter.Type == "containing" {
			return filter.Value
		}
	}
	return ""
}

func normalizeTarget(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "extension:")
	target = strings.TrimPrefix(target, "process:")
	if target == "unknown" {
		return ""
	}
	return target
}

func extractPlaceholders(template string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(template, -1)
	seen := map[string]struct{}{}
	placeholders := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) != 2 {
			continue
		}
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		placeholders = append(placeholders, name)
	}
	return placeholders
}

func appendMissing(missing []string, name string) []string {
	for _, item := range missing {
		if item == name {
			return missing
		}
	}
	return append(missing, name)
}

func extractPaths(raw string) []string {
	matches := pathTokenPattern.FindAllString(raw, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, filepath.ToSlash(m))
	}
	return result
}

func findTokenAfter(raw, keyword string) string {
	parts := strings.Fields(raw)
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], keyword) {
			return filepath.ToSlash(parts[i+1])
		}
	}
	return ""
}

func extractLeadingInt(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	digits := make([]rune, 0)
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
			continue
		}
		if len(digits) > 0 {
			break
		}
	}

	if len(digits) == 0 {
		return 0
	}

	n, err := strconv.Atoi(string(digits))
	if err != nil {
		return 0
	}
	return n
}
