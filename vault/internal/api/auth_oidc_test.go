package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// fakeOIDCVerifier is a hand-crafted OIDCVerifier we plug into the
// handler tests. It captures the state passed to AuthCodeURL so we can
// re-use it on the callback request, and lets each test customise the
// claims returned from ExchangeAndVerify.
type fakeOIDCVerifier struct {
	authURLBase string // base authorize URL the test asserts on
	lastState   string

	// Returned by ExchangeAndVerify — either the claims or an error.
	claims   *OIDCClaims
	exchErr  error
	exchCode string // populated by the call so tests can assert
}

func (f *fakeOIDCVerifier) AuthCodeURL(state string) string {
	f.lastState = state
	return f.authURLBase + "&state=" + state
}

func (f *fakeOIDCVerifier) ExchangeAndVerify(_ context.Context, code string) (*OIDCClaims, error) {
	f.exchCode = code
	if f.exchErr != nil {
		return nil, f.exchErr
	}
	return f.claims, nil
}

// oidcTestEnv wires an in-memory store seeded with one team + one
// pre-invited member + a fake verifier.
type oidcTestEnv struct {
	store    *store.Store
	verifier *fakeOIDCVerifier
	cfg      *OIDCConfig
	teamID   string
	email    string
}

func newOIDCTestEnv(t *testing.T) *oidcTestEnv {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	db := s.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	teamID := "team-oidc-001"
	if _, err := db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		teamID, "OIDC Co", "owner@oidc.co", now); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		"member-oidc-001", teamID, "alice@oidc.co", "member", now); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return &oidcTestEnv{
		store:    s,
		verifier: &fakeOIDCVerifier{authURLBase: "https://idp.example.com/authorize?client_id=k"},
		cfg: &OIDCConfig{
			IssuerURL:    "https://idp.example.com",
			ClientID:     "k",
			ClientSecret: "s",
			RedirectURL:  "https://vault.example.com/auth/oidc/callback",
			Scopes:       []string{"openid", "email", "profile"},
		},
		teamID: teamID,
		email:  "alice@oidc.co",
	}
}

// runLogin issues a GET /auth/oidc/login through the handler and
// returns the resulting state cookie value + the Location header.
func (e *oidcTestEnv) runLogin(t *testing.T) (stateCookie, location string) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	oidcLoginHandler(e.cfg, e.verifier).ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login: want 302, got %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == oidcStateCookie {
			stateCookie = c.Value
		}
	}
	location = resp.Header.Get("Location")
	return
}

func TestLoadOIDCConfigFromEnv(t *testing.T) {
	t.Run("returns nil when any required var is missing", func(t *testing.T) {
		t.Setenv("KORVA_OIDC_ISSUER_URL", "")
		t.Setenv("KORVA_OIDC_CLIENT_ID", "k")
		t.Setenv("KORVA_OIDC_CLIENT_SECRET", "s")
		t.Setenv("KORVA_OIDC_REDIRECT_URL", "https://x/c")
		if got := LoadOIDCConfigFromEnv(); got != nil {
			t.Errorf("missing issuer: want nil, got %+v", got)
		}
	})
	t.Run("trims trailing slash from issuer and applies default scopes", func(t *testing.T) {
		t.Setenv("KORVA_OIDC_ISSUER_URL", "https://idp.example.com/")
		t.Setenv("KORVA_OIDC_CLIENT_ID", "client")
		t.Setenv("KORVA_OIDC_CLIENT_SECRET", "secret")
		t.Setenv("KORVA_OIDC_REDIRECT_URL", "https://vault.example.com/auth/oidc/callback")
		t.Setenv("KORVA_OIDC_ALLOWED_DOMAINS", "")
		t.Setenv("KORVA_OIDC_SCOPES", "")
		got := LoadOIDCConfigFromEnv()
		if got == nil {
			t.Fatal("want non-nil config")
		}
		if got.IssuerURL != "https://idp.example.com" {
			t.Errorf("issuer not trimmed: %q", got.IssuerURL)
		}
		if len(got.Scopes) != 3 || got.Scopes[0] != "openid" {
			t.Errorf("default scopes: %v", got.Scopes)
		}
		if len(got.AllowedDomains) != 0 {
			t.Errorf("allowed domains: %v", got.AllowedDomains)
		}
	})
	t.Run("parses allowed domains and custom scopes", func(t *testing.T) {
		t.Setenv("KORVA_OIDC_ISSUER_URL", "https://idp.example.com")
		t.Setenv("KORVA_OIDC_CLIENT_ID", "client")
		t.Setenv("KORVA_OIDC_CLIENT_SECRET", "secret")
		t.Setenv("KORVA_OIDC_REDIRECT_URL", "https://vault.example.com/auth/oidc/callback")
		t.Setenv("KORVA_OIDC_ALLOWED_DOMAINS", "Acme.IO, @partner.dev , ")
		t.Setenv("KORVA_OIDC_SCOPES", "openid, email,  groups")
		got := LoadOIDCConfigFromEnv()
		if got == nil {
			t.Fatal("want non-nil config")
		}
		wantDomains := []string{"acme.io", "partner.dev"}
		if len(got.AllowedDomains) != len(wantDomains) {
			t.Fatalf("allowed domains len: %v", got.AllowedDomains)
		}
		for i, d := range wantDomains {
			if got.AllowedDomains[i] != d {
				t.Errorf("domain[%d]: want %q got %q", i, d, got.AllowedDomains[i])
			}
		}
		wantScopes := []string{"openid", "email", "groups"}
		for i, s := range wantScopes {
			if got.Scopes[i] != s {
				t.Errorf("scope[%d]: want %q got %q", i, s, got.Scopes[i])
			}
		}
	})
}

