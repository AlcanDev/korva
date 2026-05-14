package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"
)

// Phase 17.A — stateless signed OIDC state.
//
// The original Phase 15.D implementation stored the CSRF nonce in a
// short-lived cookie and compared it on callback. That works for a
// single-tab login flow but breaks when the operator opens two tabs
// concurrently: the second login overwrites the first's cookie, and
// the first tab's callback fails with "state mismatch" even though
// nothing malicious happened.
//
// The fix is to make the state self-contained. We embed:
//
//   8  bytes — issued_at (big-endian unix seconds)
//   16 bytes — random nonce (per-request, unguessable)
//   32 bytes — HMAC-SHA256(signingKey, issued_at || nonce)
//
// Total 56 bytes raw → 75 chars when base64url-encoded. We base the
// signing key on the vault's admin.key SHA256 — already unique per
// install, already loaded at startup, already required for the rest
// of the admin surface. If admin.key is missing the OIDC routes
// refuse to register (caller's choice), so we can't ship a vault
// where the signing key would be predictable.
//
// CSRF protection still holds: an attacker who tricks the user into
// a forged login can't predict the nonce, can't forge the HMAC, and
// can't replay a captured state past oidcStateTTL.

// oidcStateBytes is the binary length of a state payload before
// base64-encoding. issued_at + nonce + HMAC.
const oidcStateBytes = 8 + 16 + sha256.Size

// signedOIDCState wraps an opaque signing key. Construct via
// newSignedOIDCState; pass to the login/callback handlers so they
// can mint and verify state without touching crypto primitives
// directly.
type signedOIDCState struct {
	key []byte // 32-byte HMAC key
	// now is overridable for tests; production callers leave it nil
	// and the methods fall back to time.Now().
	now func() time.Time
}

// newSignedOIDCState derives a per-install HMAC signing key from the
// vault's admin key bytes and returns the helper. Returns an error
// when admin.key isn't available — OIDC must not run with a
// predictable signing key.
func newSignedOIDCState(adminKeyPath, adminKeyOverride string) (*signedOIDCState, error) {
	keyBytes, err := readAdminKeyBytes(adminKeyPath, adminKeyOverride)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(keyBytes)
	return &signedOIDCState{key: digest[:]}, nil
}

// newSignedOIDCStateFromKey is the test-friendly constructor.
// Production code goes through newSignedOIDCState.
func newSignedOIDCStateFromKey(key []byte) *signedOIDCState {
	return &signedOIDCState{key: key}
}

// nowFn returns time.Now() unless overridden by tests.
func (s *signedOIDCState) nowFn() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Mint produces a fresh signed state token. Each call returns a
// unique value — the 16-byte random nonce guarantees collision-
// resistance across concurrent logins.
func (s *signedOIDCState) Mint() (string, error) {
	if len(s.key) == 0 {
		return "", errors.New("oidc state: signing key is empty")
	}
	buf := make([]byte, oidcStateBytes)
	issuedAt := s.nowFn().Unix()
	binary.BigEndian.PutUint64(buf[0:8], uint64(issuedAt))
	if _, err := rand.Read(buf[8:24]); err != nil {
		return "", fmt.Errorf("oidc state: nonce generation failed: %w", err)
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(buf[:24])
	copy(buf[24:], mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Verify decodes the token, checks the HMAC signature, and confirms
// the issuance timestamp falls within ttl of now. Returns the issue
// timestamp on success — useful for telemetry but most callers
// ignore it.
func (s *signedOIDCState) Verify(token string, ttl time.Duration) (time.Time, error) {
	if len(s.key) == 0 {
		return time.Time{}, errors.New("oidc state: signing key is empty")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, errors.New("oidc state: token is not valid base64url")
	}
	if len(raw) != oidcStateBytes {
		return time.Time{}, fmt.Errorf("oidc state: unexpected length %d", len(raw))
	}
	gotMAC := raw[24:]
	mac := hmac.New(sha256.New, s.key)
	mac.Write(raw[:24])
	wantMAC := mac.Sum(nil)
	if subtle.ConstantTimeCompare(gotMAC, wantMAC) != 1 {
		return time.Time{}, errors.New("oidc state: signature mismatch")
	}
	issuedAt := time.Unix(int64(binary.BigEndian.Uint64(raw[0:8])), 0)
	age := s.nowFn().Sub(issuedAt)
	if age < 0 {
		// Clock skew: token from the future. Allow up to one TTL of
		// slack so a small skew between the issuer and verifier
		// doesn't surface as a hard rejection.
		if -age > ttl {
			return time.Time{}, errors.New("oidc state: token issued in the future beyond skew window")
		}
	} else if age > ttl {
		return time.Time{}, fmt.Errorf("oidc state: token expired (age=%s, ttl=%s)", age, ttl)
	}
	return issuedAt, nil
}

// readAdminKeyBytes mirrors what `internal/admin` does to obtain the
// key bytes, but stays out of that package's import graph (this file
// only needs the raw bytes for HMAC derivation, not the validation
// machinery). When KORVA_ADMIN_KEY is set we use it verbatim;
// otherwise we read the file at keyPath. Both paths are how the
// admin middleware already resolves the key, so this is consistent.
func readAdminKeyBytes(keyPath, keyOverride string) ([]byte, error) {
	if keyOverride != "" {
		return []byte(keyOverride), nil
	}
	if keyPath == "" {
		return nil, errors.New("oidc state: no admin key path configured")
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("oidc state: cannot read admin key at %s: %w", keyPath, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("oidc state: admin key at %s is empty", keyPath)
	}
	return data, nil
}
