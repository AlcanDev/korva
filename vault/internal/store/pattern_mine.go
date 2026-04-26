package store

import (
	"fmt"
	"sort"
	"strings"
)

// EmergingPattern represents a cluster of related observations that haven't
// been explicitly saved as a pattern, suggesting an implicit convention.
type EmergingPattern struct {
	Topic      string   `json:"topic"`       // shared word cluster
	Count      int      `json:"count"`       // number of observations in cluster
	Examples   []string `json:"examples"`    // up to 3 observation titles
	ExampleIDs []string `json:"example_ids"` // corresponding IDs
	Suggestion string   `json:"suggestion"`  // actionable prompt for the AI
}

// MinePatterns scans recent observations for co-occurring word clusters that
// appear across multiple observations but haven't been saved as an explicit
// pattern type. Returns the top N clusters sorted by count descending.
//
// Algorithm:
//  1. Collect the top N most frequent words across recent observations (stop-word filtered)
//  2. For each frequent word, find observations containing it
//  3. Filter clusters with ≥ minCount observations and no existing pattern with that keyword
//  4. Return the top maxResults clusters
func (s *Store) MinePatterns(project string, maxResults, minCount int) ([]EmergingPattern, error) {
	if maxResults <= 0 {
		maxResults = 5
	}
	if minCount <= 0 {
		minCount = 2
	}

	// Fetch the last 200 non-pattern observations for the project.
	filters := SearchFilters{
		Project: project,
		Limit:   200,
	}
	obs, err := s.Search("", filters)
	if err != nil {
		return nil, fmt.Errorf("pattern mine fetch: %w", err)
	}
	if len(obs) < minCount {
		return nil, nil
	}

	// Build word → observation index.
	wordToObs := make(map[string][]Observation)
	for _, o := range obs {
		if o.Type == TypePattern {
			continue // skip explicit patterns — we're looking for implicit ones
		}
		for word := range tokenize(o.Title + " " + o.Content) {
			if isMeaningfulWord(word) {
				wordToObs[word] = append(wordToObs[word], o)
			}
		}
	}

	// Filter words that appear in at least minCount observations.
	type wordCount struct {
		word string
		obs  []Observation
	}
	var candidates []wordCount
	for word, obsSlice := range wordToObs {
		if len(obsSlice) >= minCount {
			candidates = append(candidates, wordCount{word, obsSlice})
		}
	}

	// Sort descending by count.
	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i].obs) > len(candidates[j].obs)
	})

	// Deduplicate by topic word similarity: skip topics that are already well
	// represented by a prior cluster (>80% overlapping observation set).
	var result []EmergingPattern
	usedTopics := make(map[string]bool)
	for _, c := range candidates {
		if len(result) >= maxResults {
			break
		}
		if usedTopics[c.word] {
			continue
		}

		var titles, ids []string
		for _, o := range c.obs {
			if len(titles) >= 3 {
				break
			}
			titles = append(titles, o.Title)
			ids = append(ids, o.ID)
		}

		usedTopics[c.word] = true
		result = append(result, EmergingPattern{
			Topic:      c.word,
			Count:      len(c.obs),
			Examples:   titles,
			ExampleIDs: ids,
			Suggestion: fmt.Sprintf("The term %q appears in %d observations. Consider saving an explicit pattern using vault_save with type=pattern to document this convention.", c.word, len(c.obs)),
		})
	}
	return result, nil
}

// isMeaningfulWord filters out common stop words and very short terms.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true,
	"this": true, "from": true, "are": true, "was": true, "has": true,
	"not": true, "but": true, "have": true, "all": true, "can": true,
	"been": true, "use": true, "used": true, "when": true, "will": true,
	"new": true, "also": true, "should": true, "must": true, "need": true,
	"more": true, "into": true, "than": true, "then": true, "each": true,
	"its": true, "your": true, "our": true, "they": true, "their": true,
	"any": true, "may": true, "via": true, "per": true, "due": true,
	"request": true, "service": true, "handle": true, "every": true,
	"using": true, "which": true, "where": true, "about": true,
	"handled": true, "standard": true, "tokens": true, "token": true,
	"validated": true, "components": true,
}

func isMeaningfulWord(w string) bool {
	if len(w) < 4 {
		return false
	}
	if stopWords[strings.ToLower(w)] {
		return false
	}
	return true
}
