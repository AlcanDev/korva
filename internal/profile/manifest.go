package profile

import (
	"fmt"
	"strings"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/license"
)

// allowedKeys returns the set of override top-level keys permitted for the
// given license tier. Community tier may only touch the four base keys.
// Teams tier with FeatureCustomWhitelist unlocks additional keys.
func allowedKeys(lic *license.License) map[string]bool {
	keys := map[string]bool{
		"vault":        true,
		"sentinel":     true,
		"lore":         true,
		"instructions": true,
	}
	if lic.HasFeature(license.FeatureCustomWhitelist) {
		keys["ai_model"] = true
		keys["custom_skills"] = true
		// sub-keys within existing overrides (validated per-struct)
		keys["vault_retention_days"] = true
		keys["cloud_sync_endpoint"] = true
		keys["private_scrolls_dir"] = true
	}
	return keys
}

// Validate checks that a TeamProfile is well-formed and does not contain
// disallowed fields or values.
// Pass nil for lic to validate at Community tier (no Teams-only features).
func Validate(profile config.TeamProfile, lic *license.License) error {
	if err := validateMeta(profile.Profile); err != nil {
		return err
	}
	if err := validateOverrides(profile.Overrides, lic); err != nil {
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
	if strings.Contains(meta.ID, "..") || strings.Contains(meta.ID, "/") || strings.Contains(meta.ID, "\\") {
		return fmt.Errorf("team profile: profile.id contains invalid characters: %s", meta.ID)
	}
	return nil
}

func validateOverrides(overrides config.ProfileOverrides, lic *license.License) error {
	allowed := allowedKeys(lic)

	// Reject Teams-only top-level keys when license doesn't permit them.
	if overrides.AIModel != nil && !allowed["ai_model"] {
		return fmt.Errorf("team profile: ai_model override requires Korva for Teams license with custom_whitelist feature")
	}
	if overrides.CustomSkills != nil && !allowed["custom_skills"] {
		return fmt.Errorf("team profile: custom_skills override requires Korva for Teams license with custom_whitelist feature")
	}
	// Reject Teams-only sub-fields within Vault when license doesn't permit them.
	if o := overrides.Vault; o != nil {
		if (o.RetentionDays != nil || o.CloudEndpoint != "") && !allowed["vault_retention_days"] {
			return fmt.Errorf("team profile: vault.vault_retention_days / cloud_sync_endpoint require Korva for Teams license")
		}
		if o.SyncRepo != "" && containsShellMetachars(o.SyncRepo) {
			return fmt.Errorf("team profile: vault.sync_repo contains invalid characters")
		}
	}
	// Reject Teams-only sub-fields within Lore when license doesn't permit them.
	if o := overrides.Lore; o != nil {
		if o.PrivateScrollsDir != "" && !allowed["private_scrolls_dir"] {
			return fmt.Errorf("team profile: lore.private_scrolls_dir requires Korva for Teams license")
		}
	}
	// Validate instructions merge strategy.
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
