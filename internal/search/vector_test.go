package search

import (
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
