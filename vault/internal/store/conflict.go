package store

import "strings"

// conflictPairs is a list of antonym word pairs. When an incoming decision
// contains one pole and an existing decision contains the other pole, we flag
// a potential conflict. This list is intentionally small — false negatives are
// preferable to annoying false positives.
var conflictPairs = [][2]string{
	{"monolith", "microservice"},
	{"synchronous", "asynchronous"},
	{"sync", "async"},
	{"sql", "nosql"},
	{"relational", "document"},
	{"postgres", "mongo"},
	{"mysql", "cassandra"},
	{"rest", "graphql"},
	{"grpc", "rest"},
	{"jwt", "session"},
	{"cookie", "bearer"},
	{"redis", "memcached"},
	{"kubernetes", "serverless"},
	{"docker", "bare"},
	{"encrypt", "plaintext"},
	{"allow", "deny"},
	{"whitelist", "blacklist"},
	{"vertical", "horizontal"},
	{"eager", "lazy"},
}

// ConflictWarning describes a potential conflict between two decisions.
type ConflictWarning struct {
	ExistingID    string `json:"existing_id"`
	ExistingTitle string `json:"existing_title"`
	ConflictWord  string `json:"conflict_word"` // word in new decision
	OppositeWord  string `json:"opposite_word"` // contradicting word in existing decision
	Explanation   string `json:"explanation"`
}

// FindDecisionConflicts checks whether the incoming decision (title+content)
// potentially contradicts existing decisions in the same project.
//
// Returns up to 3 conflicts to keep the warning focused.
func (s *Store) FindDecisionConflicts(candidate Observation) []ConflictWarning {
	if candidate.Type != TypeDecision && string(candidate.Type) != "decision" {
		return nil
	}

	filters := SearchFilters{
		Project: candidate.Project,
		Type:    TypeDecision,
		Limit:   100,
	}
	existing, err := s.Search("", filters)
	if err != nil {
		return nil
	}

	candText := strings.ToLower(candidate.Title + " " + candidate.Content)
	var warnings []ConflictWarning

	for _, e := range existing {
		eText := strings.ToLower(e.Title + " " + e.Content)
		for _, pair := range conflictPairs {
			a, b := pair[0], pair[1]
			// New decision uses "a" AND existing decision uses "b", or vice versa.
			newUsesA := strings.Contains(candText, a)
			newUsesB := strings.Contains(candText, b)
			existUsesA := strings.Contains(eText, a)
			existUsesB := strings.Contains(eText, b)

			var conflictWord, oppositeWord string
			if newUsesA && existUsesB && !newUsesB {
				conflictWord, oppositeWord = a, b
			} else if newUsesB && existUsesA && !newUsesA {
				conflictWord, oppositeWord = b, a
			}

			if conflictWord != "" {
				warnings = append(warnings, ConflictWarning{
					ExistingID:    e.ID,
					ExistingTitle: e.Title,
					ConflictWord:  conflictWord,
					OppositeWord:  oppositeWord,
					Explanation:   "This decision uses \"" + conflictWord + "\" but an existing decision uses \"" + oppositeWord + "\". Review before committing.",
				})
				if len(warnings) >= 3 {
					return warnings
				}
				break // one warning per existing decision is enough
			}
		}
	}
	return warnings
}
