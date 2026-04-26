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
	defer func() { _ = rows.Close() }()

	var scores []ProjectHealthScore
	for rows.Next() {
		var p ProjectHealthScore
		if err := rows.Scan(&p.Project, &p.SDDPhase, &p.AvgQAScore, &p.GatePassRate, &p.RecentCheckpoints); err != nil {
			return nil, err
		}

		// Fetch observation counts
		s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ? AND type = 'bugfix'`, p.Project).Scan(&p.BugfixCount)   //nolint:errcheck
		s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ? AND type = 'pattern'`, p.Project).Scan(&p.PatternCount) //nolint:errcheck

		// Trend: compare last 3 scores vs previous 3 scores
		p.Trend = computeTrend(s, p.Project)

		// Composite score
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

		scores = append(scores, p)
	}
	if scores == nil {
		scores = []ProjectHealthScore{}
	}
	return scores, rows.Err()
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
	defer func() { _ = rows.Close() }()

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
