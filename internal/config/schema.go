package config

// KorvaConfig is the public base configuration loaded from korva.config.json.
// It contains no secrets. Team-specific overrides are applied via TeamProfile.
type KorvaConfig struct {
	Version   string          `json:"version"`
	Project   string          `json:"project"`
	Team      string          `json:"team"`
	Country   string          `json:"country"`
	Vault     VaultConfig     `json:"vault"`
	Lore      LoreConfig      `json:"lore"`
	Sentinel  SentinelConfig  `json:"sentinel"`
	Agent     string          `json:"agent"` // copilot | claude | cursor
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
}

// LoreConfig holds Scroll management settings.
type LoreConfig struct {
	ActiveScrolls  []string `json:"active_scrolls"`
	ScrollPriority string   `json:"scroll_priority,omitempty"` // private_first | public_first
}

// SentinelConfig holds pre-commit hook settings.
type SentinelConfig struct {
	Enabled         bool     `json:"enabled"`
	Hooks           []string `json:"hooks"` // pre-commit, pre-push
	RulesPath       string   `json:"rules_path,omitempty"`
	BlockOnViolation bool    `json:"block_on_violation,omitempty"`
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
type ProfileOverrides struct {
	Vault        *VaultOverride        `json:"vault,omitempty"`
	Sentinel     *SentinelOverride     `json:"sentinel,omitempty"`
	Lore         *LoreOverride         `json:"lore,omitempty"`
	Instructions *InstructionsOverride `json:"instructions,omitempty"`
}

// VaultOverride contains vault settings that a team profile can override.
type VaultOverride struct {
	SyncRepo        string   `json:"sync_repo,omitempty"`
	SyncBranch      string   `json:"sync_branch,omitempty"`
	AutoSync        *bool    `json:"auto_sync,omitempty"`
	SyncIntervalMin *int     `json:"sync_interval_minutes,omitempty"`
	PrivatePatterns []string `json:"private_patterns,omitempty"`
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
		Agent: "copilot",
	}
}
