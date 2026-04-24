package api

// team_skills_sync.go — differential sync endpoint for the CLI skill installer.
//
// GET /team/skills/sync?since=<RFC3339>
//
// Returns only skills that changed after the given timestamp. The CLI stores
// the last synced_at in ~/.korva/sync_state.json and sends it on every pull.
// Skills deleted since the last sync appear with deleted=true so the CLI
// removes the corresponding local file.
//
// Response:
//   {
//     "skills":    [ skillSyncRow, ... ],  // changed or deleted since ?since
//     "synced_at": "2026-04-24T10:00:00Z", // use as next ?since
//     "count":     3
//   }

import (
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// skillSyncRow is the wire type for the sync response.
// Deleted=true means the CLI should remove the local skill file.
type skillSyncRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	Tags      string `json:"tags"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
	Scope     string `json:"scope"`
	UpdatedAt string `json:"updated_at"`
	Deleted   bool   `json:"deleted"`
}

// teamSyncSkills returns skills for the session's team that changed after ?since.
// When ?since is absent or unparseable, all skills are returned (full sync).
//
// Deleted skills are surfaced from audit_logs (action=team_delete_skill) so the
// CLI can clean up local files without needing a soft-delete column.
//
// GET /team/skills/sync?since=<RFC3339>
func teamSyncSkills(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		syncedAt := time.Now().UTC().Format(time.RFC3339)

		// Parse optional ?since filter. Zero value = full sync (no WHERE clause added).
		sinceStr := r.URL.Query().Get("since")
		var since time.Time
		if sinceStr != "" {
			since, _ = time.Parse(time.RFC3339, sinceStr)
		}

		var (
			rows interface {
				Next() bool
				Scan(...any) error
				Close() error
				Err() error
			}
			err error
		)

		if since.IsZero() {
			rows, err = s.DB().QueryContext(r.Context(),
				`SELECT id, name, body, tags, version, updated_by, scope, updated_at
				   FROM skills
				  WHERE team_id = ?
				  ORDER BY updated_at ASC`, sess.teamID)
		} else {
			rows, err = s.DB().QueryContext(r.Context(),
				`SELECT id, name, body, tags, version, updated_by, scope, updated_at
				   FROM skills
				  WHERE team_id = ? AND updated_at > ?
				  ORDER BY updated_at ASC`,
				sess.teamID, since.UTC().Format(time.RFC3339))
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		skills := make([]skillSyncRow, 0)
		for rows.Next() {
			var sk skillSyncRow
			if err := rows.Scan(&sk.ID, &sk.Name, &sk.Body, &sk.Tags,
				&sk.Version, &sk.UpdatedBy, &sk.Scope, &sk.UpdatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			skills = append(skills, sk)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// When doing a differential sync, append deleted-skill stubs from the
		// audit log so the CLI knows which local files to remove.
		if !since.IsZero() {
			delRows, delErr := s.DB().QueryContext(r.Context(),
				`SELECT target, created_at
				   FROM audit_logs
				  WHERE action = 'team_delete_skill'
				    AND created_at > ?
				  ORDER BY created_at ASC`,
				since.UTC().Format(time.RFC3339))
			if delErr == nil {
				defer delRows.Close()
				for delRows.Next() {
					var skillID, deletedAt string
					if delRows.Scan(&skillID, &deletedAt) == nil {
						skills = append(skills, skillSyncRow{
							ID:        skillID,
							UpdatedAt: deletedAt,
							Deleted:   true,
						})
					}
				}
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"skills":    skills,
			"synced_at": syncedAt,
			"count":     len(skills),
		})
	}
}
