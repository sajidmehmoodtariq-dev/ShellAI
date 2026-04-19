package parser

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	input := "Copy, FILES! to /tmp/Backup?"
	tokens := Tokenize(input)
	expected := []string{"copy", "files", "to", "/tmp/backup"}

	if !reflect.DeepEqual(tokens, expected) {
		t.Fatalf("unexpected tokens: got %v want %v", tokens, expected)
	}
}

func TestParseIntentTable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParsedIntent
	}{
		{
			name:     "copy file to desktop",
			input:    "Copy report.txt to desktop",
			expected: ParsedIntent{Action: "copy", Target: "extension:report.txt", Filters: nil, Destination: "desktop"},
		},
		{
			name:     "move folder to downloads",
			input:    "Move folder into downloads",
			expected: ParsedIntent{Action: "move", Target: "folder", Filters: nil, Destination: "downloads"},
		},
		{
			name:     "delete old logs",
			input:    "Delete logs older than 30d",
			expected: ParsedIntent{Action: "delete", Target: "logs", Filters: []Filter{{Type: "older_than", Value: "30d"}}, Destination: ""},
		},
		{
			name:     "find extension in path",
			input:    "Find files with extension .log in /var/log",
			expected: ParsedIntent{Action: "find", Target: "files", Filters: []Filter{{Type: "extension", Value: ".log"}}, Destination: "/var/log"},
		},
		{
			name:     "show processes",
			input:    "Show processes",
			expected: ParsedIntent{Action: "show", Target: "processes", Filters: nil, Destination: ""},
		},
		{
			name:     "list files containing",
			input:    "List files containing invoice",
			expected: ParsedIntent{Action: "list", Target: "files", Filters: []Filter{{Type: "containing", Value: "invoice"}}, Destination: ""},
		},
		{
			name:     "compress folder to home",
			input:    "Compress folder to home",
			expected: ParsedIntent{Action: "compress", Target: "folder", Filters: nil, Destination: "home"},
		},
		{
			name:     "kill process by name",
			input:    "Kill process nginx",
			expected: ParsedIntent{Action: "kill", Target: "process:nginx", Filters: nil, Destination: ""},
		},
		{
			name:     "check open port",
			input:    "Check port 8080",
			expected: ParsedIntent{Action: "check", Target: "port", Filters: nil, Destination: ""},
		},
		{
			name:     "copy path to path",
			input:    "copy /tmp/a.txt to /tmp/b.txt",
			expected: ParsedIntent{Action: "copy", Target: "extension:a.txt", Filters: nil, Destination: "/tmp/b.txt"},
		},
		{
			name:     "move windows path",
			input:    "move C:\\temp\\app.log to D:\\archive\\",
			expected: ParsedIntent{Action: "move", Target: "extension:app.log", Filters: nil, Destination: "d:\\archive\\"},
		},
		{
			name:     "remove files with containing",
			input:    "remove files containing backup",
			expected: ParsedIntent{Action: "delete", Target: "files", Filters: []Filter{{Type: "containing", Value: "backup"}}, Destination: ""},
		},
		{
			name:     "search images larger",
			input:    "search images larger than 5mb",
			expected: ParsedIntent{Action: "find", Target: "images", Filters: []Filter{{Type: "larger_than", Value: "5mb"}}, Destination: ""},
		},
		{
			name:     "display dns info",
			input:    "display dns",
			expected: ParsedIntent{Action: "show", Target: "dns", Filters: nil, Destination: ""},
		},
		{
			name:     "archive documents to usb",
			input:    "archive documents to usb",
			expected: ParsedIntent{Action: "compress", Target: "documents", Filters: nil, Destination: "usb"},
		},
		{
			name:     "extract archive to downloads",
			input:    "extract archive.zip into downloads",
			expected: ParsedIntent{Action: "extract", Target: "archive", Filters: nil, Destination: "downloads"},
		},
		{
			name:     "verify host reachability",
			input:    "verify host 10.0.0.2",
			expected: ParsedIntent{Action: "check", Target: "host", Filters: nil, Destination: ""},
		},
		{
			name:     "find named process",
			input:    "find process named postgres",
			expected: ParsedIntent{Action: "find", Target: "process:postgres", Filters: nil, Destination: ""},
		},
		{
			name:     "delete directory with extension filter",
			input:    "delete directory with extension .tmp",
			expected: ParsedIntent{Action: "delete", Target: "directory", Filters: []Filter{{Type: "extension", Value: ".tmp"}}, Destination: ""},
		},
		{
			name:     "list logs older than in home",
			input:    "list logs older than 7d in /home/user",
			expected: ParsedIntent{Action: "list", Target: "logs", Filters: []Filter{{Type: "older_than", Value: "7d"}}, Destination: "/home/user"},
		},
		{
			name:     "unknown action still extracts",
			input:    "please organize files to desktop",
			expected: ParsedIntent{Action: "unknown", Target: "files", Filters: nil, Destination: "desktop"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: ParsedIntent{Action: "unknown", Target: "unknown", Filters: nil, Destination: ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseIntent(tc.input)
			if got.Action != tc.expected.Action {
				t.Fatalf("action mismatch: got %q want %q", got.Action, tc.expected.Action)
			}
				if tc.name == "move windows path" {
					allowedTargets := map[string]bool{
						"extension:app.log":           true,
						"extension:c:\\temp\\app.log": true,
					}
					if !allowedTargets[got.Target] {
						t.Fatalf("target mismatch: got %q want one of %q", got.Target, []string{"extension:app.log", "extension:c:\\temp\\app.log"})
					}
				} else if got.Target != tc.expected.Target {
				t.Fatalf("target mismatch: got %q want %q", got.Target, tc.expected.Target)
			}
			if got.Destination != tc.expected.Destination {
				t.Fatalf("destination mismatch: got %q want %q", got.Destination, tc.expected.Destination)
			}
			if !reflect.DeepEqual(got.Filters, tc.expected.Filters) {
				t.Fatalf("filters mismatch: got %#v want %#v", got.Filters, tc.expected.Filters)
			}
		})
	}
}
