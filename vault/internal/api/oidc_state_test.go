package api

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phase 17.A — direct unit tests for the signed state helper.
// Handler-level tests in auth_oidc_test.go cover the end-to-end
// behaviour; this file pins the crypto contract.

func TestSignedOIDCState_MintProducesUniqueTokens(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		s, err := signer.Mint()
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		if seen[s] {
			t.Fatalf("nonce collision after %d iterations", i)
		}
		seen[s] = true
		// All tokens should decode to exactly oidcStateBytes.
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Errorf("token is not valid base64url: %v", err)
		}
		if len(raw) != oidcStateBytes {
			t.Errorf("raw len = %d, want %d", len(raw), oidcStateBytes)
		}
	}
}

func TestSignedOIDCState_VerifyRoundtrip(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	state, err := signer.Mint()
	if err != nil {
		t.Fatal(err)
	}
	issuedAt, err := signer.Verify(state, oidcStateTTL)
	if err != nil {
		t.Errorf("verify same-signer token: %v", err)
	}
	if time.Since(issuedAt) > time.Second {
		t.Errorf("issuedAt looks wrong: %v", issuedAt)
	}
}

func TestSignedOIDCState_VerifyRejectsTamperedSignature(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	state, _ := signer.Mint()
	raw, _ := base64.RawURLEncoding.DecodeString(state)
	raw[len(raw)-1] ^= 0xFF // flip a bit in the HMAC tail
	tampered := base64.RawURLEncoding.EncodeToString(raw)
	if _, err := signer.Verify(tampered, oidcStateTTL); err == nil {
		t.Error("expected signature-mismatch error")
	}
}

func TestSignedOIDCState_VerifyRejectsTamperedTimestamp(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	state, _ := signer.Mint()
	raw, _ := base64.RawURLEncoding.DecodeString(state)
	raw[0] ^= 0xFF // flip a bit in the issued-at field
	tampered := base64.RawURLEncoding.EncodeToString(raw)
	// The HMAC covered the timestamp too; flipping it breaks the
	// signature, so verification fails before we even check the TTL.
	if _, err := signer.Verify(tampered, oidcStateTTL); err == nil {
		t.Error("expected error")
	}
}

func TestSignedOIDCState_VerifyRejectsExpired(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	signer.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	state, _ := signer.Mint()
	signer.now = nil // verify with real now
	if _, err := signer.Verify(state, oidcStateTTL); err == nil {
		t.Error("expected expired error")
	}
}

func TestSignedOIDCState_VerifyAcceptsSmallClockSkew(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	// Token "from the future" by 1 second — well within ttl.
	signer.now = func() time.Time { return time.Now().Add(1 * time.Second) }
	state, _ := signer.Mint()
	signer.now = nil
	if _, err := signer.Verify(state, oidcStateTTL); err != nil {
		t.Errorf("small forward skew should be tolerated: %v", err)
	}
}

func TestSignedOIDCState_VerifyRejectsFarFutureToken(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	signer.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	state, _ := signer.Mint()
	signer.now = nil
	if _, err := signer.Verify(state, oidcStateTTL); err == nil {
		t.Error("expected error for far-future token")
	}
}

func TestSignedOIDCState_VerifyRejectsForeignKey(t *testing.T) {
	a := newSignedOIDCStateFromKey([]byte("alice"))
	b := newSignedOIDCStateFromKey([]byte("bob"))
	state, _ := a.Mint()
	if _, err := b.Verify(state, oidcStateTTL); err == nil {
		t.Error("expected signature mismatch when verifier has different key")
	}
}

func TestSignedOIDCState_VerifyRejectsBadInput(t *testing.T) {
	signer := newSignedOIDCStateFromKey([]byte("k"))
	cases := []struct {
		name, token string
	}{
		{"empty", ""},
		{"not base64", "!!!"},
		{"truncated", "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := signer.Verify(tc.token, oidcStateTTL); err == nil {
				t.Errorf("expected error for %q", tc.token)
			}
		})
	}
}

func TestSignedOIDCState_MintFailsWithEmptyKey(t *testing.T) {
	signer := newSignedOIDCStateFromKey(nil)
	if _, err := signer.Mint(); err == nil {
		t.Error("expected error for empty key")
	}
}

func TestNewSignedOIDCState_PrefersEnvOverride(t *testing.T) {
	signer, err := newSignedOIDCState("/does/not/exist", "from-env")
	if err != nil {
		t.Fatalf("override should bypass file read: %v", err)
	}
	// Mint should work — proves the key is non-empty.
	if _, err := signer.Mint(); err != nil {
		t.Errorf("Mint after env override: %v", err)
	}
}

func TestNewSignedOIDCState_ReadsFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")
	if err := os.WriteFile(keyPath, []byte("on-disk-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := newSignedOIDCState(keyPath, "")
	if err != nil {
		t.Fatalf("disk read: %v", err)
	}
	state, err := signer.Mint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.Verify(state, oidcStateTTL); err != nil {
		t.Errorf("roundtrip via disk: %v", err)
	}
}

func TestNewSignedOIDCState_FailsWhenKeyMissing(t *testing.T) {
	_, err := newSignedOIDCState("/does/not/exist/admin.key", "")
	if err == nil {
		t.Error("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "admin key") {
		t.Errorf("error should mention admin key: %v", err)
	}
}

func TestNewSignedOIDCState_FailsWhenKeyEmpty(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")
	if err := os.WriteFile(keyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newSignedOIDCState(keyPath, "")
	if err == nil {
		t.Error("expected error for empty key file")
	}
}

func TestNewSignedOIDCState_FailsWithNoPathAndNoOverride(t *testing.T) {
	_, err := newSignedOIDCState("", "")
	if err == nil {
		t.Error("expected error when both path and override are empty")
	}
}

// Different invocations of newSignedOIDCState with the SAME admin
// key bytes should produce signers that can verify each other's
// tokens — vault restarts must not invalidate in-flight logins (the
// 10-minute TTL handles staleness, not restart).
func TestSignedOIDCState_KeyDerivationIsDeterministic(t *testing.T) {
	a, _ := newSignedOIDCState("", "stable-key")
	b, _ := newSignedOIDCState("", "stable-key")
	state, err := a.Mint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Verify(state, oidcStateTTL); err != nil {
		t.Errorf("cross-instance verify with same key bytes: %v", err)
	}
}
