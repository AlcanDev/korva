package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/store"
)

const inviteTTL = 7 * 24 * time.Hour

// adminCreateInvite generates a one-time invite token for a member email.
// The plaintext token is returned exactly once — it is never stored.
// When a Mailer is configured, an invite email is dispatched automatically.
// Email failure is non-fatal: the token is always returned in the response.
//
// Body: {"email": "alice@corp.com"}
func adminCreateInvite(s *store.Store, actor string, mailer email.Mailer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		// Verify the email is actually a member of this team.
		var memberID, teamName string
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT m.id, t.name
			   FROM team_members m
			   JOIN teams t ON t.id = m.team_id
			  WHERE m.team_id = ? AND m.email = ?`,
			teamID, body.Email).Scan(&memberID, &teamName); err != nil {
			writeError(w, http.StatusNotFound, "email is not a member of this team — add them first")
			return
		}

		// Revoke any existing pending invite for this email+team.
		s.DB().ExecContext(r.Context(), //nolint:errcheck
			`DELETE FROM member_invites WHERE team_id = ? AND email = ? AND used_at IS NULL`,
			teamID, body.Email)

		// Generate plaintext token (32 random bytes = 64 hex chars).
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			writeError(w, http.StatusInternalServerError, "token generation failed")
			return
		}
		plaintext := hex.EncodeToString(raw)
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plaintext)))

		id := newID()
		expiresAt := time.Now().UTC().Add(inviteTTL).Format(time.RFC3339)
		if _, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO member_invites(id, team_id, email, token_hash, expires_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, teamID, body.Email, hash, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeAudit(s, actor, "create_invite", id, "", hashStr(body.Email))

		// Best-effort email dispatch — never blocks the HTTP response.
		emailSent := false
		if mailer.Configured() {
			msg := email.InviteMessage(body.Email, teamName, plaintext, expiresAt)
			if err := mailer.Send(r.Context(), msg); err != nil {
				log.Printf("invite email failed for %s: %v", body.Email, err)
			} else {
				emailSent = true
			}
		}

		resp := map[string]any{
			"id":         id,
			"token":      plaintext, // shown exactly once — never stored in clear
			"email":      body.Email,
			"expires_at": expiresAt,
			"email_sent": emailSent,
		}
		if !emailSent {
			resp["note"] = "share this token with the member — it will not be shown again"
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// adminListInvites returns all invites for a team (pending, used and expired).
func adminListInvites(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.PathValue("team_id")
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, email, expires_at, used_at
			   FROM member_invites
			  WHERE team_id = ?
			  ORDER BY created_at DESC`, teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		type inviteRow struct {
			ID        string  `json:"id"`
			Email     string  `json:"email"`
			ExpiresAt string  `json:"expires_at"`
			UsedAt    *string `json:"used_at,omitempty"`
			Status    string  `json:"status"`
		}
		var invites []inviteRow
		for rows.Next() {
			var inv inviteRow
			if err := rows.Scan(&inv.ID, &inv.Email, &inv.ExpiresAt, &inv.UsedAt); err != nil {
				continue
			}
			switch {
			case inv.UsedAt != nil:
				inv.Status = "used"
			case inv.ExpiresAt < time.Now().UTC().Format(time.RFC3339):
				inv.Status = "expired"
			default:
				inv.Status = "pending"
			}
			invites = append(invites, inv)
		}
		if invites == nil {
			invites = []inviteRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"invites": invites, "count": len(invites)})
	}
}

// adminRevokeInvite deletes a pending invite before it is redeemed.
func adminRevokeInvite(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inviteID := r.PathValue("invite_id")
		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM member_invites WHERE id = ? AND used_at IS NULL`, inviteID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "invite not found or already used")
			return
		}
		writeAudit(s, actor, "revoke_invite", inviteID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	}
}
