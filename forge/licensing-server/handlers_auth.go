package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// POST /v1/admin/login
//
// Accepts {"email": "...", "password": "..."} and returns a short-lived session
// token. The password must match KORVA_ADMIN_PASSWORD (falls back to
// KORVA_LICENSING_ADMIN_SECRET if not set). The email must match KORVA_ADMIN_EMAIL.
//
// Rate-limited to 5 failures per IP before a 15-minute lockout.
func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if secs := s.limiter.SecondsLocked(ip); secs > 0 {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts — try again later")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	emailOK := subtle.ConstantTimeCompare([]byte(req.Email), []byte(s.adminEmail)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.adminPassword)) == 1

	if !emailOK || !passOK {
		s.limiter.RecordFailure(ip)
		// Identical error for email and password to prevent enumeration.
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, expiresAt, err := generateSessionToken(req.Email, s.sessionSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate session")
		return
	}

	s.limiter.RecordSuccess(ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"email":      req.Email,
		"expires_at": expiresAt,
	})
}
