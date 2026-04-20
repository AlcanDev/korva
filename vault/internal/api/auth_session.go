package api

import (
	"context"
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
// team, role, features available, session expiry. Used by the CLI on every `korva` run.
// Requires X-Session-Token header.
func authMe(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := requireSession(s, w, r)
		if !ok {
			return
		}

		// Touch last_seen
		s.DB().ExecContext(r.Context(), //nolint:errcheck
			`UPDATE member_sessions SET last_seen=datetime('now') WHERE id=?`, sess.id)

		// Fetch team name (role is already in sess via requireSession JOIN)
		var teamName string
		s.DB().QueryRowContext(r.Context(), //nolint:errcheck
			`SELECT name FROM teams WHERE id=?`, sess.teamID).Scan(&teamName)

		writeJSON(w, http.StatusOK, map[string]any{
			"email":      sess.email,
			"team_id":    sess.teamID,
			"team":       teamName,
			"role":       sess.role,
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
	role      string // "admin" or "member"
	expiresAt string
}

// requireSession extracts and validates X-Session-Token. Returns false and writes
// an error response when the session is missing, invalid, or expired.
// The member's role is fetched in the same query via a LEFT JOIN on team_members.
func requireSession(s *store.Store, w http.ResponseWriter, r *http.Request) (sessionInfo, bool) {
	plain := r.Header.Get("X-Session-Token")
	if plain == "" {
		writeError(w, http.StatusUnauthorized, "X-Session-Token header required")
		return sessionInfo{}, false
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	var sess sessionInfo
	err := s.DB().QueryRowContext(r.Context(),
		`SELECT ms.id, ms.team_id, ms.email, ms.expires_at,
		        COALESCE(tm.role, 'member')
		   FROM member_sessions ms
		   LEFT JOIN team_members tm
		          ON tm.team_id = ms.team_id AND tm.email = ms.email
		  WHERE ms.token_hash = ?`, hash).
		Scan(&sess.id, &sess.teamID, &sess.email, &sess.expiresAt, &sess.role)
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

// ── Session context injection ─────────────────────────────────────────────────

// sessionCtxKey is the unexported context key for injected sessionInfo.
type sessionCtxKey struct{}

// withSession returns middleware that validates X-Session-Token and injects
// the resulting sessionInfo into the request context. The handler is only
// called when authentication succeeds.
func withSession(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := requireSession(s, w, r)
			if !ok {
				return
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// sessionFromCtx retrieves the sessionInfo injected by withSession.
// Panics if called from a handler that is not behind withSession.
func sessionFromCtx(r *http.Request) sessionInfo {
	return r.Context().Value(sessionCtxKey{}).(sessionInfo)
}

// requireAdmin writes 403 and returns false when the session role is not "admin".
// Use for write/delete operations that team members should not perform.
func requireAdmin(sess sessionInfo, w http.ResponseWriter) bool {
	if sess.role != "admin" {
		writeError(w, http.StatusForbidden, "team admin role required")
		return false
	}
	return true
}
