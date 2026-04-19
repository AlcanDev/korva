package profile

import (
	"testing"

	"github.com/alcandev/korva/internal/config"
)

// --- helpers ---

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func validProfile() config.TeamProfile {
	return config.TeamProfile{
		Profile: config.ProfileMeta{
			ID:      "test-team",
			Version: "1.0.0",
			Team:    "Test Team",
		},
	}
}

// ---------------------------------------------------------------------------
// Validate — manifest.go
// ---------------------------------------------------------------------------

func TestValidate_OK(t *testing.T) {
	if err := Validate(validProfile(), nil); err != nil {
		t.Fatalf("expected valid profile, got error: %v", err)
	}
}

func TestValidate_MissingID(t *testing.T) {
	p := validProfile()
	p.Profile.ID = ""
	if err := Validate(p, nil); err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	p := validProfile()
	p.Profile.Version = ""
	if err := Validate(p, nil); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidate_MissingTeam(t *testing.T) {
	p := validProfile()
	p.Profile.Team = ""
	if err := Validate(p, nil); err == nil {
		t.Error("expected error for missing team")
	}
}

func TestValidate_PathTraversalInID(t *testing.T) {
	cases := []string{"../etc/passwd", "team/../../root", "team\\bad"}
	for _, id := range cases {
		p := validProfile()
		p.Profile.ID = id
		if err := Validate(p, nil); err == nil {
			t.Errorf("expected error for ID with path traversal: %q", id)
		}
	}
}

func TestValidate_ShellMetacharsInSyncRepo(t *testing.T) {
	dangerous := []string{
		"git@github.com:org/repo.git; rm -rf /",
		"git@github.com:org/repo.git | cat /etc/passwd",
		"git@github.com:org/repo.git`id`",
		"git@github.com:org/repo.git$(whoami)",
	}
	for _, repo := range dangerous {
		p := validProfile()
		p.Overrides.Vault = &config.VaultOverride{SyncRepo: repo}
		if err := Validate(p, nil); err == nil {
			t.Errorf("expected error for dangerous sync_repo: %q", repo)
		}
	}
}

func TestValidate_SafeSyncRepo(t *testing.T) {
	safe := []string{
		"git@github.com:org/korva-vault-sync.git",
		"https://github.com/org/korva-vault-sync.git",
		"ssh://git@bitbucket.org/org/repo.git",
	}
	for _, repo := range safe {
		p := validProfile()
		p.Overrides.Vault = &config.VaultOverride{SyncRepo: repo}
		if err := Validate(p, nil); err != nil {
			t.Errorf("unexpected error for safe sync_repo %q: %v", repo, err)
		}
	}
}

func TestValidate_InvalidMergeStrategy(t *testing.T) {
	p := validProfile()
	p.Overrides.Instructions = &config.InstructionsOverride{MergeStrategy: "overwrite"}
	if err := Validate(p, nil); err == nil {
		t.Error("expected error for invalid merge_strategy")
	}
}

func TestValidate_ValidMergeStrategies(t *testing.T) {
	for _, strat := range []string{"append", "replace", ""} {
		p := validProfile()
		p.Overrides.Instructions = &config.InstructionsOverride{MergeStrategy: strat}
		if err := Validate(p, nil); err != nil {
			t.Errorf("unexpected error for merge_strategy %q: %v", strat, err)
		}
	}
}

// ---------------------------------------------------------------------------
// ApplyOverrides — overlay.go
// ---------------------------------------------------------------------------

func TestApplyOverrides_NilOverrides(t *testing.T) {
	base := config.DefaultConfig()
	base.Project = "my-project"
	result := ApplyOverrides(base, validProfile())

	// Nothing should change
	if result.Project != "my-project" {
		t.Errorf("unexpected project change: %q", result.Project)
	}
	if result.Vault.Port != base.Vault.Port {
		t.Errorf("vault port changed unexpectedly")
	}
}

func TestApplyOverrides_VaultSyncRepo(t *testing.T) {
	base := config.DefaultConfig()
	p := validProfile()
	p.Overrides.Vault = &config.VaultOverride{
		SyncRepo:   "git@github.com:org/vault-sync.git",
		SyncBranch: "main",
	}

	result := ApplyOverrides(base, p)
	if result.Vault.SyncRepo != "git@github.com:org/vault-sync.git" {
		t.Errorf("sync_repo not applied: %q", result.Vault.SyncRepo)
	}
	if result.Vault.SyncBranch != "main" {
		t.Errorf("sync_branch not applied: %q", result.Vault.SyncBranch)
	}
}

func TestApplyOverrides_VaultAutoSync(t *testing.T) {
	base := config.DefaultConfig()
	base.Vault.AutoSync = false

	p := validProfile()
	p.Overrides.Vault = &config.VaultOverride{
		AutoSync:        boolPtr(true),
		SyncIntervalMin: intPtr(30),
	}

	result := ApplyOverrides(base, p)
	if !result.Vault.AutoSync {
		t.Error("auto_sync should be true")
	}
	if result.Vault.SyncIntervalMin != 30 {
		t.Errorf("sync_interval_minutes should be 30, got %d", result.Vault.SyncIntervalMin)
	}
}

func TestApplyOverrides_PrivatePatternsAreAppended(t *testing.T) {
	base := config.DefaultConfig()
	baseCount := len(base.Vault.PrivatePatterns)

	// Use patterns that are NOT in DefaultConfig to avoid deduplication
	p := validProfile()
	p.Overrides.Vault = &config.VaultOverride{
		PrivatePatterns: []string{"consumerKey", "consumerSecret", "fif.tech"},
	}

	result := ApplyOverrides(base, p)
	if len(result.Vault.PrivatePatterns) != baseCount+3 {
		t.Errorf("expected %d patterns, got %d", baseCount+3, len(result.Vault.PrivatePatterns))
	}
}

func TestApplyOverrides_PrivatePatternsDeduplicated(t *testing.T) {
	base := config.DefaultConfig()
	base.Vault.PrivatePatterns = []string{"password", "token"}

	p := validProfile()
	// "password" already exists — should not be doubled
	p.Overrides.Vault = &config.VaultOverride{
		PrivatePatterns: []string{"password", "ROLE_ID"},
	}

	result := ApplyOverrides(base, p)
	seen := make(map[string]int)
	for _, pat := range result.Vault.PrivatePatterns {
		seen[pat]++
	}
	if seen["password"] > 1 {
		t.Errorf("'password' duplicated in private_patterns: count=%d", seen["password"])
	}
}

func TestApplyOverrides_SentinelOverrides(t *testing.T) {
	base := config.DefaultConfig()
	base.Sentinel.BlockOnViolation = false

	p := validProfile()
	p.Overrides.Sentinel = &config.SentinelOverride{
		RulesPath:        "sentinel/my-rules.md",
		BlockOnViolation: boolPtr(true),
		Hooks:            []string{"pre-commit", "pre-push"},
	}

	result := ApplyOverrides(base, p)
	if result.Sentinel.RulesPath != "sentinel/my-rules.md" {
		t.Errorf("rules_path not applied: %q", result.Sentinel.RulesPath)
	}
	if !result.Sentinel.BlockOnViolation {
		t.Error("block_on_violation should be true")
	}
	if len(result.Sentinel.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(result.Sentinel.Hooks))
	}
}

