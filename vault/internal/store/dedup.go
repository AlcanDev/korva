package store

import (
	"strings"
	"unicode"
)

// FindSimilar searches for an existing observation that is semantically similar
// to the candidate obs. Returns (similar, id) when one is found above the
// threshold, or (nil, "") when no match is found.
//
// The similarity metric is a Jaccard coefficient on word-level token sets
// computed against the combined title+content of candidates in the same
// project+type bucket. This is O(n) per save but n is small per project
// in practice, and no index or ML infrastructure is needed.
//
// threshold should be in [0, 1]. 0.70 catches near-duplicates while allowing
// genuine follow-up observations to pass through.
func (s *Store) FindSimilar(candidate Observation, threshold float64) (*Observation, string) {
	if candidate.Content == "" {
		return nil, ""
	}

	filters := SearchFilters{
		Project: candidate.Project,
		Type:    candidate.Type,
		Limit:   50,
	}
	existing, err := s.Search("", filters)
	if err != nil || len(existing) == 0 {
		return nil, ""
	}

	candWords := tokenize(candidate.Title + " " + candidate.Content)
	if len(candWords) < 5 {
		return nil, ""
	}

	for i := range existing {
		e := &existing[i]
		eWords := tokenize(e.Title + " " + e.Content)
		if jaccardSimilarity(candWords, eWords) >= threshold {
			return e, e.ID
		}
	}
	return nil, ""
}

// tokenize splits text into a deduplicated set of lowercase words (≥3 chars),
// stripping punctuation. Stop words are not removed — the heuristic is good
// enough at 70% threshold without a stopword list.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(word) >= 3 {
			words[word] = true
		}
	}
	return words
}

// jaccardSimilarity computes |A∩B| / |A∪B| for two word sets.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
