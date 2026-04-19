package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"shellai/internal/parser"
)

type CommandFlag struct {
	Flag        string `json:"flag" yaml:"flag"`
	Description string `json:"description" yaml:"description"`
}

type CommandEntry struct {
	Intent          string        `json:"intent" yaml:"intent"`
	Keywords        []string      `json:"keywords" yaml:"keywords"`
	CommandTemplate string        `json:"command_template" yaml:"command_template"`
	Explanation     string        `json:"explanation" yaml:"explanation"`
	Flags           []CommandFlag `json:"flags" yaml:"flags"`
	Danger          bool          `json:"danger" yaml:"danger"`
	Platform        string        `json:"platform" yaml:"platform"`
}

type ScoredMatch struct {
	Entry CommandEntry
	Score float64
}

type Engine struct {
	entries  []CommandEntry
	vectors  []map[string]float64
	idf      map[string]float64
	vecNorms []float64
	postings map[string][]int
	cache    *resultCache
}

var nonWordPattern = regexp.MustCompile(`[^a-z0-9]+`)
var whitespacePattern = regexp.MustCompile(`\s+`)

const (
	searchCacheCapacity = 500
	searchCacheTopK     = 3
)

func NewEngineFromJSON(path string) (*Engine, error) {
	entries, err := LoadEntriesFromJSON(path)
	if err != nil {
		return nil, err
	}
	return NewEngine(entries), nil
}

func NewEngineFromDatabases(corePath, userPath string) (*Engine, error) {
	core, err := LoadEntriesFromJSON(corePath)
	if err != nil {
		return nil, err
	}
	user, err := LoadEntriesFromJSON(userPath)
	if err != nil {
		return nil, err
	}
	return NewEngine(MergeEntriesByIntent(core, user)), nil
}

func LoadEntriesFromJSON(path string) ([]CommandEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read commands database %s: %w", path, err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}

	var entries []CommandEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse commands database %s: %w", path, err)
	}
	return entries, nil
}

func MergeEntriesByIntent(core, user []CommandEntry) []CommandEntry {
	merged := make([]CommandEntry, 0, len(core)+len(user))
	index := make(map[string]int)

	for _, entry := range core {
		key := normalizeIntentKey(entry.Intent)
		if key == "" {
			continue
		}
		index[key] = len(merged)
		merged = append(merged, entry)
	}

	for _, entry := range user {
		key := normalizeIntentKey(entry.Intent)
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			merged[idx] = entry
			continue
		}
		index[key] = len(merged)
		merged = append(merged, entry)
	}

	return merged
}

func normalizeIntentKey(intent string) string {
	return strings.ToLower(strings.TrimSpace(intent))
}

func NewEngine(entries []CommandEntry) *Engine {
	docTF := make([]map[string]float64, len(entries))
	docFreq := make(map[string]int)
	postings := make(map[string][]int)

	for i, entry := range entries {
		text := entryToSearchText(entry)
		tf := termFrequency(tokenize(text))
		docTF[i] = tf

		seen := make(map[string]struct{})
		for token := range tf {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			docFreq[token]++
			postings[token] = append(postings[token], i)
		}
	}

	n := float64(len(entries))
	idf := make(map[string]float64, len(docFreq))
	for token, df := range docFreq {
		idf[token] = math.Log((1.0+n)/(1.0+float64(df))) + 1.0
	}

	vectors := make([]map[string]float64, len(entries))
	vecNorms := make([]float64, len(entries))
	for i, tf := range docTF {
		vec := make(map[string]float64, len(tf))
		var sumSq float64
		for token, value := range tf {
			weight := value * idf[token]
			vec[token] = weight
			sumSq += weight * weight
		}
		vectors[i] = vec
		vecNorms[i] = math.Sqrt(sumSq)
	}

	return &Engine{
		entries:  entries,
		vectors:  vectors,
		idf:      idf,
		vecNorms: vecNorms,
		postings: postings,
		cache:    newResultCache(searchCacheCapacity),
	}
}