func TestEmailDomainAllowed(t *testing.T) {
	cases := []struct {
		name    string
		domains []string
		email   string
		want    bool
	}{
		{"no allowlist permits everything", nil, "anyone@anywhere.io", true},
		{"empty allowlist permits everything", []string{}, "anyone@anywhere.io", true},
		{"allowed exact match", []string{"acme.io"}, "alice@acme.io", true},
		{"allowed case-insensitive", []string{"acme.io"}, "Alice@ACME.IO", true},
		{"rejected non-listed", []string{"acme.io"}, "alice@evil.org", false},
		{"rejected sub-domain match (no implicit wildcard)", []string{"acme.io"}, "alice@evil.acme.io", false},
		{"multiple domains — second match wins", []string{"acme.io", "partner.dev"}, "bob@partner.dev", true},
		{"malformed email rejected", []string{"acme.io"}, "no-at-sign", false},
		{"trailing @ rejected", []string{"acme.io"}, "alice@", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &OIDCConfig{AllowedDomains: tc.domains}
			if got := c.EmailDomainAllowed(tc.email); got != tc.want {
				t.Errorf("EmailDomainAllowed(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}
}

func TestGenerateStateIsUniqueAndHex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s, err := generateState()
		if err != nil {
			t.Fatalf("generateState: %v", err)
		}
		if len(s) != 48 { // 24 bytes hex-encoded
			t.Fatalf("state len: %d", len(s))
		}
		if seen[s] {
			t.Fatalf("collision after %d iters", i)
		}
		seen[s] = true
	}
}

func TestOIDCLoginRedirectsAndSetsStateCookie(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, location := env.runLogin(t)
	if stateCookie == "" {
		t.Fatal("state cookie missing")
	}
	if env.verifier.lastState != stateCookie {
		t.Errorf("cookie %q != verifier state %q", stateCookie, env.verifier.lastState)
	}
	wantPrefix := env.verifier.authURLBase
	if !strings.HasPrefix(location, wantPrefix) {
		t.Errorf("Location %q has no expected prefix %q", location, wantPrefix)
	}
	if !strings.Contains(location, "state="+stateCookie) {
		t.Errorf("Location %q missing state param", location)
	}
}

func TestOIDCLoginRefusesWhenVerifierMissing(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	oidcLoginHandler(&OIDCConfig{}, nil).ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when verifier nil, got %d", w.Code)
	}
}

func TestOIDCLoginRefusesWhenAuthURLEmpty(t *testing.T) {
	// Models the lazy-init failure path: the verifier's underlying
	// oidc.Provider couldn't reach the IdP discovery endpoint, so
	// AuthCodeURL returns "".
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	oidcLoginHandler(&OIDCConfig{}, authURLEmptyVerifier{}).ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when AuthCodeURL empty, got %d", w.Code)
	}
}

// authURLEmptyVerifier always returns empty from AuthCodeURL. It models
// the "lazy init failed to reach the IdP discovery endpoint" failure
// mode that the lazyOIDCVerifier returns in production.
type authURLEmptyVerifier struct{}

func (authURLEmptyVerifier) AuthCodeURL(string) string { return "" }
func (authURLEmptyVerifier) ExchangeAndVerify(_ context.Context, _ string) (*OIDCClaims, error) {
	return nil, errors.New("not reachable in this test")
}

