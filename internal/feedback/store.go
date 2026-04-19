package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"shellai/internal/config"
	"shellai/internal/search"
)

type Store struct {
	missesPath string
	hitsPath   string
}

func (s *Store) MissesPath() string {
	if s == nil {
		return ""
	}
	return s.missesPath
}

func (s *Store) HitsPath() string {
	if s == nil {
		return ""
	}
	return s.hitsPath
}

type missEntry struct {
	Query    string `json:"query"`
	Returned string `json:"returned"`
}

type EntryHit struct {
	Intent string
	Hits   int
}

func NewStore() *Store {
	dir := config.ConfigDir()
	return &Store{
		missesPath: filepath.Join(dir, "misses.log"),
		hitsPath:   filepath.Join(dir, "hits.json"),
	}
}

func (s *Store) RecordMiss(query, returned string) error {
	if s == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.missesPath), 0o755); err != nil {
		return fmt.Errorf("create feedback directory: %w", err)
	}

	entry := missEntry{
		Query:    strings.TrimSpace(query),
		Returned: strings.TrimSpace(returned),
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode miss entry: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(s.missesPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open misses log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write misses log: %w", err)
	}
	return nil
}

func (s *Store) RecordMatches(matches []search.ScoredMatch) error {
	if s == nil || len(matches) == 0 {
		return nil
	}

	hits, err := s.loadHits()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	for _, match := range matches {
		intent := normalizeIntent(match.Entry.Intent)
		if intent == "" {
			continue
		}
		if _, ok := seen[intent]; ok {
			continue
		}
		seen[intent] = struct{}{}
		hits[intent]++
	}

	if err := s.saveHits(hits); err != nil {
		return err
	}
	return nil
}

func (s *Store) NeverMatched(entries []search.CommandEntry) ([]string, error) {
	hits, err := s.loadHits()
	if err != nil {
		return nil, err
	}

	never := make([]string, 0)
	seen := make(map[string]struct{})
	for _, entry := range entries {
		intent := normalizeIntent(entry.Intent)
		if intent == "" {
			continue
		}
		if _, ok := seen[intent]; ok {
			continue
		}
		seen[intent] = struct{}{}
		if hits[intent] == 0 {
			never = append(never, entry.Intent)
		}
	}

	sort.Strings(never)
	return never, nil
}

func (s *Store) TopHits(limit int) ([]EntryHit, error) {
	if limit <= 0 {
		limit = 10
	}
	hits, err := s.loadHits()
	if err != nil {
		return nil, err
	}

	items := make([]EntryHit, 0, len(hits))
	for intent, count := range hits {
		items = append(items, EntryHit{Intent: intent, Hits: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Hits == items[j].Hits {
			return items[i].Intent < items[j].Intent
		}
		return items[i].Hits > items[j].Hits
	})
	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], nil
}

func (s *Store) loadHits() (map[string]int, error) {
	if s == nil {
		return map[string]int{}, nil
	}
	data, err := os.ReadFile(s.hitsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("read hits file: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]int{}, nil
	}

	var hits map[string]int
	if err := json.Unmarshal(data, &hits); err != nil {
		return nil, fmt.Errorf("parse hits file: %w", err)
	}
	if hits == nil {
		hits = map[string]int{}
	}
	return hits, nil
}

func (s *Store) saveHits(hits map[string]int) error {
	if s == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.hitsPath), 0o755); err != nil {
		return fmt.Errorf("create feedback directory: %w", err)
	}
	data, err := json.MarshalIndent(hits, "", "  ")
	if err != nil {
		return fmt.Errorf("encode hits file: %w", err)
	}
	if err := os.WriteFile(s.hitsPath, data, 0o644); err != nil {
		return fmt.Errorf("write hits file: %w", err)
	}
	return nil
}

func normalizeIntent(intent string) string {
	return strings.ToLower(strings.TrimSpace(intent))
}
