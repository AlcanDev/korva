package api

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 10.4 — Pattern insights endpoint.
//
//   GET /admin/insights/patterns?project=X&min_count=3
//
// Inspects observations for recurring topical clusters that *aren't*
// already declared as `pattern` or `decision`. When the same n-gram / topic
// appears in N observations (default ≥ 3) but no formal `pattern`-typed
// observation exists for it, we suggest the operator promote it.
//
// Why this matters: teams accidentally accumulate informal patterns —
// "we always use ULID over UUID", "every controller logs to OpenTelemetry"
// — that live as 5 different `context` rows instead of 1 canonical
// `pattern`. The insight nudges the operator to consolidate.
//
// Implementation: light tokenization + n-gram counting. No ML — clarity
// and determinism beat fancy here; the operator decides whether to act.

// PatternSuggestion is one recurring topic we noticed.
type PatternSuggestion struct {
	Phrase         string   `json:"phrase"`
	Project        string   `json:"project"`
	Count          int      `json:"count"`           // distinct observations that mention the phrase
	SampleIDs      []string `json:"sample_ids"`      // up to 5 obs IDs for drill-down
	SampleTitles   []string `json:"sample_titles"`   // matching titles for the same IDs
	AlreadyPattern bool     `json:"already_pattern"` // true when at least one matching obs is type=pattern
	Severity       string   `json:"severity"`        // "info" | "high"
}

// InsightsResponse is the wire shape.
type InsightsResponse struct {
	Project     string              `json:"project"`
	WindowDays  int                 `json:"window_days,omitempty"`
	Suggestions []PatternSuggestion `json:"suggestions"`
}

// minPhraseTokens / maxPhraseTokens bound the n-gram sizes we extract.
// Bigrams + trigrams catch most useful phrases ("always log errors", "use
// ulid") without exploding the candidate space.
const (
	minPhraseTokens = 2
	maxPhraseTokens = 3
	minPhraseLen    = 8 // chars; filters out junk like "of the"
)

// stopWords are common English fillers we strip before tokenization. Keep
// the list short — over-zealous stopword filtering buries genuine patterns.
var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "of": {}, "to": {}, "in": {}, "on": {}, "at": {},
	"for": {}, "with": {}, "is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "by": {}, "as": {}, "from": {},
	"that": {}, "this": {}, "it": {}, "its": {}, "we": {}, "our": {}, "us": {}, "you": {}, "your": {},
}

// wordRegex keeps unicode letters + digits + hyphen/underscore so identifiers
// survive (e.g. "korva-vault", "snake_case_var").
var wordRegex = regexp.MustCompile(`[\p{L}\p{N}_-]+`)

func adminPatternInsights(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		minCount := 3
		if v := r.URL.Query().Get("min_count"); v != "" {
			if n, err := parseClampedInt(v, 2, 50, 3); err == nil {
				minCount = n
			}
		}

		filters := store.SearchFilters{Limit: 1000}
		if project != "" {
			filters.Project = project
		}
		obs, err := s.Search("", filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		suggestions := computePatternSuggestions(obs, minCount)
		writeJSON(w, http.StatusOK, InsightsResponse{
			Project:     project,
			Suggestions: suggestions,
		})
	}
}

// computePatternSuggestions extracts n-grams from observation titles +
// content, counts cross-observation occurrences, and returns the top
// recurring phrases that aren't already covered by a `pattern` row.
func computePatternSuggestions(obs []store.Observation, minCount int) []PatternSuggestion {
	// phraseStats[phrase] = {set of obs IDs, set of titles, already-pattern flag, project}
	type stat struct {
		obsIDs         map[string]struct{}
		obsTitles      map[string]string // id → title
		alreadyPattern bool
		project        string
	}
	stats := make(map[string]*stat)

	for _, o := range obs {
		project := o.Project
		title := o.Title
		text := title + " " + o.Content
		phrases := extractPhrases(text)
		for ph := range phrases {
			st, ok := stats[ph]
			if !ok {
				st = &stat{
					obsIDs:    make(map[string]struct{}),
					obsTitles: make(map[string]string),
					project:   project,
				}
				stats[ph] = st
			}
			st.obsIDs[o.ID] = struct{}{}
			st.obsTitles[o.ID] = title
			if o.Type == store.TypePattern {
				st.alreadyPattern = true
			}
		}
	}

	var out []PatternSuggestion
	for phrase, st := range stats {
		count := len(st.obsIDs)
		if count < minCount {
			continue
		}
		ids := make([]string, 0, len(st.obsIDs))
		for id := range st.obsIDs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 5 {
			ids = ids[:5]
		}
		titles := make([]string, 0, len(ids))
		for _, id := range ids {
			titles = append(titles, st.obsTitles[id])
		}
		severity := "info"
		if count >= 5 && !st.alreadyPattern {
			severity = "high"
		}
		out = append(out, PatternSuggestion{
			Phrase:         phrase,
			Project:        st.project,
			Count:          count,
			SampleIDs:      ids,
			SampleTitles:   titles,
			AlreadyPattern: st.alreadyPattern,
			Severity:       severity,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		// High severity first, then by count desc.
		if out[i].Severity != out[j].Severity {
			return out[i].Severity == "high"
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > 30 {
		out = out[:30]
	}
	return out
}

// extractPhrases returns the set of n-grams (length minPhraseTokens..max)
// extracted from text, lowercased + stripped of stop-words. A set instead
// of a multiset because we count cross-observation occurrences, not
// in-document frequency.
func extractPhrases(text string) map[string]struct{} {
	tokens := tokenize(text)
	out := make(map[string]struct{})
	for n := minPhraseTokens; n <= maxPhraseTokens; n++ {
		for i := 0; i+n <= len(tokens); i++ {
			phrase := strings.Join(tokens[i:i+n], " ")
			if len(phrase) < minPhraseLen {
				continue
			}
			out[phrase] = struct{}{}
		}
	}
	return out
}

// tokenize lowercases, splits on non-letter chars, and drops stopwords.
func tokenize(text string) []string {
	words := wordRegex.FindAllString(text, -1)
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(w)
		if _, stop := stopWords[w]; stop {
			continue
		}
		if len(w) < 3 {
			continue
		}
		out = append(out, w)
	}
	return out
}

// parseClampedInt parses v as int and clamps it to [lo, hi]. Returns def on
// parse failure.
func parseClampedInt(v string, lo, hi, def int) (int, error) {
	var n int
	if _, err := fmtSscanf(v, &n); err != nil || n <= 0 {
		return def, err
	}
	if n < lo {
		n = lo
	}
	if n > hi {
		n = hi
	}
	return n, nil
}

// fmtSscanf is a tiny shim so the import list stays small (we'd otherwise
// pull in fmt just for Sscanf).
func fmtSscanf(s string, p *int) (int, error) {
	var n int
	count := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			if count == 0 {
				return 0, errBadInt
			}
			break
		}
		n = n*10 + int(c-'0')
		count++
	}
	if count == 0 {
		return 0, errBadInt
	}
	*p = n
	return count, nil
}

var errBadInt = stringError("not a positive integer")

type stringError string

func (e stringError) Error() string { return string(e) }