func TestApplyOverrides_LoreScrollsMerged(t *testing.T) {
	base := config.DefaultConfig()
	base.Lore.ActiveScrolls = []string{"forge-sdd"}

	p := validProfile()
	p.Overrides.Lore = &config.LoreOverride{
		ScrollPriority: "private_first",
		ActiveScrolls:  []string{"nestjs-hexagonal", "apigee-fif"},
	}

	result := ApplyOverrides(base, p)

	// Team scrolls come first, base scrolls appended (no duplicates)
	if result.Lore.ScrollPriority != "private_first" {
		t.Errorf("scroll_priority not applied: %q", result.Lore.ScrollPriority)
	}
	if len(result.Lore.ActiveScrolls) != 3 {
		t.Errorf("expected 3 scrolls (2 team + 1 base), got %d: %v",
			len(result.Lore.ActiveScrolls), result.Lore.ActiveScrolls)
	}
	if result.Lore.ActiveScrolls[0] != "nestjs-hexagonal" {
		t.Errorf("team scrolls should come first, got %q", result.Lore.ActiveScrolls[0])
	}
}

func TestApplyOverrides_LoreScrollsDeduplicated(t *testing.T) {
	base := config.DefaultConfig()
	base.Lore.ActiveScrolls = []string{"forge-sdd", "typescript"}

	p := validProfile()
	// "forge-sdd" is in both base and team — should appear once
	p.Overrides.Lore = &config.LoreOverride{
		ActiveScrolls: []string{"forge-sdd", "nestjs-hexagonal"},
	}

	result := ApplyOverrides(base, p)
	seen := make(map[string]int)
	for _, s := range result.Lore.ActiveScrolls {
		seen[s]++
	}
	if seen["forge-sdd"] > 1 {
		t.Errorf("'forge-sdd' duplicated in active_scrolls: count=%d", seen["forge-sdd"])
	}
}

func TestApplyOverrides_DoesNotMutateBase(t *testing.T) {
	base := config.DefaultConfig()
	original := base.Vault.SyncRepo

	p := validProfile()
	p.Overrides.Vault = &config.VaultOverride{SyncRepo: "git@github.com:org/new.git"}

	_ = ApplyOverrides(base, p)

	// base must be unchanged
	if base.Vault.SyncRepo != original {
		t.Errorf("ApplyOverrides mutated the base config (sync_repo changed)")
	}
}
