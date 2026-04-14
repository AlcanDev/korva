package profile

import (
	"fmt"
	"strings"

	"github.com/alcandev/korva/internal/config"
)

// allowedOverrideKeys lists the top-level keys a team profile is allowed to override.
// Any other key in the overrides section is rejected to prevent injection attacks.
var allowedOverrideKeys = map[string]bool{
	"vault":        true,
	"sentinel":     true,
	"lore":         true,
	"instructions": true,
}

// Validate checks that a TeamProfile is well-formed and does not contain
// disallowed fields or values.
func Validate(profile config.TeamProfile) error {
	if err := validateMeta(profile.Profile); err != nil {
		return err
	}
	if err := validateOverrides(profile.Overrides); err != nil {
		return err
	}
	return nil
}

func validateMeta(meta config.ProfileMeta) error {
	if strings.TrimSpace(meta.ID) == "" {
		return fmt.Errorf("team profile: profile.id is required")
	}
	if strings.TrimSpace(meta.Version) == "" {
		return fmt.Errorf("team profile: profile.version is required")
	}
	if strings.TrimSpace(meta.Team) == "" {
		return fmt.Errorf("team profile: profile.team is required")
	}
	// Ensure profile ID has no path traversal characters
	if strings.Contains(meta.ID, "..") || strings.Contains(meta.ID, "/") || strings.Contains(meta.ID, "\\") {
		return fmt.Errorf("team profile: profile.id contains invalid characters: %s", meta.ID)
	}
	return nil
}

func validateOverrides(overrides config.ProfileOverrides) error {
	// Validate vault override
	if o := overrides.Vault; o != nil {
		// SyncRepo must not contain shell metacharacters to prevent injection
		if o.SyncRepo != "" && containsShellMetachars(o.SyncRepo) {
			return fmt.Errorf("team profile: vault.sync_repo contains invalid characters")
		}
	}
	// Validate instructions merge strategy
	if o := overrides.Instructions; o != nil {
		valid := map[string]bool{"append": true, "replace": true, "": true}
		if !valid[o.MergeStrategy] {
			return fmt.Errorf("team profile: instructions.merge_strategy must be 'append' or 'replace', got %q", o.MergeStrategy)
		}
	}
	return nil
}

// containsShellMetachars checks for characters that could cause shell injection.
func containsShellMetachars(s string) bool {
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "<", ">", "\n", "\r"}
	for _, c := range dangerous {
		if strings.Contains(s, c) {
			return true
		}
	}
	return false
}
