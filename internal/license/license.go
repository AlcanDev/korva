// Package license verifies and gates the Korva for Teams license.
//
// The license is a JWS (RS256) signed by the Korva licensing server.
// Verification is fully offline — the public key is embedded into the binary.
// After activation the JWS is stored at ~/.korva/license.key. A local state
// file (~/.korva/license.state.json) tracks the last successful online
// heartbeat; if more than GraceDays elapse without contact, HasFeature
// returns false and the install degrades to Community tier with a banner.
package license

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// License is the parsed, validated payload from the JWS.
type License struct {
	LicenseID string    `json:"license_id"`
	TeamID    string    `json:"sub"`
	Tier      Tier      `json:"tier"`
	Features  []string  `json:"features"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
	GraceDays int       `json:"grace_days"`
	Seats     int       `json:"seats"`
}

// State persists the last successful heartbeat. Stored as JSON next to the license file.
type State struct {
	LastHeartbeat time.Time `json:"last_heartbeat"`
	LicenseID     string    `json:"license_id"`
}

// ErrMissing is returned by Load when no license file is present.
var ErrMissing = errors.New("license: no license installed (Community tier)")

// Load reads the JWS at path, verifies the signature against an embedded
// public key, and returns the parsed License. If the file is missing,
// returns ErrMissing — callers should treat this as Community tier.
func Load(path string) (*License, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrMissing
		}
		return nil, fmt.Errorf("license: read: %w", err)
	}
	return verifyJWS(string(raw))
}

// Validate runs business-rule checks against the parsed license:
// expiration, tier whitelist, etc. Signature is already verified by Load.
func (l *License) Validate() error {
	if l == nil {
		return errors.New("license: nil")
	}
	if l.Tier != TierTeams && l.Tier != TierCommunity {
		return fmt.Errorf("license: unknown tier %q", l.Tier)
	}
	if !l.ExpiresAt.IsZero() && time.Now().After(l.ExpiresAt) {
		return errors.New("license: expired")
	}
	return nil
}

// HasFeature reports whether the license includes a given feature.
// Always false for nil license (Community tier).
func (l *License) HasFeature(feature string) bool {
	if l == nil || l.Validate() != nil {
		return false
	}
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// CurrentTier returns the effective tier for this install, taking the
// heartbeat grace window into account. If the license is valid but the
// heartbeat lapsed, the install degrades to Community tier.
func (l *License) CurrentTier(state *State) Tier {
	if l == nil {
		return TierCommunity
	}
	if l.Validate() != nil {
		return TierCommunity
	}
	if state != nil && l.GraceDays > 0 {
		if time.Since(state.LastHeartbeat) > time.Duration(l.GraceDays)*24*time.Hour {
			return TierCommunity
		}
	}
	return l.Tier
}

// GraceRemaining returns how much of the offline grace window is left.
// Negative durations indicate the grace period has lapsed.
func (l *License) GraceRemaining(state *State) time.Duration {
	if l == nil || state == nil || l.GraceDays <= 0 {
		return 0
	}
	deadline := state.LastHeartbeat.Add(time.Duration(l.GraceDays) * 24 * time.Hour)
	return time.Until(deadline)
}
