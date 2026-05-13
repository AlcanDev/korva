package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 10.5 — Decision drift detector.
//
//   GET /admin/drift/decisions?project=X&days=30
//
// Looks for `decision`-typed observations whose declared rule may have
// been violated by *more recent* observations. Heuristic:
//   1. Find every TypeDecision row in the project
//   2. For each, find newer observations in the same project that share a
//      high token overlap AND have type ∈ {bugfix, antipattern, incident,
//      refactor} — i.e. an outcome that often implies a violation
//   3. Surface the pair to the operator with the decision title, the
//      potentially-violating observation, and the overlap score
//
// This is a heuristic, not a proof. The dashboard shows the pair side-by-
// side with a "Confirm violation" + "Dismiss" choice — the operator
// decides if it's a real drift.
//
// Why this matters: decisions silently age. Without a periodic check,
// teams forget why they decided X six months ago and the code drifts.
// Highlighting candidate drifts turns institutional knowledge into a
// live signal.

// DriftAlert is one detected drift candidate.
type DriftAlert struct {
	DecisionID      string    `json:"decision_id"`
	DecisionTitle   string    `json:"decision_title"`
	DecisionCreated time.Time `json:"decision_created"`
	ViolatorID      string    `json:"violator_id"`
	ViolatorTitle   string    `json:"violator_title"`
	ViolatorType    string    `json:"violator_type"`
	ViolatorCreated time.Time `json:"violator_created"`
	OverlapScore    float64   `json:"overlap_score"` // 0..1
	Project         string    `json:"project"`
	Severity        string    `json:"severity"` // info | warning | danger
	Reason          string    `json:"reason"`
}

// DriftResponse is the wire shape.
type DriftResponse struct {
	Project    string       `json:"project"`
	WindowDays int          `json:"window_days"`
	Alerts     []DriftAlert `json:"alerts"`
}

// violationTypes are observation types whose presence likely implies the
// decision didn't hold. The operator can drill in and either confirm or
// dismiss.
var violationTypes = map[store.ObservationType]struct{}{
	store.TypeBugfix:      {},
	store.TypeAntiPattern: {},
	store.TypeIncident:    {},
	store.TypeRefactor:    {},
}

// minOverlapScore is the Jaccard threshold below which we don't bother
// surfacing the pair. 0.2 catches genuinely related topics without flooding
// the dashboard with false positives.
const minOverlapScore = 0.2

func adminDecisionDrift(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		days := 30
		if v := r.URL.Query().Get("days"); v != "" {
			if n, err := parseClampedInt(v, 1, 365, 30); err == nil {
				days = n
			}
		}

		// Pull a wide window of observations.
		filters := store.SearchFilters{Limit: 2000}
		if project != "" {
			filters.Project = project
		}
		obs, err := s.Search("", filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		alerts := computeDriftAlerts(obs, days)
		writeJSON(w, http.StatusOK, DriftResponse{
			Project:    project,
			WindowDays: days,
			Alerts:     alerts,
		})
	}
}

// computeDriftAlerts is the pure-logic core. Takes a slice of observations
// (any project mix) and a days window; returns ranked alerts. Splitting
// the handler from this fn keeps unit tests trivial.
func computeDriftAlerts(obs []store.Observation, days int) []DriftAlert {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	// Partition rows.
	decisions := make([]store.Observation, 0)
	violators := make([]store.Observation, 0)
	for _, o := range obs {
		if o.Type == store.TypeDecision {
			decisions = append(decisions, o)
			continue
		}
		if _, ok := violationTypes[o.Type]; ok && o.CreatedAt.After(cutoff) {
			violators = append(violators, o)
		}
	}

	// Pre-tokenize once.
	type tokenized struct {
		obs store.Observation
		set map[string]struct{}
	}
	tokenize := func(o store.Observation) tokenized {
		toks := tokenizeForDrift(o.Title + " " + o.Content)
		set := make(map[string]struct{}, len(toks))
		for _, t := range toks {
			set[t] = struct{}{}
		}
		return tokenized{obs: o, set: set}
	}
	dTok := make([]tokenized, 0, len(decisions))
	for _, d := range decisions {
		dTok = append(dTok, tokenize(d))
	}
	vTok := make([]tokenized, 0, len(violators))
	for _, v := range violators {
		vTok = append(vTok, tokenize(v))
	}

	var alerts []DriftAlert
	for _, dec := range dTok {
		for _, vio := range vTok {
			if dec.obs.Project != vio.obs.Project {
				continue
			}
			if !vio.obs.CreatedAt.After(dec.obs.CreatedAt) {
				// The violator must be newer than the decision to be drift.
				continue
			}
			score := jaccard(dec.set, vio.set)
			if score < minOverlapScore {
				continue
			}
			alerts = append(alerts, DriftAlert{
				DecisionID:      dec.obs.ID,
				DecisionTitle:   dec.obs.Title,
				DecisionCreated: dec.obs.CreatedAt,
				ViolatorID:      vio.obs.ID,
				ViolatorTitle:   vio.obs.Title,
				ViolatorType:    string(vio.obs.Type),
				ViolatorCreated: vio.obs.CreatedAt,
				OverlapScore:    score,
				Project:         dec.obs.Project,
				Severity:        severityForOverlap(score),
				Reason: "A " + string(vio.obs.Type) + " observation lands after a related " +
					"decision and shares significant terminology — confirm whether the decision still holds.",
			})
		}
	}

	// Sort: severity desc (danger > warning > info), then score desc.
	sevWeight := map[string]int{"danger": 3, "warning": 2, "info": 1}
	sort.SliceStable(alerts, func(i, j int) bool {
		si, sj := sevWeight[alerts[i].Severity], sevWeight[alerts[j].Severity]
		if si != sj {
			return si > sj
		}
		return alerts[i].OverlapScore > alerts[j].OverlapScore
	})
	if len(alerts) > 50 {
		alerts = alerts[:50]
	}
	return alerts
}

func severityForOverlap(score float64) string {
	switch {
	case score >= 0.5:
		return "danger"
	case score >= 0.35:
		return "warning"
	default:
		return "info"
	}
}

// jaccard returns |A ∩ B| / |A ∪ B|. 0 when both empty.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// tokenizeForDrift shares the same lowercasing + stopword stripping as the
// insights detector but allows shorter tokens (≥ 4 chars) — drift cares
// about technical terms like "ulid", "tls", "rls".
func tokenizeForDrift(text string) []string {
	words := wordRegex.FindAllString(text, -1)
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(w)
		if _, stop := stopWords[w]; stop {
			continue
		}
		if len(w) < 4 {
			continue
		}
		out = append(out, w)
	}
	return out
}
