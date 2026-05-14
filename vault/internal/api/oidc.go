package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the operator-supplied OIDC settings for a self-hosted
// vault. All fields are read from environment variables at startup so
// the docker-compose deployment surface stays in one place (no edits to
// korva.config.json required).
//
// Required envs:
//
//	KORVA_OIDC_ISSUER_URL    — Discovery URL (no trailing slash).
//	KORVA_OIDC_CLIENT_ID     — Application client ID issued by the IdP.
//	KORVA_OIDC_CLIENT_SECRET — Application secret. Confidential client only.
//	KORVA_OIDC_REDIRECT_URL  — Public URL the IdP can reach, ending in
//	                            /auth/oidc/callback.
//
// Optional envs:
//
//	KORVA_OIDC_ALLOWED_DOMAINS — Comma-separated email-suffix allowlist
//	                              ("acme.io,partner.dev"). Empty = no
//	                              domain filter (still email_verified+
//	                              team_member required).
//	KORVA_OIDC_SCOPES         — Comma-separated scope list. Defaults to
//	                              "openid,email,profile".
//
// All five required envs must be present to enable OIDC; if any is
// blank LoadOIDCConfigFromEnv returns nil and the /auth/oidc/* routes
// are not registered.
type OIDCConfig struct {
	IssuerURL      string
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	AllowedDomains []string
	Scopes         []string
}

// LoadOIDCConfigFromEnv returns a populated OIDCConfig or nil when
// OIDC is not configured. It performs only syntactic validation; the
// IdP itself is contacted lazily on first request.
func LoadOIDCConfigFromEnv() *OIDCConfig {
	issuer := strings.TrimSpace(os.Getenv("KORVA_OIDC_ISSUER_URL"))
	clientID := strings.TrimSpace(os.Getenv("KORVA_OIDC_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("KORVA_OIDC_CLIENT_SECRET"))
	redirect := strings.TrimSpace(os.Getenv("KORVA_OIDC_REDIRECT_URL"))
	if issuer == "" || clientID == "" || clientSecret == "" || redirect == "" {
		return nil
	}
	cfg := &OIDCConfig{
		IssuerURL:    strings.TrimRight(issuer, "/"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirect,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	if raw := strings.TrimSpace(os.Getenv("KORVA_OIDC_ALLOWED_DOMAINS")); raw != "" {
		for _, d := range strings.Split(raw, ",") {
			d = strings.TrimSpace(d)
			d = strings.ToLower(strings.TrimPrefix(d, "@"))
			if d != "" {
				cfg.AllowedDomains = append(cfg.AllowedDomains, d)
			}
		}
	}
	if raw := strings.TrimSpace(os.Getenv("KORVA_OIDC_SCOPES")); raw != "" {
		var scopes []string
		for _, sc := range strings.Split(raw, ",") {
			sc = strings.TrimSpace(sc)
			if sc != "" {
				scopes = append(scopes, sc)
			}
		}
		if len(scopes) > 0 {
			cfg.Scopes = scopes
		}
	}
	return cfg
}

// EmailDomainAllowed returns true when email belongs to a configured
// allowed domain (or when no allowlist is set).
func (c *OIDCConfig) EmailDomainAllowed(email string) bool {
	if len(c.AllowedDomains) == 0 {
		return true
	}
	idx := strings.LastIndex(email, "@")
	if idx < 0 || idx == len(email)-1 {
		return false
	}
	suffix := strings.ToLower(email[idx+1:])
	for _, d := range c.AllowedDomains {
		if suffix == d {
			return true
		}
	}
	return false
}

// OIDCClaims is the minimal subset of id_token claims we pull from the
// IdP. Vendor-specific extras (groups, picture, locale) are
// intentionally ignored — the vault's identity unit is "email already
// invited to a team".
type OIDCClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// OIDCVerifier abstracts the parts of the OIDC dance the HTTP handlers
// need. The production implementation is realOIDCVerifier (wrapping
// go-oidc + oauth2); tests fake this directly so they never need a
// live IdP.
type OIDCVerifier interface {
	// AuthCodeURL returns the IdP authorize endpoint with state baked in.
	AuthCodeURL(state string) string
	// ExchangeAndVerify swaps the auth code for tokens and verifies the
	// id_token signature + expiry. The returned claims are JSON-decoded
	// from the verified id_token payload.
	ExchangeAndVerify(ctx context.Context, code string) (*OIDCClaims, error)
}

// realOIDCVerifier is the production implementation. Lazy-init pattern:
// the underlying oidc.Provider hits the IdP's discovery endpoint, which
// we want to defer until the first /auth/oidc/* request so vault
// startup is never blocked by IdP availability.
type realOIDCVerifier struct {
	cfg      *OIDCConfig
	provider *oidc.Provider
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewOIDCVerifier eagerly contacts the IdP and returns a ready verifier.
// Use this when you want fast-fail at startup; otherwise prefer the
// lazyOIDCVerifier wrapper below.
func NewOIDCVerifier(ctx context.Context, cfg *OIDCConfig) (OIDCVerifier, error) {
	if cfg == nil {
		return nil, errors.New("oidc: config is nil")
	}
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery failed for %s: %w", cfg.IssuerURL, err)
	}
	return &realOIDCVerifier{
		cfg:      cfg,
		provider: provider,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (v *realOIDCVerifier) AuthCodeURL(state string) string {
	return v.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (v *realOIDCVerifier) ExchangeAndVerify(ctx context.Context, code string) (*OIDCClaims, error) {
	tok, err := v.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc: code exchange failed: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, errors.New("oidc: id_token missing from token response")
	}
	idTok, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", err)
	}
	var claims OIDCClaims
	if err := idTok.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: claims decode failed: %w", err)
	}
	return &claims, nil
}

// lazyOIDCVerifier defers the discovery call to the first invocation.
// It guards initialization so concurrent first-requests don't double-
// dial the IdP.
type lazyOIDCVerifier struct {
	cfg *OIDCConfig
	mu  sync.Mutex
	v   OIDCVerifier
}

// NewLazyOIDCVerifier wraps cfg with a verifier that contacts the IdP
// only when a route handler first needs it. Vault startup stays fast
// even when the IdP is unreachable.
func NewLazyOIDCVerifier(cfg *OIDCConfig) OIDCVerifier {
	return &lazyOIDCVerifier{cfg: cfg}
}

func (l *lazyOIDCVerifier) ensure(ctx context.Context) (OIDCVerifier, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.v != nil {
		return l.v, nil
	}
	v, err := NewOIDCVerifier(ctx, l.cfg)
	if err != nil {
		return nil, err
	}
	l.v = v
	return v, nil
}

func (l *lazyOIDCVerifier) AuthCodeURL(state string) string {
	// The Auth URL needs the OAuth endpoint; if we haven't dialed yet,
	// dial with a background context (the caller doesn't have one in
	// scope for the redirect-build step).
	v, err := l.ensure(context.Background())
	if err != nil {
		// Return an empty URL — the handler treats this as 503.
		return ""
	}
	return v.AuthCodeURL(state)
}

func (l *lazyOIDCVerifier) ExchangeAndVerify(ctx context.Context, code string) (*OIDCClaims, error) {
	v, err := l.ensure(ctx)
	if err != nil {
		return nil, err
	}
	return v.ExchangeAndVerify(ctx, code)
}
