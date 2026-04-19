package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

const sessionTTL = 30 * 24 * time.Hour

// authRedeem exchanges a one-time invite token for a long-lived session token.
// This is the only public endpoint — no admin key required.
// Body: {"token": "<plaintext-invite-token>"}
func authRedeem(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}

		tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(body.Token)))

		// Look up the invite
		var invite struct {
			id       string
			teamID   string
			email    string
			usedAt   *string
			expiresAt string
		}
		err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, team_id, email, used_at, expires_at
			   FROM member_invites WHERE token_hash=?`, tokenHash).
			Scan(&invite.id, &invite.teamID, &invite.email, &invite.usedAt, &invite.expiresAt)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired invite token")
			return
		}
		if invite.usedAt != nil {
			writeError(w, http.StatusGone, "invite already used — ask your admin for a new one")
			return
		}
		if invite.expiresAt < time.Now().UTC().Format(time.RFC3339) {
			writeError(w, http.StatusGone, "invite expired — ask your admin for a new one")
			return
		}

		// Look up the member record
		var memberID string
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT id FROM team_members WHERE team_id=? AND email=?`,
			invite.teamID, invite.email).Scan(&memberID); err != nil {
			writeError(w, http.StatusUnauthorized, "member not found — contact your admin")
			return
		}

		// Generate session token
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			writeError(w, http.StatusInternalServerError, "session generation failed")
			return
		}
		sessionPlain := hex.EncodeToString(raw)
		sessionHash := fmt.Sprintf("%x", sha256.Sum256([]byte(sessionPlain)))

		now := time.Now().UTC()
		expiresAt := now.Add(sessionTTL).Format(time.RFC3339)
		sessionID := newID()

		// Single session: revoke any existing session for this member
		s.DB().ExecContext(r.Context(),
			`DELETE FROM member_sessions WHERE email=? AND team_id=?`,
			invite.email, invite.teamID)

		_, err = s.DB().ExecContext(r.Context(),
			`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
			 VALUES(?,?,?,?,?,?)`,
			sessionID, invite.teamID, memberID, invite.email, sessionHash, expiresAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Mark invite as used
		usedNow := now.Format(time.RFC3339)
		s.DB().ExecContext(r.Context(),
			`UPDATE member_invites SET used_at=? WHERE id=?`, usedNow, invite.id)

		// Fetch team name for the response
		var teamName string
		s.DB().QueryRowContext(r.Context(),
			`SELECT name FROM teams WHERE id=?`, invite.teamID).Scan(&teamName)

		writeJSON(w, http.StatusOK, map[string]string{
			"session_token": sessionPlain, // stored at ~/.korva/session.token by CLI
			"email":         invite.email,
			"team_id":       invite.teamID,
			"team":          teamName,
			"expires_at":    expiresAt,
		})
	}
}

// authMe validates the session token and returns the member's current context:
// team, features available, session expiry. Used by the CLI on every `korva` run.
// Requires X-Session-Token header.
func authMe(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := requireSession(s, w, r)
		if !ok {
			return
		}

		// Touch last_seen
		s.DB().ExecContext(r.Context(),
			`UPDATE member_sessions SET last_seen=datetime('now') WHERE id=?`, sess.id)

		// Fetch team + member role
		var teamName, role string
		s.DB().QueryRowContext(r.Context(),
			`SELECT t.name, m.role FROM teams t
			  JOIN team_members m ON m.team_id=t.id AND m.email=?
			 WHERE t.id=?`, sess.email, sess.teamID).Scan(&teamName, &role)

		writeJSON(w, http.StatusOK, map[string]any{
			"email":      sess.email,
			"team_id":    sess.teamID,
			"team":       teamName,
			"role":       role,
			"expires_at": sess.expiresAt,
		})
	}
}

// authLogout revokes the current session.
// Requires X-Session-Token header.
func authLogout(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := requireSession(s, w, r)
		if !ok {
			return
		}
		s.DB().ExecContext(r.Context(),
			`DELETE FROM member_sessions WHERE id=?`, sess.id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
	}
}

// sessionInfo holds the data needed for authenticated handlers.
type sessionInfo struct {
	id        string
	teamID    string
	email     string
	expiresAt string
}

// requireSession extracts and validates X-Session-Token. Returns false and writes
// an error response when the session is missing, invalid, or expired.
func requireSession(s *store.Store, w http.ResponseWriter, r *http.Request) (sessionInfo, bool) {
	plain := r.Header.Get("X-Session-Token")
	if plain == "" {
		writeError(w, http.StatusUnauthorized, "X-Session-Token header required")
		return sessionInfo{}, false
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	var sess sessionInfo
	err := s.DB().QueryRowContext(r.Context(),
		`SELECT id, team_id, email, expires_at
		   FROM member_sessions WHERE token_hash=?`, hash).
		Scan(&sess.id, &sess.teamID, &sess.email, &sess.expiresAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session token")
		return sessionInfo{}, false
	}
	if sess.expiresAt < time.Now().UTC().Format(time.RFC3339) {
		writeError(w, http.StatusUnauthorized, "session expired — run 'korva auth <invite-token>'")
		return sessionInfo{}, false
	}
	return sess, true
}
