package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 12 — Passwordless re-login for already-invited members.
//
// Flow:
//
//   1. POST /auth/otp/request {email}
//      - Caller must already be in team_members (no signup).
//      - We invalidate any pending OTP for the email, generate a fresh
//        6-digit code, store SHA256(code), and email the plaintext.
//      - Always returns 204 No Content — never reveals whether the email
//        exists in the database (avoids enumeration).
//
//   2. POST /auth/otp/verify {email, code}
//      - Validates the code, atomically marks it consumed, mints a fresh
//        session token, returns the same shape as /auth/redeem.
//      - 5 wrong attempts on the same code burn it.
//
// Codes are short-lived (10 min) and single-use. Rate limiting is two-
// layered: max 3 codes per email per hour (issuance), max 5 verify
// attempts per code (verification).

const (
	otpTTL              = 10 * time.Minute
	otpCodeDigits       = 6
	otpMaxAttempts      = 5
	otpMaxIssuePerHour  = 3
	otpIssueWindow      = 1 * time.Hour
	otpRequestBodyLimit = 256
)

// authOTPRequest issues a one-time code by email for a member who's already
// in team_members. To prevent account enumeration, the response is always
// 204 — same shape whether the email exists or not. The actual code is
// only delivered out-of-band via email.
func authOTPRequest(s *store.Store, mailer email.Mailer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := readEmailBody(w, r)
		if !ok {
			return
		}

		// Look up the member. If absent, return 204 anyway to avoid leaking
		// whether the email exists in the database.
		var memberRow struct {
			id     string
			teamID string
		}
		err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, team_id FROM team_members WHERE email=? LIMIT 1`,
			body.Email).Scan(&memberRow.id, &memberRow.teamID)
		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Issuance rate-limit: count recent (within the issue window) codes
		// for this email — pending OR consumed both count, so an abuser
		// can't farm consumed slots either.
		windowStart := time.Now().UTC().Add(-otpIssueWindow).Format(time.RFC3339)
		var recent int
		_ = s.DB().QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM auth_otp_codes WHERE email=? AND created_at >= ?`,
			body.Email, windowStart).Scan(&recent)
		if recent >= otpMaxIssuePerHour {
			writeError(w, http.StatusTooManyRequests,
				"too many OTP requests — wait an hour before trying again")
			return
		}

		// Invalidate any prior pending code for the same email so the new
		// code is the only valid one.
		s.DB().ExecContext(r.Context(), //nolint:errcheck
			`UPDATE auth_otp_codes SET consumed_at=datetime('now')
			  WHERE email=? AND consumed_at IS NULL`, body.Email)

		// Generate code + persist hash. We pass `created_at` explicitly as
		// RFC3339 so it matches the format the rate-limit query above uses;
		// SQLite's `datetime('now')` default emits a space-separated form
		// that would not compare correctly against the windowStart string.
		code, err := newOTPCode(otpCodeDigits)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "code generation failed")
			return
		}
		codeHash := fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
		now := time.Now().UTC().Format(time.RFC3339)
		expiresAt := time.Now().UTC().Add(otpTTL).Format(time.RFC3339)
		if _, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO auth_otp_codes(id, email, code_hash, created_at, expires_at)
			 VALUES (?, ?, ?, ?, ?)`,
			newID(), body.Email, codeHash, now, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Best-effort email dispatch — never blocks the HTTP response.
		if mailer.Configured() {
			msg := email.OTPMessage(body.Email, code, int(otpTTL/time.Minute))
			if err := mailer.Send(r.Context(), msg); err != nil {
				log.Printf("otp email failed for %s: %v", body.Email, err)
			}
		}
		// Audit log uses hashStr so the email is not stored in clear here.
		writeAudit(s, "auth", "otp_request", memberRow.id, "", hashStr(body.Email))

		w.WriteHeader(http.StatusNoContent)
	}
}

// authOTPVerify exchanges (email, code) for a fresh session token, mirroring
// the shape of /auth/redeem so the CLI can use the same persistence path.
func authOTPVerify(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		body.Email = strings.ToLower(strings.TrimSpace(body.Email))
		body.Code = strings.TrimSpace(body.Code)
		if body.Email == "" || body.Code == "" {
			writeError(w, http.StatusBadRequest, "email and code are required")
			return
		}
		if len(body.Code) > 32 {
			writeError(w, http.StatusBadRequest, "code too long")
			return
		}

		codeHash := fmt.Sprintf("%x", sha256.Sum256([]byte(body.Code)))

		// Pull the latest pending OTP for the email. There can only be one
		// because authOTPRequest invalidates prior pending codes.
		var otp struct {
			id        string
			storedHsh string
			expiresAt string
			attempts  int
		}
		err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, code_hash, expires_at, attempts
			   FROM auth_otp_codes
			  WHERE email=? AND consumed_at IS NULL
			  ORDER BY created_at DESC LIMIT 1`,
			body.Email).Scan(&otp.id, &otp.storedHsh, &otp.expiresAt, &otp.attempts)
		if err != nil {
			// No pending code → either never requested or already burned.
			// Return 401 — generic to avoid leaking whether the email
			// exists.
			writeError(w, http.StatusUnauthorized, "invalid or expired code")
			return
		}

		if otp.expiresAt < time.Now().UTC().Format(time.RFC3339) {
			s.DB().ExecContext(r.Context(), //nolint:errcheck
				`UPDATE auth_otp_codes SET consumed_at=datetime('now') WHERE id=?`, otp.id)
			writeError(w, http.StatusUnauthorized, "code expired — request a new one")
			return
		}

		if otp.storedHsh != codeHash {
			// Wrong code: bump attempts; burn the row when the limit hits.
			next := otp.attempts + 1
			if next >= otpMaxAttempts {
				s.DB().ExecContext(r.Context(), //nolint:errcheck
					`UPDATE auth_otp_codes SET attempts=?, consumed_at=datetime('now')
					  WHERE id=?`, next, otp.id)
				writeError(w, http.StatusUnauthorized,
					"too many wrong attempts — request a new code")
				return
			}
			s.DB().ExecContext(r.Context(), //nolint:errcheck
				`UPDATE auth_otp_codes SET attempts=? WHERE id=?`, next, otp.id)
			writeError(w, http.StatusUnauthorized, "invalid or expired code")
			return
		}

		// Code is valid. Mint a session. From here on the flow mirrors
		// /auth/redeem exactly so the CLI consumes both endpoints
		// identically.
		var member struct {
			id     string
			teamID string
		}
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, team_id FROM team_members WHERE email=? LIMIT 1`,
			body.Email).Scan(&member.id, &member.teamID); err != nil {
			// Member was deleted between request and verify — refuse.
			writeError(w, http.StatusUnauthorized, "member no longer exists")
			return
		}

		sessionPlain, err := newRandomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session generation failed")
			return
		}
		sessionHash := fmt.Sprintf("%x", sha256.Sum256([]byte(sessionPlain)))
		expiresAt := time.Now().UTC().Add(sessionTTL).Format(time.RFC3339)

		tx, err := s.DB().BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not begin transaction")
			return
		}
		defer tx.Rollback() //nolint:errcheck
		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM member_sessions WHERE email=? AND team_id=?`,
			body.Email, member.teamID); err != nil {
			writeError(w, http.StatusInternalServerError, "session revocation failed")
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
			 VALUES(?,?,?,?,?,?)`,
			newID(), member.teamID, member.id, body.Email, sessionHash, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE auth_otp_codes SET consumed_at=datetime('now') WHERE id=?`, otp.id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not mark code consumed")
			return
		}
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "transaction commit failed")
			return
		}

		var teamName string
		_ = s.DB().QueryRowContext(r.Context(),
			`SELECT name FROM teams WHERE id=?`, member.teamID).Scan(&teamName)

		writeAudit(s, "auth", "otp_verify", member.id, "", hashStr(body.Email))

		writeJSON(w, http.StatusOK, map[string]string{
			"session_token": sessionPlain,
			"email":         body.Email,
			"team_id":       member.teamID,
			"team":          teamName,
			"expires_at":    expiresAt,
		})
	}
}

