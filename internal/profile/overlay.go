package profile

import (
	"github.com/alcandev/korva/internal/config"
)

// ApplyOverrides merges a TeamProfile's overrides onto a base KorvaConfig.
// It does NOT mutate either input — it returns a new KorvaConfig.
// Only fields explicitly set in the profile (non-nil pointers) are applied.
func ApplyOverrides(base config.KorvaConfig, profile config.TeamProfile) config.KorvaConfig {
	result := base // value copy

	if o := profile.Overrides.Vault; o != nil {
		if o.SyncRepo != "" {
			result.Vault.SyncRepo = o.SyncRepo
		}
		if o.SyncBranch != "" {
			result.Vault.SyncBranch = o.SyncBranch
		}
		if o.AutoSync != nil {
			result.Vault.AutoSync = *o.AutoSync
		}
		if o.SyncIntervalMin != nil {
			result.Vault.SyncIntervalMin = *o.SyncIntervalMin
		}
		if len(o.PrivatePatterns) > 0 {
			// Append to base patterns, don't replace
			seen := make(map[string]bool)
			for _, p := range result.Vault.PrivatePatterns {
				seen[p] = true
			}
			for _, p := range o.PrivatePatterns {
				if !seen[p] {
					result.Vault.PrivatePatterns = append(result.Vault.PrivatePatterns, p)
				}
			}
		}
	}

	if o := profile.Overrides.Sentinel; o != nil {
		if o.RulesPath != "" {
			result.Sentinel.RulesPath = o.RulesPath
		}
		if o.BlockOnViolation != nil {
			result.Sentinel.BlockOnViolation = *o.BlockOnViolation
		}
		if len(o.Hooks) > 0 {
			result.Sentinel.Hooks = o.Hooks
		}
	}

	if o := profile.Overrides.Lore; o != nil {
		if o.ScrollPriority != "" {
			result.Lore.ScrollPriority = o.ScrollPriority
		}
		if len(o.ActiveScrolls) > 0 {
			// Merge: team scrolls + base scrolls, no duplicates
			seen := make(map[string]bool)
			merged := make([]string, 0)
			for _, s := range o.ActiveScrolls {
				if !seen[s] {
					merged = append(merged, s)
					seen[s] = true
				}
			}
			for _, s := range result.Lore.ActiveScrolls {
				if !seen[s] {
					merged = append(merged, s)
					seen[s] = true
				}
			}
			result.Lore.ActiveScrolls = merged
		}
	}

	return result
}
