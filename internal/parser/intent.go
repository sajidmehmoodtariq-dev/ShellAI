package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

type Filter struct {
	Type  string
	Value string
}

type ParsedIntent struct {
	Raw         string
	Action      string
	Target      string
	Filters     []Filter
	Destination string
}

var punctuationPattern = regexp.MustCompile(`[^a-z0-9._/\\:\-\s]+`)
var multiSpacePattern = regexp.MustCompile(`\s+`)
var extensionPattern = regexp.MustCompile(`\.([a-z0-9]+)$`)

var actionMap = map[string]string{
	"copy":      "copy",
	"duplicate": "copy",
	"move":      "move",
	"relocate":  "move",
	"delete":    "delete",
	"remove":    "delete",
	"erase":     "delete",
	"find":      "find",
	"search":    "find",
	"show":      "show",
	"display":   "show",
	"list":      "list",
	"compress":  "compress",
	"archive":   "compress",
	"zip":       "compress",
	"extract":   "extract",
	"unzip":     "extract",
	"kill":      "kill",
	"stop":      "kill",
	"check":     "check",
	"verify":    "check",
}

var fileTypeTerms = map[string]struct{}{
	"file": {}, "files": {}, "folder": {}, "folders": {}, "directory": {}, "directories": {},
	"log": {}, "logs": {}, "image": {}, "images": {}, "video": {}, "videos": {},
	"archive": {}, "archives": {}, "document": {}, "documents": {},
}

var networkTerms = map[string]struct{}{
	"port": {}, "ports": {}, "host": {}, "ip": {}, "dns": {}, "network": {}, "url": {},
}

var processTerms = map[string]struct{}{
	"process": {}, "processes": {}, "pid": {}, "service": {}, "services": {},
}

var destinationKeywords = map[string]struct{}{
	"usb": {}, "desktop": {}, "home": {}, "downloads": {},
}

func ParseIntent(input string) ParsedIntent {
	tokens := Tokenize(input)
	rawLower := strings.ToLower(strings.TrimSpace(input))

	if isCurrentDirectoryQuery(rawLower) {
		return ParsedIntent{
			Raw:         input,
			Action:      "show",
			Target:      "directory",
			Filters:     nil,
			Destination: "",
		}
	}

	return ParsedIntent{
		Raw:         input,
		Action:      ExtractAction(tokens),
		Target:      ExtractTarget(tokens),
		Filters:     ExtractFilters(tokens),
		Destination: ExtractDestination(tokens),
	}
}

func Tokenize(input string) []string {
	clean := strings.ToLower(input)
	clean = punctuationPattern.ReplaceAllString(clean, " ")
	clean = multiSpacePattern.ReplaceAllString(strings.TrimSpace(clean), " ")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, " ")
}

func ExtractAction(tokens []string) string {
	for _, token := range tokens {
		if action, ok := actionMap[token]; ok {
			return action
		}
	}
	return "unknown"
}

func ExtractTarget(tokens []string) string {
	for i, token := range tokens {
		if _, isAction := actionMap[token]; isAction {
			continue
		}

		if _, ok := fileTypeTerms[token]; ok {
			return token
		}
		if _, ok := networkTerms[token]; ok {
			return token
		}
		if _, ok := processTerms[token]; ok {
			if token == "process" || token == "processes" {
				if i+2 < len(tokens) && tokens[i+1] == "named" {
					return "process:" + tokens[i+2]
				}
				if i+1 < len(tokens) {
					next := tokens[i+1]
					if next != "on" && next != "in" && next != "with" {
						return "process:" + next
					}
				}
			}
			return token
		}

		if extensionPattern.MatchString(token) {
			if strings.Contains(token, "/") || strings.Contains(token, "\\") {
				base := filepath.Base(token)
				ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(base)), ".")
				if isArchiveExtension(ext) {
					return "archive"
				}
				if base != "" {
					return "extension:" + strings.ToLower(base)
				}
			}

			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(token)), ".")
			if isArchiveExtension(ext) {
				return "archive"
			}
			return "extension:" + token
		}
	}

	for i, token := range tokens {
		if token == "named" && i+1 < len(tokens) {
			return tokens[i+1]
		}
	}

	return "unknown"
}

func ExtractFilters(tokens []string) []Filter {
	var filters []Filter

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if token == "older" && i+2 < len(tokens) && tokens[i+1] == "than" {
			filters = append(filters, Filter{Type: "older_than", Value: tokens[i+2]})
		}

		if token == "larger" && i+2 < len(tokens) && tokens[i+1] == "than" {
			filters = append(filters, Filter{Type: "larger_than", Value: tokens[i+2]})
		}

		if token == "with" && i+2 < len(tokens) && tokens[i+1] == "extension" {
			filters = append(filters, Filter{Type: "extension", Value: tokens[i+2]})
		}

		if token == "containing" && i+1 < len(tokens) {
			filters = append(filters, Filter{Type: "containing", Value: tokens[i+1]})
		}
	}

	return filters
}

func ExtractDestination(tokens []string) string {
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if (token == "to" || token == "into") && i+1 < len(tokens) {
			next := tokens[i+1]
			if isPathLike(next) || isDestinationKeyword(next) {
				return next
			}
		}
	}

	for _, token := range tokens {
		if isDestinationKeyword(token) {
			return token
		}
	}

	for i := len(tokens) - 1; i >= 0; i-- {
		token := tokens[i]
		if isPathLike(token) {
			return token
		}
	}

	return ""
}

func isPathLike(token string) bool {
	return strings.Contains(token, "/") || strings.Contains(token, "\\") || strings.Contains(token, ":")
}

func isDestinationKeyword(token string) bool {
	_, ok := destinationKeywords[token]
	return ok
}

func isArchiveExtension(ext string) bool {
	switch ext {
	case "zip", "tar", "gz", "tgz", "bz2", "xz", "rar", "7z":
		return true
	default:
		return false
	}
}

func isCurrentDirectoryQuery(raw string) bool {
	if raw == "" {
		return false
	}

	patterns := []string{
		"where am i",
		"where i am",
		"current directory",
		"working directory",
		"what is my directory",
		"what directory am i in",
		"show current directory",
		"show working directory",
		"pwd",
	}

	for _, p := range patterns {
		if strings.Contains(raw, p) {
			return true
		}
	}

	return false
}