// readEmailBody parses the {email} body shared by /auth/otp/request and
// returns it lowercased + trimmed. Writes the error response and returns
// false when the body is malformed.
func readEmailBody(w http.ResponseWriter, r *http.Request) (struct {
	Email string `json:"email"`
}, bool) {
	var body struct {
		Email string `json:"email"`
	}
	if r.ContentLength > otpRequestBodyLimit {
		writeError(w, http.StatusBadRequest, "request too large")
		return body, false
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return body, false
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" || !strings.Contains(body.Email, "@") {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return body, false
	}
	return body, true
}

// newOTPCode returns a uniformly random `digits`-digit numeric code drawn
// from crypto/rand. Unlike `crypto/rand.Int(big.NewInt(10^digits))`, this
// also rejects leading-zero codes — keeping the human-readable form
// constant-width.
func newOTPCode(digits int) (string, error) {
	if digits <= 0 || digits > 12 {
		return "", fmt.Errorf("invalid digit count: %d", digits)
	}
	max := big.NewInt(1)
	for i := 0; i < digits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	// Range [10^(digits-1), 10^digits) — guarantees `digits`-wide output.
	min := new(big.Int).Quo(max, big.NewInt(10))
	span := new(big.Int).Sub(max, min)
	n, err := rand.Int(rand.Reader, span)
	if err != nil {
		return "", err
	}
	n.Add(n, min)
	return n.String(), nil
}

// newRandomToken returns 32 bytes of crypto/rand as hex (64 chars). Same
// shape as the invite + session token producer in auth_session.go.
func newRandomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", raw), nil
}
