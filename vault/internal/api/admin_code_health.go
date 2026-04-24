package api

// admin_code_health.go — aggregated code quality view for the Beacon admin.
//
// GET /admin/code-health
//
// Combines three data sources:
//   - quality_checkpoints: AI-scored phase reviews per project
//   - sdd_state:           current SDD phase per project
//   - observations:        bug/pattern counts per project
//
// Response shape designed for the Beacon Code Health dashboard.

import (
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

type codeHealthProject struct {
	Project          string  `json:"project"`
	SDDPhase         string  `json:"sdd_phase"`
	AvgScore         float64 `json:"avg_score"`
	TotalCheckpoints int     `json:"total_checkpoints"`
	LastScore        int     `json:"last_score"`
	LastStatus       string  `json:"last_status"`
	LastPhase        string  `json:"last_phase"`
	LastCheckedAt    string  `json:"last_checked_at"`
	GatePassed       bool    `json:"gate_passed"`
	BugfixCount      int     `json:"bugfix_count"`
	PatternCount     int     `json:"pattern_count"`
}

type recentCheckpoint struct {
	ID          string `json:"id"`
	Project     string `json:"project"`
	Phase       string `json:"phase"`
	Status      string `json:"status"`
	Score       int    `json:"score"`
	GatePassed  bool   `json:"gate_passed"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
}

// adminCodeHealth returns an aggregated code-health view suitable for
// dashboarding. It is intentionally read-only and has no write side-effects.
//
// GET /admin/code-health
func adminCodeHealth(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// ── 1. Per-project quality summary ───────────────────────────────────────
		projRows, err := s.DB().QueryContext(ctx, `
			SELECT
				qc.project,
				COALESCE(sd.phase, '') AS sdd_phase,
				AVG(qc.score)           AS avg_score,
				COUNT(qc.id)            AS total_checkpoints,
				MAX(qc.score)           AS last_score,
				'' AS last_status,
				'' AS last_phase,
				MAX(qc.created_at)      AS last_checked_at
			FROM quality_checkpoints qc
			LEFT JOIN sdd_state sd ON sd.project = qc.project
			GROUP BY qc.project
			ORDER BY MAX(qc.created_at) DESC
			LIMIT 20`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer projRows.Close()

		projects := make([]codeHealthProject, 0)
		for projRows.Next() {
			var p codeHealthProject
			if err := projRows.Scan(
				&p.Project, &p.SDDPhase, &p.AvgScore, &p.TotalCheckpoints,
				&p.LastScore, &p.LastStatus, &p.LastPhase, &p.LastCheckedAt,
			); err != nil {
				continue
			}
			projects = append(projects, p)
		}
		if err := projRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// ── 2. Fill in last_status / last_phase + bug/pattern counts ─────────────
		for i, p := range projects {
			// Latest checkpoint details for this project.
			s.DB().QueryRowContext(ctx, `
				SELECT phase, status, gate_passed
				  FROM quality_checkpoints
				 WHERE project = ?
				 ORDER BY created_at DESC
				 LIMIT 1`, p.Project).
				Scan(&projects[i].LastPhase, &projects[i].LastStatus, &projects[i].GatePassed) //nolint:errcheck

			// Observation counts for bug/pattern signals.
			s.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM observations
				 WHERE project = ? AND type = 'bugfix'`, p.Project).
				Scan(&projects[i].BugfixCount) //nolint:errcheck
			s.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM observations
				 WHERE project = ? AND type = 'pattern'`, p.Project).
				Scan(&projects[i].PatternCount) //nolint:errcheck
		}

		// ── 3. Recent checkpoints (last 10, across all projects) ─────────────────
		recentRows, err := s.DB().QueryContext(ctx, `
			SELECT id, project, phase, status, score, gate_passed,
			       COALESCE(notes, '') AS notes, created_at
			  FROM quality_checkpoints
			 ORDER BY created_at DESC
			 LIMIT 10`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer recentRows.Close()

		recent := make([]recentCheckpoint, 0)
		for recentRows.Next() {
			var rc recentCheckpoint
			var gatePassed int
			if err := recentRows.Scan(
				&rc.ID, &rc.Project, &rc.Phase, &rc.Status,
				&rc.Score, &gatePassed, &rc.Notes, &rc.CreatedAt,
			); err != nil {
				continue
			}
			rc.GatePassed = gatePassed == 1
			recent = append(recent, rc)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"projects":      projects,
			"recent":        recent,
			"project_count": len(projects),
		})
	}
}
