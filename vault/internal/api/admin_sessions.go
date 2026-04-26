package api

import (
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

type memberSessionRow struct {
	ID        string `json:"id"`
	MemberID  string `json:"member_id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	LastSeen  string `json:"last_seen"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status"`
}

// adminListTeamSessions returns all active (non-expired) sessions for a team.
func adminListTeamSessions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, member_id, email, created_at, last_seen, expires_at
			   FROM member_sessions
			  WHERE team_id=?
			  ORDER BY last_seen DESC`, teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		now := time.Now().UTC().Format(time.RFC3339)
		var sessions []memberSessionRow
		for rows.Next() {
			var sess memberSessionRow
			if err := rows.Scan(&sess.ID, &sess.MemberID, &sess.Email,
				&sess.CreatedAt, &sess.LastSeen, &sess.ExpiresAt); err != nil {
				continue
			}
			if sess.ExpiresAt < now {
				sess.Status = "expired"
			} else {
				sess.Status = "active"
			}
			sessions = append(sessions, sess)
		}
		if sessions == nil {
			sessions = []memberSessionRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "count": len(sessions)})
	}
}

// adminRevokeSession force-logs out a member by deleting their session.
func adminRevokeSession(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		sessionID := r.PathValue("session_id")

		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM member_sessions WHERE id=? AND team_id=?`, sessionID, teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeAudit(s, actor, "revoke_session", sessionID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	}
}
