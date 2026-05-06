package license

// Feature constants gated by license tier.
// Add new features here, then gate via license.HasFeature(FeatureX).
//
// All paid features live under the single Teams tier. Per-feature
// availability is driven by the Features array in the JWS payload, not by
// the tier name — the licensing server picks which subset a given license
// includes based on the customer's plan.
const (
	FeatureAdminSkills      = "admin_skills"       // Skills Hub editor in Beacon + vault_skill_match
	FeatureCustomWhitelist  = "custom_whitelist"   // extra overrides in team-profile
	FeatureAuditLog         = "audit_log"          // admin change audit trail
	FeaturePrivateScrolls   = "private_scrolls"    // team-managed private scrolls
	FeatureSmartSkillLoader = "smart_skill_loader" // auto-skill injection in vault_context
	FeatureCodeHealth       = "code_health"        // vault_code_health MCP tool (A-F grade)
	FeaturePatternMine      = "pattern_mine"       // vault_pattern_mine MCP tool
	FeatureMultiProfile     = "multi_profile"      // multiple active team profiles simultaneously
	FeatureCloudPrivate     = "cloud_private"      // private team sync (not community)
)

// Tier is the license tier name.
type Tier string

const (
	TierCommunity Tier = "community" // free — core vault + basic MCP tools
	TierTeams     Tier = "teams"     // paid — every gated feature lives here
)
