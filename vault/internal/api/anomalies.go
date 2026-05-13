package api

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 9.2 — Anomaly detection for cost/token usage.
//
//   GET /admin/cost/anomalies?days=30
//
// Reads the same daily + by-project series as /admin/cost/summary and
// applies a z-score outlier detector to surface days (and projects) that
// consumed dramatically more than their historical baseline. This is the
// metric finance asks for first: "did anything unusual cost us money
// last week?"
//
// Z-score rationale: simple, well-understood, and works well when the
// baseline distribution is roughly symmetric — which token usage tends
// to be over weekly windows. We skip outlier detection when n < 5 days
// (not enough signal) or when std == 0 (uniform window).

// Anomaly is one detected outlier event.
type Anomaly struct {
	Kind        string  `json:"kind"`         // "daily_spike" | "project_spike"
	Subject     string  `json:"subject"`      // date for daily, project name for per-project
	Tokens      int64   `json:"tokens"`       // observed value
	BaselineAvg float64 `json:"baseline_avg"` // average of the comparable window
	BaselineStd float64 `json:"baseline_std"` // std-dev of the same window
	ZScore      float64 `json:"z_score"`      // (observed - avg) / std
	Severity    string  `json:"severity"`     // "warning" | "danger"
	Suggestion  string  `json:"suggestion"`   // human-readable hint
}

// AnomaliesResponse is the wire shape.
type AnomaliesResponse struct {
	WindowDays int       `json:"window_days"`
	Anomalies  []Anomaly `json:"anomalies"`
}

// Z-score thresholds. >2 = warning, >3 = danger. The values are
// well-trodden in observability; lower thresholds would create alert
// noise on every 20%-busier-than-normal day.
const (
	zThresholdWarn   = 2.0
	zThresholdDanger = 3.0
)

func adminCostAnomalies(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := 30
		if v := r.URL.Query().Get("days"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				if n > 365 {
					n = 365
				}
				days = n
			}
		}
		to := time.Now().UTC()
		from := to.AddDate(0, 0, -days)
		stats, err := s.GetTokenStats(from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var anomalies []Anomaly

		// 1. Daily spikes — compare the last day vs the rest of the window.
		if len(stats.Daily) >= 5 {
			values := make([]float64, len(stats.Daily))
			for i, d := range stats.Daily {
				values[i] = float64(d.InputTokens + d.OutputTokens)
			}
			anomalies = append(anomalies, detectDailySpikes(stats.Daily, values)...)
		}

		// 2. Per-project last-day spikes — if a single project consumed
		// significantly more on the last day vs its own daily average.
		// We approximate per-project average using ByProject totals/days.
		if days > 0 {
			for name, b := range stats.ByProject {
				total := float64(b.InputTokens + b.OutputTokens)
				avgPerDay := total / float64(days)
				// Heuristic spike: today's signal estimated as (project count
				// / total count) * today's tokens. If a project has very few
				// rows but accounts for a large share of today's volume, flag.
				if b.Count == 0 {
					continue
				}
				lastDayTokens := int64(0)
				if len(stats.Daily) > 0 {
					last := stats.Daily[len(stats.Daily)-1]
					lastDayTokens = last.InputTokens + last.OutputTokens
				}
				if lastDayTokens <= 0 || avgPerDay <= 0 {
					continue
				}
				// Simple heuristic — project's daily share much higher than its window share.
				windowShare := total / max64(1, float64(stats.InputTokens+stats.OutputTokens))
				todayShare := float64(b.InputTokens+b.OutputTokens) / float64(lastDayTokens)
				if todayShare > windowShare*3 && b.Count >= 3 {
					anomalies = append(anomalies, Anomaly{
						Kind:        "project_spike",
						Subject:     name,
						Tokens:      b.InputTokens + b.OutputTokens,
						BaselineAvg: avgPerDay,
						Severity:    "warning",
						Suggestion: "Project consumed an unusually large share of today's tokens. " +
							"Inspect the latest sessions for runaway prompts or context bloat.",
					})
				}
			}
		}

		writeJSON(w, http.StatusOK, AnomaliesResponse{
			WindowDays: days,
			Anomalies:  anomalies,
		})
	}
}

// detectDailySpikes returns anomalies for any day whose token total exceeds
// μ + zThresholdWarn × σ. Excludes std=0 distributions (uniform windows).
func detectDailySpikes(rows []store.DailyTokenCount, values []float64) []Anomaly {
	if len(values) < 5 {
		return nil
	}
	avg, std := meanAndStd(values)
	if std <= 0 {
		return nil
	}
	var out []Anomaly
	for i, v := range values {
		z := (v - avg) / std
		if z >= zThresholdWarn {
			sev := "warning"
			if z >= zThresholdDanger {
				sev = "danger"
			}
			out = append(out, Anomaly{
				Kind:        "daily_spike",
				Subject:     rows[i].Date,
				Tokens:      int64(v),
				BaselineAvg: avg,
				BaselineStd: std,
				ZScore:      z,
				Severity:    sev,
				Suggestion:  spikeSuggestion(z, rows[i].Date),
			})
		}
	}
	return out
}

func spikeSuggestion(z float64, date string) string {
	if z >= zThresholdDanger {
		return "Token usage on " + date + " was over 3 standard deviations above the window average. " +
			"Check the Sessions panel for that day and look for runaway agents."
	}
	return "Token usage on " + date + " was above the typical band. " +
		"Worth a quick glance if the trend continues."
}

// meanAndStd returns the arithmetic mean and the (sample) standard deviation
// of a slice of values. Returns (0, 0) for empty input.
func meanAndStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	if len(xs) < 2 {
		return mean, 0
	}
	var sq float64
	for _, x := range xs {
		d := x - mean
		sq += d * d
	}
	return mean, math.Sqrt(sq / float64(len(xs)-1))
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
