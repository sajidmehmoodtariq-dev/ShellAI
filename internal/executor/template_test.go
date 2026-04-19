package executor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"shellai/internal/parser"
)

func TestBuildDesktopDestination(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "copy /tmp/a.txt to desktop", Destination: "desktop"}

	result := engine.Build("cp {source} {destination}", intent)

	if result.Command != "cp /tmp/a.txt ~/Desktop" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
	if len(result.MissingPlaceholders) != 0 {
		t.Fatalf("expected no missing placeholders, got %v", result.MissingPlaceholders)
	}
}

func TestBuildDownloadsDestination(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "move /tmp/a.txt to downloads", Destination: "downloads"}

	result := engine.Build("mv {source} {destination}", intent)
	if result.Command != "mv /tmp/a.txt ~/Downloads" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestBuildHomeDestination(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "move /tmp/a.txt to home", Destination: "home"}

	result := engine.Build("mv {source} {destination}", intent)
	if result.Command != "mv /tmp/a.txt ~" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestBuildUSBWithSingleMount(t *testing.T) {
	mediaRoot := t.TempDir()
	if err := ensureDir(filepath.Join(mediaRoot, "USB1")); err != nil {
		t.Fatalf("failed to setup media root: %v", err)
	}

	engine := NewTemplateEngineWithMediaRoot(mediaRoot)
	intent := parser.ParsedIntent{Raw: "copy /tmp/a.txt to usb", Destination: "usb"}

	result := engine.Build("cp {source} {destination}", intent)
	expected := "cp /tmp/a.txt " + filepath.ToSlash(filepath.Join(mediaRoot, "USB1"))
	if result.Command != expected {
		t.Fatalf("unexpected command: got %q want %q", result.Command, expected)
	}
}

func TestBuildUSBWithMultipleMountsNeedsPrompt(t *testing.T) {
	mediaRoot := t.TempDir()
	if err := ensureDir(filepath.Join(mediaRoot, "USB_A")); err != nil {
		t.Fatalf("failed to setup media root: %v", err)
	}
	if err := ensureDir(filepath.Join(mediaRoot, "USB_B")); err != nil {
		t.Fatalf("failed to setup media root: %v", err)
	}

	engine := NewTemplateEngineWithMediaRoot(mediaRoot)
	intent := parser.ParsedIntent{Raw: "copy /tmp/a.txt to usb", Destination: "usb"}

	result := engine.Build("cp {source} {destination}", intent)
	if !contains(result.MissingPlaceholders, "destination") {
		t.Fatalf("expected destination missing, got %v", result.MissingPlaceholders)
	}
	options := result.PromptOptions["destination"]
	if len(options) != 2 {
		t.Fatalf("expected 2 destination options, got %v", options)
	}
}

func TestMissingSourceDoesNotGuess(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "copy file to desktop", Destination: "desktop"}

	result := engine.Build("cp {source} {destination}", intent)
	if !contains(result.MissingPlaceholders, "source") {
		t.Fatalf("expected source as missing, got %v", result.MissingPlaceholders)
	}
	if result.Command != "cp {source} ~/Desktop" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestPatternFromExtensionFilter(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{
		Raw: "find files with extension .jpg",
		Filters: []parser.Filter{{Type: "extension", Value: ".jpg"}},
	}

	result := engine.Build("find {path} -name \"{pattern}\"", intent)
	if result.Command != "find {path} -name \"*.jpg\"" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
	if !contains(result.MissingPlaceholders, "path") {
		t.Fatalf("expected missing path placeholder")
	}
}

func TestPatternFromAllFilesPhrase(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "find all jpg files in /tmp"}

	result := engine.Build("find {path} -name \"{pattern}\"", intent)
	if result.Command != "find /tmp -name \"*.jpg\"" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestTimeFilterFromOlderThanDays(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{
		Raw:     "find files older than 7 days in /var/log",
		Filters: []parser.Filter{{Type: "older_than", Value: "7 days"}},
	}

	result := engine.Build("find {path} {time_filter}", intent)
	if result.Command != "find /var/log -mtime +7" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestTimeFilterFromOlderThanShort(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{
		Raw:     "find files older than 10d in /var/log",
		Filters: []parser.Filter{{Type: "older_than", Value: "10d"}},
	}

	result := engine.Build("find {path} {time_filter}", intent)
	if result.Command != "find /var/log -mtime +10" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
}

func TestContainingFilterValue(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{
		Raw:     "list files containing invoice",
		Filters: []parser.Filter{{Type: "containing", Value: "invoice"}},
	}

	result := engine.Build("grep -R \"{containing}\" {path}", intent)
	if result.Command != "grep -R \"invoice\" {path}" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
	if !reflect.DeepEqual(result.MissingPlaceholders, []string{"path"}) {
		t.Fatalf("unexpected missing placeholders: %v", result.MissingPlaceholders)
	}
}

func TestUnknownPlaceholderIsMarkedMissing(t *testing.T) {
	engine := NewTemplateEngineWithMediaRoot(t.TempDir())
	intent := parser.ParsedIntent{Raw: "show processes", Target: "processes"}

	result := engine.Build("do {target} with {extra}", intent)
	if result.Command != "do processes with {extra}" {
		t.Fatalf("unexpected command: %s", result.Command)
	}
	if !reflect.DeepEqual(result.MissingPlaceholders, []string{"extra"}) {
		t.Fatalf("unexpected missing placeholders: %v", result.MissingPlaceholders)
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
