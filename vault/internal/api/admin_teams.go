package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/store"
)

type teamRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	LicenseID string `json:"license_id"`
	CreatedAt string `json:"created_at"`
}

type memberRow struct {
	ID        string `json:"id"`
	TeamID    string `json:"team_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

func adminListTeams(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, name, owner, license_id, created_at FROM teams ORDER BY created_at DESC`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		var teams []teamRow
		for rows.Next() {
			var t teamRow
			if err := rows.Scan(&t.ID, &t.Name, &t.Owner, &t.LicenseID, &t.CreatedAt); err != nil {
				continue
			}
			teams = append(teams, t)
		}
		if teams == nil {
			teams = []teamRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
	}
}

func adminCreateTeam(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name      string `json:"name"`
			Owner     string `json:"owner"`
			LicenseID string `json:"license_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		id := newID()
		_, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO teams(id, name, owner, license_id) VALUES(?,?,?,?)`,
			id, body.Name, body.Owner, body.LicenseID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeAudit(s, actor, "create_team", id, "", hashStr(body.Name))
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func adminListMembers(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, team_id, email, role, created_at FROM team_members WHERE team_id=? ORDER BY created_at`,
			teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		var members []memberRow
		for rows.Next() {
			var m memberRow
			if err := rows.Scan(&m.ID, &m.TeamID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
				continue
			}
			members = append(members, m)
		}
		if members == nil {
			members = []memberRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
	}
}

func adminAddMember(s *store.Store, actor string, lic *license.License) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if body.Role == "" {
			body.Role = "member"
		}

		// Seat limit enforcement: count existing members for this team
		if lic != nil && lic.Seats > 0 {
			var count int
			if err := s.DB().QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM team_members WHERE team_id=?`, teamID).Scan(&count); err == nil {
				if count >= lic.Seats {
					writeError(w, http.StatusPaymentRequired,
						fmt.Sprintf("seat limit reached (%d/%d) — upgrade your plan at korva.dev/pricing", count, lic.Seats))
					return
				}
			}
		}

		id := newID()
		_, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
			id, teamID, body.Email, body.Role)
		if err != nil {
			writeError(w, http.StatusConflict, "member already exists or DB error: "+err.Error())
			return
		}
		writeAudit(s, actor, "add_member", id, "", hashStr(body.Email))
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func adminRemoveMember(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID := r.PathValue("member_id")
		// capture email for audit before delete
		var email string
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT email FROM team_members WHERE id=?`, memberID).Scan(&email); err != nil {
			log.Printf("admin: pre-delete member email lookup: %v", err)
		}
		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM team_members WHERE id=?`, memberID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}
		writeAudit(s, actor, "remove_member", memberID, hashStr(email), "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	}
}

// adminActiveProfile returns the active team profile from the DB (teams table).
// Returns 204 No Content when no team is configured — callers fall back to Git.
func adminActiveProfile(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var t teamRow
		err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, name, owner, license_id, created_at FROM teams ORDER BY created_at DESC LIMIT 1`).
			Scan(&t.ID, &t.Name, &t.Owner, &t.LicenseID, &t.CreatedAt)
		if err != nil {
			// No team configured — tell callers to fall back to Git
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Gather members
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, team_id, email, role, created_at FROM team_members WHERE team_id=?`, t.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		var members []memberRow
		for rows.Next() {
			var m memberRow
			if err := rows.Scan(&m.ID, &m.TeamID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
				continue
			}
			members = append(members, m)
		}
		if members == nil {
			members = []memberRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"team": t, "members": members})
	}
}

// newID generates a time-ordered hex ID.
func newID() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(time.Now().String())))[:16]
}
