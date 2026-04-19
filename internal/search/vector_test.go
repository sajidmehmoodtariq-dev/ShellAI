package search

import (
	"fmt"
	"testing"

	"shellai/internal/parser"
)

func TestInvertedIndexBuildsPostings(t *testing.T) {
	entries := []CommandEntry{
		{Intent: "Delete file", Keywords: []string{"delete", "remove"}, CommandTemplate: "rm {target}", Explanation: "Delete one file", Platform: "linux"},
		{Intent: "List files", Keywords: []string{"list", "files"}, CommandTemplate: "ls -la {path}", Explanation: "List directory contents", Platform: "linux"},
		{Intent: "Delete folder", Keywords: []string{"delete", "folder"}, CommandTemplate: "rm -r {target}", Explanation: "Delete one folder", Platform: "linux"},
	}

	eng := NewEngine(entries)
	deleteDocs := eng.postings["delete"]
	if len(deleteDocs) != 2 {
		t.Fatalf("expected 2 docs for token delete, got %d", len(deleteDocs))
	}
	if deleteDocs[0] != 0 || deleteDocs[1] != 2 {
		t.Fatalf("unexpected postings order for delete: %v", deleteDocs)
	}
}

func TestSearchUsesCandidateSetAndRanksByTFIDF(t *testing.T) {
	entries := []CommandEntry{
		{
			Intent:          "Delete file",
			Keywords:        []string{"delete", "remove", "file"},
			CommandTemplate: "rm {target}",
			Explanation:     "Delete a file quickly",
			Platform:        "linux",
		},
		{
			Intent:          "Review cleanup plan",
			Keywords:        []string{"review", "cleanup", "plan"},
			CommandTemplate: "echo cleanup plan",
			Explanation:     "Review before you delete",
			Platform:        "linux",
		},
		{
			Intent:          "List files",
			Keywords:        []string{"list", "files"},
			CommandTemplate: "ls -la {path}",
			Explanation:     "List directory contents",
			Platform:        "linux",
		},
	}

	eng := NewEngine(entries)
	intent := parser.ParsedIntent{Action: "delete", Raw: "delete file"}
	results := eng.Search(intent, 3)
	if len(results) == 0 {
		t.Fatalf("expected search results")
	}

	if results[0].Entry.Intent != "Delete file" {
		t.Fatalf("expected top result to be Delete file, got %q", results[0].Entry.Intent)
	}

	for _, r := range results {
		if r.Entry.Intent == "List files" {
			t.Fatalf("non-matching command should not be scored when using candidate filtering")
		}
	}
}

func TestSearchCachesByNormalizedQuery(t *testing.T) {
	entries := []CommandEntry{
		{Intent: "Copy files to USB", Keywords: []string{"copy", "files", "usb"}, CommandTemplate: "cp -r {source} /media/usb", Explanation: "Copy files to usb drive", Platform: "linux"},
	}

	eng := NewEngine(entries)

	first := parser.ParsedIntent{Action: "copy", Target: "files", Destination: "usb", Raw: "Copy files to USB!!!"}
	second := parser.ParsedIntent{Action: "copy", Target: "files", Destination: "usb", Raw: "copy files to usb"}

	firstResults := eng.Search(first, 3)
	if len(firstResults) == 0 {
		t.Fatalf("expected first query to return results")
	}
	if eng.cache.Len() != 1 {
		t.Fatalf("expected cache size 1 after first query, got %d", eng.cache.Len())
	}

	secondResults := eng.Search(second, 3)
	if len(secondResults) == 0 {
		t.Fatalf("expected second query to return results")
	}
	if eng.cache.Len() != 1 {
		t.Fatalf("expected normalized query to reuse cache entry, got %d entries", eng.cache.Len())
	}
	if firstResults[0].Entry.Intent != secondResults[0].Entry.Intent {
		t.Fatalf("expected cached result equivalence, got %q and %q", firstResults[0].Entry.Intent, secondResults[0].Entry.Intent)
	}
}

func TestSearchCacheEvictsOldestAtCapacity(t *testing.T) {
	entries := []CommandEntry{
		{Intent: "Copy files", Keywords: []string{"copy", "files", "usb"}, CommandTemplate: "cp -r {source} /media/usb", Explanation: "Copy files to usb", Platform: "linux"},
	}

	eng := NewEngine(entries)
	for i := 0; i < 550; i++ {
		intent := parser.ParsedIntent{
			Action:      "copy",
			Target:      "files",
			Destination: "usb",
			Raw:         fmt.Sprintf("copy files to usb %d", i),
		}
		results := eng.Search(intent, 3)
		if len(results) == 0 {
			t.Fatalf("expected result for query %d", i)
		}
	}

	if eng.cache.Len() != 500 {
		t.Fatalf("expected cache capacity 500, got %d", eng.cache.Len())
	}

	oldestIntent := parser.ParsedIntent{
		Action:      "copy",
		Target:      "files",
		Destination: "usb",
		Raw:         "copy files to usb 0",
	}
	oldestKey := normalizeQuery(IntentToQuery(oldestIntent))
	if _, ok := eng.cache.Get(oldestKey); ok {
		t.Fatalf("expected oldest cache key to be evicted")
	}

	newestIntent := parser.ParsedIntent{
		Action:      "copy",
		Target:      "files",
		Destination: "usb",
		Raw:         "copy files to usb 549",
	}
	newestKey := normalizeQuery(IntentToQuery(newestIntent))
	if _, ok := eng.cache.Get(newestKey); !ok {
		t.Fatalf("expected newest cache key to be present")
	}
}