func (e *Engine) Search(intent parser.ParsedIntent, topK int) []ScoredMatch {
	if topK <= 0 || len(e.entries) == 0 {
		return nil
	}

	query := IntentToQuery(intent)
	normalizedQuery := normalizeQuery(query)
	if normalizedQuery == "" {
		return nil
	}

	if topK <= searchCacheTopK {
		if cached, ok := e.cache.Get(normalizedQuery); ok {
			if topK > len(cached) {
				topK = len(cached)
			}
			return cloneMatches(cached, topK)
		}
	}

	queryVec, queryNorm := e.queryVector(normalizedQuery)
	if queryNorm == 0 {
		return nil
	}

	candidates := e.candidateDocIDs(queryVec)
	if len(candidates) == 0 {
		return nil
	}

	results := make([]ScoredMatch, 0, len(candidates))
	for _, i := range candidates {
		docVec := e.vectors[i]
		docNorm := e.vecNorms[i]
		if docNorm == 0 {
			continue
		}

		dot := 0.0
		for token, qWeight := range queryVec {
			if dWeight, ok := docVec[token]; ok {
				dot += qWeight * dWeight
			}
		}

		score := dot / (queryNorm * docNorm)
		if score > 0 {
			results = append(results, ScoredMatch{Entry: e.entries[i], Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Entry.Intent < results[j].Entry.Intent
		}
		return results[i].Score > results[j].Score
	})

	cacheCount := searchCacheTopK
	if cacheCount > len(results) {
		cacheCount = len(results)
	}
	if cacheCount > 0 {
		e.cache.Put(normalizedQuery, cloneMatches(results, cacheCount))
	}

	if topK > len(results) {
		topK = len(results)
	}

	return cloneMatches(results, topK)
}

func (e *Engine) candidateDocIDs(queryVec map[string]float64) []int {
	if len(queryVec) == 0 {
		return nil
	}

	seen := make(map[int]struct{})
	candidates := make([]int, 0)
	for token := range queryVec {
		docs, ok := e.postings[token]
		if !ok {
			continue
		}
		for _, id := range docs {
			if _, already := seen[id]; already {
				continue
			}
			seen[id] = struct{}{}
			candidates = append(candidates, id)
		}
	}

	return candidates
}

func IntentToQuery(intent parser.ParsedIntent) string {
	parts := []string{intent.Action, intent.Target, intent.Destination, intent.Raw}
	for _, f := range intent.Filters {
		parts = append(parts, f.Type, f.Value)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func normalizeQuery(text string) string {
	return strings.Join(tokenize(text), " ")
}

func (e *Engine) queryVector(text string) (map[string]float64, float64) {
	tf := termFrequency(tokenize(text))
	vec := make(map[string]float64, len(tf))
	var sumSq float64

	for token, value := range tf {
		idf, ok := e.idf[token]
		if !ok {
			continue
		}
		weight := value * idf
		vec[token] = weight
		sumSq += weight * weight
	}

	return vec, math.Sqrt(sumSq)
}

func entryToSearchText(entry CommandEntry) string {
	parts := []string{entry.Intent, entry.CommandTemplate, entry.Explanation, entry.Platform}
	parts = append(parts, entry.Keywords...)
	for _, flag := range entry.Flags {
		parts = append(parts, flag.Flag, flag.Description)
	}
	return strings.Join(parts, " ")
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	clean := nonWordPattern.ReplaceAllString(lower, " ")
	clean = whitespacePattern.ReplaceAllString(strings.TrimSpace(clean), " ")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, " ")
}

func termFrequency(tokens []string) map[string]float64 {
	freq := make(map[string]float64)
	if len(tokens) == 0 {
		return freq
	}

	for _, token := range tokens {
		if token == "" {
			continue
		}
		freq[token]++
	}

	den := float64(len(tokens))
	for token, count := range freq {
		freq[token] = count / den
	}

	return freq
}

func cloneMatches(matches []ScoredMatch, n int) []ScoredMatch {
	if n <= 0 || len(matches) == 0 {
		return nil
	}
	if n > len(matches) {
		n = len(matches)
	}
	cloned := make([]ScoredMatch, n)
	copy(cloned, matches[:n])
	return cloned
}
