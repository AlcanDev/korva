package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 15.D — OIDC web flow for self-hosted vaults.
//
// Why a second auth path: the invite-token + OTP flow (Phase 11/12)
// covers individual developers, but enterprise deployments want one of
// two things:
//
//   1. SSO via the company IdP (Okta, Azure AD, Google Workspace,
//      Authentik, Keycloak…) so de-provisioning at the IdP cascades to
//      the vault.
//   2. No per-user secrets stored in ~/.korva: the browser carries the
//      session cookie minted from the IdP id_token.
//
// The flow is deliberately minimal:
//
//   GET  /auth/oidc/login    → 302 to IdP authorize endpoint
//                              + sets short-lived korva_oidc_state cookie
//   GET  /auth/oidc/callback → verifies state, exchanges code,
//                              checks email_verified + domain allowlist,
//                              looks up an EXISTING team_member by
//                              email, mints a member_sessions row,
//                              redirects to the Beacon dashboard with
//                              the session token hash-fragmented in.
//
// The team admin must still pre-invite the user (POST /admin/teams/.../
// members) — OIDC only proves "this email controls the IdP account",
// not "this person is in the team". This keeps multi-tenant boundaries
// intact and avoids implicit account provisioning.

const (
	// oidcStateCookie holds the per-request CSRF nonce. HttpOnly +
	// SameSite=Lax so the cookie survives the IdP round-trip but is
	// inaccessible to JS. Short-lived: 10 minutes.
	oidcStateCookie = "korva_oidc_state"
	// oidcStateTTL bounds the IdP round-trip. Anything longer is
	// rejected at the callback step — typical IdP flows take seconds.
	oidcStateTTL = 10 * time.Minute
)

