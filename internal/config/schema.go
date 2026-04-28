package config

// KorvaConfig is the public base configuration loaded from korva.config.json.
// It contains no secrets. Team-specific overrides are applied via TeamProfile.
type KorvaConfig struct {
	Version  string         `json:"version"`
	Project  string         `json:"project"`
	Team     string         `json:"team"`
	Country  string         `json:"country"`
	Vault    VaultConfig    `json:"vault"`
	Lore     LoreConfig     `json:"lore"`
	Sentinel SentinelConfig `json:"sentinel"`
	Hive     HiveConfig     `json:"hive"`
	License  LicenseConfig  `json:"license"`
	Agent    string         `json:"agent"` // copilot | claude | cursor
}

// HiveConfig controls the cloud community brain (Korva Hive).
// Enabled by default. Users can opt out globally via Enabled=false
// or via env var KORVA_HIVE_DISABLE=1 (kill switch).
type HiveConfig struct {
	Enabled        bool     `json:"enabled"`
	Endpoint       string   `json:"endpoint"`
	IntervalMin    int      `json:"interval_minutes"`
	AllowedTypes   []string `json:"allowed_types"`
	RejectPatterns []string `json:"reject_patterns,omitempty"`
}

// LicenseConfig controls Korva for Teams licensing.
// Empty defaults work for the Community tier (no license needed).
type LicenseConfig struct {
	ActivationURL string `json:"activation_url,omitempty"`
}

// VaultConfig holds Vault server settings.
type VaultConfig struct {
	Port            int      `json:"port"`
	AutoStart       bool     `json:"auto_start"`
	SyncRepo        string   `json:"sync_repo,omitempty"`
	SyncBranch      string   `json:"sync_branch,omitempty"`
	AutoSync        bool     `json:"auto_sync,omitempty"`
	SyncIntervalMin int      `json:"sync_interval_minutes,omitempty"`
	PrivatePatterns []string `json:"private_patterns,omitempty"`
	// RetentionDays auto-purges observations older than N days (0 = disabled).
	// Requires Korva for Teams license with the custom_whitelist feature.
	RetentionDays int `json:"retention_days,omitempty"`
	// WebhookURL receives a POST for every saved observation (best-effort, async).
	// Useful for Slack / Teams / Discord integrations. Teams-tier feature.
	WebhookURL string `json:"webhook_url,omitempty"`
}

// LoreConfig holds Scroll management settings.
type LoreConfig struct {
	ActiveScrolls  []string `json:"active_scrolls"`
	ScrollPriority string   `json:"scroll_priority,omitempty"` // private_first | public_first
}

// SentinelConfig holds pre-commit hook settings.
type SentinelConfig struct {
	Enabled          bool     `json:"enabled"`
	Hooks            []string `json:"hooks"` // pre-commit, pre-push
	RulesPath        string   `json:"rules_path,omitempty"`
	BlockOnViolation bool     `json:"block_on_violation,omitempty"`
}

// TeamProfile is the structure loaded from a private team profile repository.
// It can only override the fields defined in ProfileOverrides — nothing else.
type TeamProfile struct {
	Profile   ProfileMeta      `json:"profile"`
	Overrides ProfileOverrides `json:"overrides"`
	Access    AccessPolicy     `json:"access"`
}

// ProfileMeta contains metadata about the team profile.
type ProfileMeta struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	Owner      string `json:"owner"`
	Team       string `json:"team"`
	SourceRepo string `json:"source_repo"`
}

// ProfileOverrides contains the fields that a team profile is allowed to override.
// All fields are pointers so we can distinguish "not specified" from "zero value".
// Fields marked "Teams only" require the FeatureCustomWhitelist license feature.
type ProfileOverrides struct {
	Vault        *VaultOverride        `json:"vault,omitempty"`
	Sentinel     *SentinelOverride     `json:"sentinel,omitempty"`
	Lore         *LoreOverride         `json:"lore,omitempty"`
	Instructions *InstructionsOverride `json:"instructions,omitempty"`
	// Teams only
	AIModel      *AIModelOverride      `json:"ai_model,omitempty"`
	CustomSkills *CustomSkillsOverride `json:"custom_skills,omitempty"`
}

