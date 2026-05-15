package store

import "fmt"

// ProjectHealthScore is the composite code health score for one project.
// Score is in [0, 100] — higher is better.
type ProjectHealthScore struct {
	Project           string  `json:"project"`
	Score             int     `json:"score"`
	Grade             string  `json:"grade"` // A/B/C/D/F
	SDDPhase          string  `json:"sdd_phase"`
	AvgQAScore        float64 `json:"avg_qa_score"`
	GatePassRate      float64 `json:"gate_pass_rate"` // 0-1
	RecentCheckpoints int     `json:"recent_checkpoints"`
	BugfixCount       int     `json:"bugfix_count"`
	PatternCount      int     `json:"pattern_count"`
	Trend             string  `json:"trend"` // "improving" / "declining" / "stable"
}

// CodeHealthSummary returns a composite code health score per project.
// Score formula:
//   - 50% avg QA score (quality_checkpoints.score, normalized 0-100)
//   - 30% gate pass rate (fraction of checkpoints where gate_passed = 1)
//   - 20% pattern signal (patterns / (patterns + bugfixes), capped)
//
// Phase 20.A — earlier versions made nested QueryRow calls inside
// the rows.Next() loop. Combined with `SetMaxOpenConns(1)` from
// internal/db/sqlite.go, that deadlocked in production: the
// iterator owned the connection while the inner query waited for
// one. The fix is to drain the outer rows into a slice first, then
// run the per-project enrichment queries against a freed
// connection.
func (s *Store) CodeHealthSummary() ([]ProjectHealthScore, error) {
	rows, err := s.db.Query(`
		SELECT
			qc.project,
			COALESCE(sd.phase, 'planning') AS sdd_phase,
			AVG(CAST(qc.score AS REAL))              AS avg_score,
			CAST(SUM(qc.gate_passed) AS REAL) / COUNT(*) AS pass_rate,
			COUNT(*)                                 AS total
		FROM quality_checkpoints qc
		LEFT JOIN sdd_state sd ON sd.project = qc.project
		GROUP BY qc.project
		ORDER BY avg_score DESC
		LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("code health query: %w", err)
	}

	// Drain all rows into the result slice WITHOUT making any
	// nested queries — releases the connection.
	var scores []ProjectHealthScore
	for rows.Next() {
		var p ProjectHealthScore
		if err := rows.Scan(&p.Project, &p.SDDPhase, &p.AvgQAScore, &p.GatePassRate, &p.RecentCheckpoints); err != nil {
			rows.Close()
			return nil, err
		}
		scores = append(scores, p)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Now safe to make per-project enrichment queries.
	for i := range scores {
		p := &scores[i]
		s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ? AND type = 'bugfix'`, p.Project).Scan(&p.BugfixCount)   //nolint:errcheck
		s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ? AND type = 'pattern'`, p.Project).Scan(&p.PatternCount) //nolint:errcheck

		p.Trend = computeTrend(s, p.Project)

		patternSignal := 0.5
		if total := p.PatternCount + p.BugfixCount; total > 0 {
			patternSignal = float64(p.PatternCount) / float64(total)
		}
		raw := 0.5*p.AvgQAScore + 0.3*p.GatePassRate*100 + 0.2*patternSignal*100
		if raw > 100 {
			raw = 100
		}
		p.Score = int(raw)
		p.Grade = scoreGrade(p.Score)
	}

	if scores == nil {
		scores = []ProjectHealthScore{}
	}
	return scores, nil
}

// computeTrend compares the average of the last 3 QA scores vs the 3 before that.
func computeTrend(s *Store, project string) string {
	rows, err := s.db.Query(`
		SELECT score FROM quality_checkpoints
		WHERE project = ?
		ORDER BY created_at DESC
		LIMIT 6`, project)
	if err != nil {
		return "stable"
	}
	defer rows.Close()

	var scores []float64
	for rows.Next() {
		var sc int
		if rows.Scan(&sc) == nil {
			scores = append(scores, float64(sc))
		}
	}

	if len(scores) < 4 {
		return "stable"
	}

	recent := avg(scores[:3])
	older := avg(scores[3:])
	switch {
	case recent > older+5:
		return "improving"
	case recent < older-5:
		return "declining"
	default:
		return "stable"
	}
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func scoreGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
