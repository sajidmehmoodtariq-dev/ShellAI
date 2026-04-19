package impact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewDeleteWithGlob(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.log")
	f2 := filepath.Join(dir, "b.log")
	if err := os.WriteFile(f1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("bbbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Preview("rm " + filepath.Join(dir, "*.log"))
	if report.Kind != "delete" {
		t.Fatalf("expected delete kind, got %q", report.Kind)
	}
	if len(report.Items) < 2 {
		t.Fatalf("expected at least 2 impacted files, got %d", len(report.Items))
	}
}

func TestPreviewFindOlderThan(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "old.log")
	if err := os.WriteFile(f1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Preview("find " + dir + " -name \"*.log\" -mtime +0")
	if report.Kind != "find" {
		t.Fatalf("expected find kind, got %q", report.Kind)
	}
	if !strings.Contains(report.Note, "pattern") {
		t.Fatalf("expected note to include pattern details, got %q", report.Note)
	}
}

func TestPreviewLinesFallback(t *testing.T) {
	lines := PreviewLines("echo hello", 5)
	if len(lines) == 0 {
		t.Fatalf("expected preview lines")
	}
}