// generateState mints a fresh CSRF nonce. The same value is set as a
// cookie AND sent to the IdP via the `state` param; the callback
// rejects requests where they don't match (classic OAuth CSRF guard).
func generateState() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// oidcLoginHandler starts the OAuth dance.
//
// Steps:
//  1. Mint a state nonce, set as cookie + auth URL param.
//  2. Redirect the browser to the IdP authorize endpoint.
//
// Failure modes (all 503 with a hint operator can act on):
//   - verifier == nil (config missing at startup)
//   - AuthCodeURL returns "" (discovery failed, lazy init couldn't
//     contact the IdP)
func oidcLoginHandler(_ *OIDCConfig, v OIDCVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v == nil {
			writeError(w, http.StatusServiceUnavailable,
				"OIDC is not configured on this vault — see docs/SELF_HOSTING_OIDC.md")
			return
		}
		state, err := generateState()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not generate state")
			return
		}
		authURL := v.AuthCodeURL(state)
		if authURL == "" {
			writeError(w, http.StatusServiceUnavailable,
				"OIDC discovery failed — check KORVA_OIDC_ISSUER_URL and vault network access")
			return
		}
		// Same TTL on cookie + server-side check so a stale tab can't
		// replay a 30-minute-old state.
		http.SetCookie(w, &http.Cookie{
			Name:     oidcStateCookie,
			Value:    state,
			Path:     "/auth/oidc",
			MaxAge:   int(oidcStateTTL.Seconds()),
			HttpOnly: true,
			Secure:   isHTTPSRequest(r),
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// oidcCallbackHandler completes the OAuth dance and mints a member
// session.
//
// Steps:
//  1. Extract `code` + `state` query params + cookie state.
//  2. Reject if any are missing or state mismatches (CSRF guard).
//  3. Exchange code with the IdP, verify id_token.
//  4. Reject if email_verified=false or domain not allowlisted.
//  5. Look up team_members by email; reject if not pre-invited.
//  6. Insert a fresh member_sessions row.
//  7. Redirect to /app with #session=<plain_token> so the SPA can
//     pluck it from window.location.hash.
//
// All failure modes return 4xx with a clear hint so operators can
// debug from logs without leaking IdP internals.
func oidcCallbackHandler(s *store.Store, cfg *OIDCConfig, v OIDCVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v == nil || cfg == nil {
			writeError(w, http.StatusServiceUnavailable,
				"OIDC is not configured on this vault")
			return
		}

		// IdP errors land here when the user denies consent or the
		// authorize step fails. Surface their error verbatim — they're
		// already user-facing strings.
		if e := r.URL.Query().Get("error"); e != "" {
			desc := r.URL.Query().Get("error_description")
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("IdP returned error: %s — %s", e, desc))
			return
		}

		stateParam := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		if stateParam == "" || code == "" {
			writeError(w, http.StatusBadRequest,
				"missing state or code in callback — restart the login")
			return
		}

		stateCookie, err := r.Cookie(oidcStateCookie)
		if err != nil {
			writeError(w, http.StatusBadRequest,
				"missing state cookie — make sure cookies are enabled and restart the login")
			return
		}
		if stateCookie.Value == "" || stateCookie.Value != stateParam {
			writeError(w, http.StatusBadRequest, "state mismatch — possible CSRF, restart the login")
			return
		}

		// State checked; clear the cookie so a stale value can't be
		// replayed.
		http.SetCookie(w, &http.Cookie{
			Name:     oidcStateCookie,
			Value:    "",
			Path:     "/auth/oidc",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   isHTTPSRequest(r),
			SameSite: http.SameSiteLaxMode,
		})

		claims, err := v.ExchangeAndVerify(r.Context(), code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if claims.Email == "" {
			writeError(w, http.StatusBadRequest, "id_token has no email claim — check IdP scope settings")
			return
		}
		if !claims.EmailVerified {
			writeError(w, http.StatusForbidden, "email is not verified at the IdP — contact your admin")
			return
		}
		email := strings.ToLower(strings.TrimSpace(claims.Email))
		if !cfg.EmailDomainAllowed(email) {
			writeError(w, http.StatusForbidden,
				"email domain is not in KORVA_OIDC_ALLOWED_DOMAINS — contact your admin")
			return
		}

		// Look up the team_member by email. We do NOT auto-provision —
		// the admin must have invited the user first.
		var memberID, teamID string
		err = s.DB().QueryRowContext(r.Context(),
			`SELECT id, team_id FROM team_members WHERE lower(email)=? LIMIT 1`,
			email).Scan(&memberID, &teamID)
		if err != nil {
			writeError(w, http.StatusForbidden,
				"no team membership found for this email — ask your admin to invite you first")
			return
		}

		token, hash, err := mintSessionToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session generation failed")
			return
		}

		now := time.Now().UTC()
		expiresAt := now.Add(sessionTTL).Format(time.RFC3339)
		sessionID := newID()

		// Same transactional invariants as authRedeem: revoke any
		// existing session for this (team, email) pair before minting
		// a new one. Prevents stale tokens piling up after repeat
		// logins.
		tx, err := s.DB().BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not begin transaction")
			return
		}
		defer tx.Rollback() //nolint:errcheck
		if _, err = tx.ExecContext(r.Context(),
			`DELETE FROM member_sessions WHERE email=? AND team_id=?`,
			email, teamID); err != nil {
			writeError(w, http.StatusInternalServerError, "session revocation failed")
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
			 VALUES(?,?,?,?,?,?)`,
			sessionID, teamID, memberID, email, hash, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "transaction commit failed")
			return
		}

		// Redirect target: the SPA at /app/overview, with the session
		// token in the URL fragment so it stays on the client and is
		// never logged server-side. The Beacon SPA's bootstrap reads
		// `window.location.hash`, stores the token, and clears the
		// fragment.
		redirectTo := url.URL{
			Path:     "/app/overview",
			Fragment: "session=" + token,
		}
		http.Redirect(w, r, redirectTo.String(), http.StatusFound)
	}
}

// mintSessionToken returns the plaintext token + sha256 hash. Caller
// stores the hash; the plaintext is delivered exactly once (URL
// fragment in the redirect, never logged).
func mintSessionToken() (plain, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(raw)
	hash = fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	return plain, hash, nil
}

// isHTTPSRequest reports whether the incoming request is over HTTPS,
// either directly or via a trusted reverse proxy that set
// X-Forwarded-Proto. Used to decide the Secure cookie flag.
func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if p := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(p, "https") {
		return true
	}
	return false
}
