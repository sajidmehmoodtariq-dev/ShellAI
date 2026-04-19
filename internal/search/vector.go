package search

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"shellai/internal/parser"
)

type CommandFlag struct {
	Flag        string `json:"flag"`
	Description string `json:"description"`
}

type CommandEntry struct {
	Intent          string        `json:"intent"`
	Keywords        []string      `json:"keywords"`
	CommandTemplate string        `json:"command_template"`
	Explanation     string        `json:"explanation"`
	Flags           []CommandFlag `json:"flags"`
	Danger          bool          `json:"danger"`
	Platform        string        `json:"platform"`
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
}

var nonWordPattern = regexp.MustCompile(`[^a-z0-9]+`)
var whitespacePattern = regexp.MustCompile(`\s+`)

func NewEngineFromJSON(path string) (*Engine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read commands database: %w", err)
	}

	var entries []CommandEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse commands database: %w", err)
	}

	return NewEngine(entries), nil
}

func NewEngine(entries []CommandEntry) *Engine {
	docTF := make([]map[string]float64, len(entries))
	docFreq := make(map[string]int)

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
	}
}

func (e *Engine) Search(intent parser.ParsedIntent, topK int) []ScoredMatch {
	if topK <= 0 || len(e.entries) == 0 {
		return nil
	}

	query := IntentToQuery(intent)
	queryVec, queryNorm := e.queryVector(query)
	if queryNorm == 0 {
		return nil
	}

	results := make([]ScoredMatch, 0, len(e.entries))
	for i, docVec := range e.vectors {
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

	if topK > len(results) {
		topK = len(results)
	}

	return results[:topK]
}

func IntentToQuery(intent parser.ParsedIntent) string {
	parts := []string{intent.Action, intent.Target, intent.Destination, intent.Raw}
	for _, f := range intent.Filters {
		parts = append(parts, f.Type, f.Value)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
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
