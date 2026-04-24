package api

// skills_sync_telemetry.go — sync reporting and developer status endpoints.
//
//   POST /team/skills/sync/report   — CLI posts after a successful sync
//   GET  /admin/skills/sync-status  — admin sees who is up-to-date

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// teamReportSkillSync records one sync event from the CLI.
// Called automatically after `korva skills sync` completes successfully.
//
// POST /team/skills/sync/report
// Body: { "skills_count": 5, "target": "/home/user/.claude" }
func teamReportSkillSync(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)

		var body struct {
			SkillsCount int    `json:"skills_count"`
			Target      string `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		id := newID()
		now := time.Now().UTC().Format(time.RFC3339)

		if _, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO skill_sync_log (id, team_id, user_email, synced_at, skills_count, target)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, sess.teamID, sess.email, now, body.SkillsCount, body.Target,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"status": "reported", "id": id})
	}
}

// syncStatusEntry is the per-developer wire type for the sync-status endpoint.
type syncStatusEntry struct {
	UserEmail   string `json:"user_email"`
	LastSync    string `json:"last_sync"`
	SkillsCount int    `json:"skills_count"`
	Target      string `json:"target"`
	IsUpToDate  bool   `json:"is_up_to_date"`
}

// adminSkillsSyncStatus returns the latest sync entry per developer for a team.
// Includes an is_up_to_date flag based on whether the developer synced after the
// team's most recently updated skill.
//
// GET /admin/skills/sync-status?team_id=<id>
func adminSkillsSyncStatus(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.URL.Query().Get("team_id")

		// Latest skill updated_at for the team (determines "behind" threshold).
		var latestSkillAt string
		if teamID != "" {
			s.DB().QueryRowContext(r.Context(), //nolint:errcheck
				`SELECT COALESCE(MAX(updated_at), '') FROM skills WHERE team_id = ?`, teamID).
				Scan(&latestSkillAt) //nolint:errcheck
		} else {
			s.DB().QueryRowContext(r.Context(), //nolint:errcheck
				`SELECT COALESCE(MAX(updated_at), '') FROM skills`).
				Scan(&latestSkillAt) //nolint:errcheck
		}

		// Latest sync per (user_email, target) pair — one developer may have multiple machines.
		var (
			q    string
			args []any
		)
		if teamID != "" {
			q = `SELECT user_email, MAX(synced_at), skills_count, target
			       FROM skill_sync_log
			      WHERE team_id = ?
			      GROUP BY user_email, target
			      ORDER BY MAX(synced_at) DESC`
			args = []any{teamID}
		} else {
			q = `SELECT user_email, MAX(synced_at), skills_count, target
			       FROM skill_sync_log
			      GROUP BY user_email, target
			      ORDER BY MAX(synced_at) DESC`
		}

		rows, err := s.DB().QueryContext(r.Context(), q, args...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		entries := make([]syncStatusEntry, 0)
		for rows.Next() {
			var e syncStatusEntry
			if err := rows.Scan(&e.UserEmail, &e.LastSync, &e.SkillsCount, &e.Target); err != nil {
				continue
			}
			// Up to date when: no skills exist yet, OR last sync is >= latest skill update.
			e.IsUpToDate = latestSkillAt == "" || e.LastSync >= latestSkillAt
			entries = append(entries, e)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"entries":         entries,
			"latest_skill_at": latestSkillAt,
			"count":           len(entries),
		})
	}
}