func TestOIDCCallbackHappyPathMintsSession(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{
		Subject:       "auth0|abc",
		Email:         env.email,
		EmailVerified: true,
		Name:          "Alice",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=auth-code-1&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("want 302, got %d body=%s", resp.StatusCode, w.Body.String())
	}
	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("location parse: %v", err)
	}
	if u.Path != "/app/overview" {
		t.Errorf("want path /app/overview, got %q", u.Path)
	}
	if !strings.HasPrefix(u.Fragment, "session=") {
		t.Errorf("Location.Fragment missing session=: %q", u.Fragment)
	}
	token := strings.TrimPrefix(u.Fragment, "session=")
	if len(token) != 64 { // 32 bytes hex
		t.Errorf("session token len = %d, want 64", len(token))
	}

	// One session row exists for the member.
	var n int
	if err := env.store.DB().QueryRow(
		`SELECT COUNT(*) FROM member_sessions WHERE team_id=? AND email=?`,
		env.teamID, env.email).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 session row, got %d", n)
	}

	// Re-logging in should rotate the session (delete-then-insert).
	stateCookie2, _ := env.runLogin(t)
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=auth-code-2&state="+stateCookie2, nil)
	r2.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie2})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w2, r2)
	if err := env.store.DB().QueryRow(
		`SELECT COUNT(*) FROM member_sessions WHERE team_id=? AND email=?`,
		env.teamID, env.email).Scan(&n); err != nil {
		t.Fatalf("count after rotate: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 session row after rotate, got %d", n)
	}
}

func TestOIDCCallbackRejectsMissingState(t *testing.T) {
	env := newOIDCTestEnv(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=x", nil)
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOIDCCallbackRejectsMissingCookie(t *testing.T) {
	env := newOIDCTestEnv(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=x&state=abc", nil)
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOIDCCallbackRejectsStateMismatch(t *testing.T) {
	env := newOIDCTestEnv(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=x&state=from-url", nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "from-cookie"})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "CSRF") {
		t.Errorf("expected CSRF hint, got: %s", w.Body.String())
	}
}

func TestOIDCCallbackPropagatesIdPError(t *testing.T) {
	env := newOIDCTestEnv(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?error=access_denied&error_description=User+declined", nil)
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "access_denied") {
		t.Errorf("expected access_denied hint, got: %s", w.Body.String())
	}
}

func TestOIDCCallbackRejectsUnverifiedEmail(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{Email: env.email, EmailVerified: false}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestOIDCCallbackRejectsEmptyEmail(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{EmailVerified: true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOIDCCallbackRejectsDisallowedDomain(t *testing.T) {
	env := newOIDCTestEnv(t)
	env.cfg.AllowedDomains = []string{"acme.io"}
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{
		Email: "intruder@evil.org", EmailVerified: true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestOIDCCallbackRejectsUnknownMember(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{
		Email: "stranger@oidc.co", EmailVerified: true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no team membership") {
		t.Errorf("expected membership hint, got: %s", w.Body.String())
	}
}

func TestOIDCCallbackPropagatesVerifyError(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.exchErr = errors.New("id_token verification failed: signature invalid")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "signature invalid") {
		t.Errorf("expected verify-error passthrough, got: %s", w.Body.String())
	}
}

func TestOIDCCallbackClearsCookieAfterUse(t *testing.T) {
	env := newOIDCTestEnv(t)
	stateCookie, _ := env.runLogin(t)
	env.verifier.claims = &OIDCClaims{
		Email: env.email, EmailVerified: true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/auth/oidc/callback?code=c&state="+stateCookie, nil)
	r.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: stateCookie})
	oidcCallbackHandler(env.store, env.cfg, env.verifier).ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()
	var cleared bool
	for _, c := range resp.Cookies() {
		if c.Name == oidcStateCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("state cookie was not cleared after callback")
	}
}

func TestMintSessionTokenHashesPlaintext(t *testing.T) {
	plain, hash, err := mintSessionToken()
	if err != nil {
		t.Fatalf("mintSessionToken: %v", err)
	}
	if len(plain) != 64 {
		t.Errorf("plain len: %d", len(plain))
	}
	if len(hash) != 64 {
		t.Errorf("hash len: %d", len(hash))
	}
	if plain == hash {
		t.Error("plain must differ from hash")
	}
}

func TestIsHTTPSRequest(t *testing.T) {
	cases := []struct {
		name string
		mod  func(r *http.Request)
		want bool
	}{
		{"plain http", func(_ *http.Request) {}, false},
		{"X-Forwarded-Proto=https", func(r *http.Request) {
			r.Header.Set("X-Forwarded-Proto", "https")
		}, true},
		{"X-Forwarded-Proto=HTTPS uppercase", func(r *http.Request) {
			r.Header.Set("X-Forwarded-Proto", "HTTPS")
		}, true},
		{"X-Forwarded-Proto=http", func(r *http.Request) {
			r.Header.Set("X-Forwarded-Proto", "http")
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.mod(r)
			if got := isHTTPSRequest(r); got != tc.want {
				t.Errorf("isHTTPSRequest = %v, want %v", got, tc.want)
			}
		})
	}
}
