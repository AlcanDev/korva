package license

// Feature constants gated by the Korva for Teams license.
// Add new features here, then check via license.HasFeature(FeatureX).
const (
	FeatureAdminSkills     = "admin_skills"      // editor de skills/instrucciones en Beacon
	FeatureCustomWhitelist = "custom_whitelist"  // overrides extra en team-profile
	FeatureAuditLog        = "audit_log"         // bitácora de cambios admin
	FeaturePrivateScrolls  = "private_scrolls"   // scrolls privados gestionados desde panel
	FeatureMultiProfile    = "multi_profile"     // múltiples team profiles activos a la vez
	FeatureCloudPrivate    = "cloud_private"     // sync privado equipo (no comunitario)
)

// Tier is the license tier name.
type Tier string

const (
	TierCommunity Tier = "community" // open source, gratis
	TierTeams     Tier = "teams"     // pago
)