// AIModelOverride allows Teams-tier profiles to override the AI model provider.
type AIModelOverride struct {
	Provider string `json:"provider,omitempty"` // e.g. "anthropic", "openai", "azure"
	Model    string `json:"model,omitempty"`    // e.g. "claude-opus-4-7"
	BaseURL  string `json:"base_url,omitempty"` // optional custom endpoint
}

// CustomSkillsOverride allows Teams-tier profiles to specify additional skills.
type CustomSkillsOverride struct {
	Path   string   `json:"path,omitempty"`   // relative path to skills directory in profile repo
	Skills []string `json:"skills,omitempty"` // list of skill names to activate
}

// VaultOverride contains vault settings that a team profile can override.
type VaultOverride struct {
	SyncRepo        string   `json:"sync_repo,omitempty"`
	SyncBranch      string   `json:"sync_branch,omitempty"`
	AutoSync        *bool    `json:"auto_sync,omitempty"`
	SyncIntervalMin *int     `json:"sync_interval_minutes,omitempty"`
	PrivatePatterns []string `json:"private_patterns,omitempty"`
	// Teams only
	RetentionDays *int   `json:"vault_retention_days,omitempty"`
	CloudEndpoint string `json:"cloud_sync_endpoint,omitempty"`
}

// SentinelOverride contains sentinel settings that a team profile can override.
type SentinelOverride struct {
	RulesPath        string   `json:"rules_path,omitempty"`
	BlockOnViolation *bool    `json:"block_on_violation,omitempty"`
	Hooks            []string `json:"hooks,omitempty"`
}

// LoreOverride contains lore settings that a team profile can override.
type LoreOverride struct {
	PrivateScrollsPath string   `json:"private_scrolls_path,omitempty"`
	ScrollPriority     string   `json:"scroll_priority,omitempty"`
	ActiveScrolls      []string `json:"active_scrolls,omitempty"`
	// Teams only
	PrivateScrollsDir string `json:"private_scrolls_dir,omitempty"`
}

// InstructionsOverride contains instruction merge settings.
type InstructionsOverride struct {
	CopilotExtensions string `json:"copilot_extensions,omitempty"`
	ClaudeExtensions  string `json:"claude_extensions,omitempty"`
	MergeStrategy     string `json:"merge_strategy,omitempty"` // append | replace
}

// AccessPolicy defines who can use this team profile.
type AccessPolicy struct {
	RequireSSHKey  bool     `json:"require_ssh_key,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// DefaultConfig returns a KorvaConfig with sensible defaults.
func DefaultConfig() KorvaConfig {
	return KorvaConfig{
		Version: "1",
		Country: "CL",
		Vault: VaultConfig{
			Port:      7437,
			AutoStart: true,
			PrivatePatterns: []string{
				"password", "passwd", "pwd",
				"token", "secret", "api_key", "apikey",
				"ROLE_ID", "SECRET_ID",
			},
		},
		Lore: LoreConfig{
			ActiveScrolls:  []string{"forge-sdd"},
			ScrollPriority: "private_first",
		},
		Sentinel: SentinelConfig{
			Enabled:          true,
			Hooks:            []string{"pre-commit"},
			BlockOnViolation: true,
		},
		Hive: HiveConfig{
			Enabled:      true,
			Endpoint:     "https://hive.korva.dev",
			IntervalMin:  15,
			AllowedTypes: []string{"pattern", "decision", "learning"},
		},
		License: LicenseConfig{
			ActivationURL: "https://licensing.korva.dev/v1/activate",
		},
		Agent: "copilot",
	}
}
