package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ─── Session tokens ───────────────────────────────────────────────────────────
//
// Tokens are self-contained HMAC-SHA256 signed payloads — no external JWT lib.
// Format: base64url(payload JSON) + "." + base64url(HMAC-SHA256 signature)

const sessionDuration = 8 * time.Hour

type sessionClaims struct {
	Email string `json:"email"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
	Jti   string `json:"jti"` // random nonce — prevents token reuse if secret rotates
}

// generateSessionToken creates a signed session token for the given email.
func generateSessionToken(email, secret string) (token string, expiresAt time.Time, err error) {
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return
	}
	now := time.Now().UTC()
	expiresAt = now.Add(sessionDuration)

	claims := sessionClaims{
		Email: email,
		Iat:   now.Unix(),
		Exp:   expiresAt.Unix(),
		Jti:   base64.RawURLEncoding.EncodeToString(nonce),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := hmacSign(encoded, secret)
	token = encoded + "." + sig
	return
}

// verifySessionToken validates a session token and returns its claims.
func verifySessionToken(token, secret string) (*sessionClaims, error) {
	dot := lastDot(token)
	if dot < 0 {
		return nil, errors.New("malformed token")
	}
	payload, sig := token[:dot], token[dot+1:]
	expected := hmacSign(payload, secret)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return nil, errors.New("invalid signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("malformed token")
	}
	var claims sessionClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, errors.New("malformed token")
	}
	if time.Now().UTC().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}

func hmacSign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// ─── Rate limiter ─────────────────────────────────────────────────────────────
//
// Per-IP rate limiter: 5 failures triggers a 15-minute lockout.
// Entries are cleaned up lazily to keep memory bounded.

const (
	maxLoginFailures   = 5
	loginLockoutPeriod = 15 * time.Minute
	cleanupInterval    = time.Hour
)

type rateEntry struct {
	failures    int
	lockedUntil time.Time
}

type rateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rateEntry
	lastCleanup time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		entries:     make(map[string]*rateEntry),
		lastCleanup: time.Now(),
	}
}

// Allow returns true if the IP is allowed to attempt a login.
func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maybeCleanup()

	e := rl.entries[ip]
	if e == nil {
		return true
	}
	if time.Now().Before(e.lockedUntil) {
		return false
	}
	return true
}

// RecordFailure increments the failure counter for an IP and locks it if over threshold.
func (rl *rateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e := rl.entries[ip]
	if e == nil {
		e = &rateEntry{}
		rl.entries[ip] = e
	}
	e.failures++
	if e.failures >= maxLoginFailures {
		e.lockedUntil = time.Now().Add(loginLockoutPeriod)
	}
}

// RecordSuccess clears the failure counter for an IP.
func (rl *rateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, ip)
}

// SecondsLocked returns the seconds remaining in a lockout (0 if not locked).
func (rl *rateLimiter) SecondsLocked(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e := rl.entries[ip]
	if e == nil {
		return 0
	}
	remaining := time.Until(e.lockedUntil)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Seconds()) + 1
}

func (rl *rateLimiter) maybeCleanup() {
	if time.Since(rl.lastCleanup) < cleanupInterval {
		return
	}
	now := time.Now()
	for ip, e := range rl.entries {
		if e.lockedUntil.Before(now) && e.failures < maxLoginFailures {
			delete(rl.entries, ip)
		}
	}
	rl.lastCleanup = now
}

// ─── Auth middleware ──────────────────────────────────────────────────────────

// adminAuth validates the Authorization header.
// Accepts either:
//   - Bearer <KORVA_LICENSING_ADMIN_SECRET>  (programmatic / backward-compat)
//   - Bearer <session token>                  (web UI after POST /v1/admin/login)
func (s *server) adminAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		return false
	}
	token := auth[7:]

	// Fast path: raw admin secret (programmatic tools, CI/CD).
	if s.secret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.secret)) == 1 {
		return true
	}

	// Slow path: session token issued by POST /v1/admin/login.
	if s.sessionSecret == "" {
		return false
	}
	claims, err := verifySessionToken(token, s.sessionSecret)
	if err != nil {
		return false
	}
	// Session token must be for the configured admin email.
	return subtle.ConstantTimeCompare([]byte(claims.Email), []byte(s.adminEmail)) == 1
}

// ─── CORS middleware ──────────────────────────────────────────────────────────

func (s *server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.corsOrigin == "*" || (s.corsOrigin != "" && origin == s.corsOrigin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if s.corsOrigin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// clientIP extracts the real client IP from the request.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client), not a proxy.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Strip port.
	ip := r.RemoteAddr
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}
	return ip
}
