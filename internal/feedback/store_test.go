package feedback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shellai/internal/search"
)

func TestRecordMissWritesLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLAI_CONFIG_DIR", dir)

	store := NewStore()
	if err := store.RecordMiss("copy files to usb", "cp -r src /media/usb"); err != nil {
		t.Fatalf("RecordMiss failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "misses.log"))
	if err != nil {
		t.Fatalf("read misses.log failed: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "copy files to usb") {
		t.Fatalf("misses.log missing query: %s", text)
	}
	if !strings.Contains(text, "cp -r src /media/usb") {
		t.Fatalf("misses.log missing returned command: %s", text)
	}
}

func TestNeverMatchedUsesRecordedHits(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLAI_CONFIG_DIR", dir)

	store := NewStore()
	matches := []search.ScoredMatch{
		{Entry: search.CommandEntry{Intent: "Copy files"}, Score: 0.9},
		{Entry: search.CommandEntry{Intent: "List files"}, Score: 0.8},
	}
	if err := store.RecordMatches(matches); err != nil {
		t.Fatalf("RecordMatches failed: %v", err)
	}

	entries := []search.CommandEntry{
		{Intent: "Copy files"},
		{Intent: "List files"},
		{Intent: "Delete files"},
	}
	never, err := store.NeverMatched(entries)
	if err != nil {
		t.Fatalf("NeverMatched failed: %v", err)
	}
	if len(never) != 1 || never[0] != "Delete files" {
		t.Fatalf("unexpected never matched entries: %v", never)
	}
}
