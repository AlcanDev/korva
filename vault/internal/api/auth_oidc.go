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

// Phase 17.B context — earlier drafts of this file carried an
// `isHTTPSRequest` helper that looked at `r.TLS` and
// `X-Forwarded-Proto`. That helper existed because we needed to
// decide whether to set `Secure` on the state cookie. Phase 17.A
// removed the cookie entirely (stateless signed state), so the
// helper became dead code AND the original concern about trusting
// `X-Forwarded-Proto` from arbitrary requests went away with it.
// The transport-security boundary now lives at the reverse proxy
// (TLS termination) and the SPA's URL fragment (never sent to the
// server). Nothing in this file inspects the transport scheme
// directly.

// Phase 15.D — OIDC web flow for self-hosted vaults.
// Phase 17.A — stateless signed state (see oidc_state.go).
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
//                              with HMAC-signed state param (no cookie)
//   GET  /auth/oidc/callback → verifies state signature + TTL, exchanges
//                              code, checks email_verified + domain
//                              allowlist, looks up an EXISTING
//                              team_member by email, mints a
//                              member_sessions row, redirects to the
//                              Beacon dashboard with the session token
//                              in the URL fragment.
//
// The team admin must still pre-invite the user (POST /admin/teams/.../
// members) — OIDC only proves "this email controls the IdP account",
// not "this person is in the team". This keeps multi-tenant boundaries
// intact and avoids implicit account provisioning.

// oidcStateTTL bounds the IdP round-trip. Anything longer is
// rejected at the callback step — typical IdP flows take seconds.
const oidcStateTTL = 10 * time.Minute

// oidcLoginHandler starts the OAuth dance.
//
// Steps:
//  1. Mint a self-signed state token (HMAC over nonce + timestamp).
//  2. Redirect the browser to the IdP authorize endpoint.
//
// No cookie is set: the state is fully self-contained, so two tabs
// can start logins concurrently without one overwriting the other.
//
// Failure modes (all 503 with a hint operator can act on):
//   - verifier == nil (config missing at startup)
//   - signer == nil (admin.key not available — guarded at registration)
//   - AuthCodeURL returns "" (discovery failed, lazy init couldn't
//     contact the IdP)
func oidcLoginHandler(_ *OIDCConfig, v OIDCVerifier, signer *signedOIDCState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v == nil || signer == nil {
			writeError(w, http.StatusServiceUnavailable,
				"OIDC is not configured on this vault — see docs/SELF_HOSTING_OIDC.md")
			return
		}
		state, err := signer.Mint()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not mint state token")
			return
		}
		authURL := v.AuthCodeURL(state)
		if authURL == "" {
			writeError(w, http.StatusServiceUnavailable,
				"OIDC discovery failed — check KORVA_OIDC_ISSUER_URL and vault network access")
			return
		}
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// oidcCallbackHandler completes the OAuth dance and mints a member
// session.
//
// Steps:
//  1. Extract `code` + `state` query params.
//  2. Verify the state's HMAC signature + TTL (no cookie needed).
//  3. Exchange code with the IdP, verify id_token.
//  4. Reject if email_verified=false or domain not allowlisted.
//  5. Look up team_members by email; reject if not pre-invited.
//  6. Insert a fresh member_sessions row.
//  7. Redirect to /app with #session=<plain_token> so the SPA can
//     pluck it from window.location.hash.
//
// All failure modes return 4xx with a clear hint so operators can
// debug from logs without leaking IdP internals.
func oidcCallbackHandler(s *store.Store, cfg *OIDCConfig, v OIDCVerifier, signer *signedOIDCState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v == nil || cfg == nil || signer == nil {
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

		// Verify the state HMAC + TTL. A stateless check means
		// concurrent tabs no longer fight over a single cookie value.
		if _, err := signer.Verify(stateParam, oidcStateTTL); err != nil {
			writeError(w, http.StatusBadRequest,
				"invalid state — possible CSRF or expired token, restart the login")
			return
		}

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
