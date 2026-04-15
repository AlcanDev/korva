package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_DefaultsWhenFileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/path/korva.config.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	def := DefaultConfig()
	if cfg.Vault.Port != def.Vault.Port {
		t.Errorf("expected default port %d, got %d", def.Vault.Port, cfg.Vault.Port)
	}
	if cfg.Sentinel.Enabled != def.Sentinel.Enabled {
		t.Errorf("expected default sentinel.enabled=%v, got %v", def.Sentinel.Enabled, cfg.Sentinel.Enabled)
	}
}

func TestLoad_ReadsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "korva.config.json")

	cfg := KorvaConfig{
		Version: "1",
		Project: "my-project",
		Team:    "my-team",
		Country: "PE",
		Vault: VaultConfig{
			Port:      8000,
			AutoStart: false,
		},
		Agent: "claude",
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Project != "my-project" {
		t.Errorf("project: expected 'my-project', got %q", loaded.Project)
	}
	if loaded.Country != "PE" {
		t.Errorf("country: expected 'PE', got %q", loaded.Country)
	}
	if loaded.Vault.Port != 8000 {
		t.Errorf("vault.port: expected 8000, got %d", loaded.Vault.Port)
	}
	if loaded.Agent != "claude" {
		t.Errorf("agent: expected 'claude', got %q", loaded.Agent)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid json}"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoad_PartialConfig_DefaultsApplied(t *testing.T) {
	// JSON with only project set — other fields should get defaults from DefaultConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.json")
	os.WriteFile(path, []byte(`{"project": "partial", "version": "1"}`), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Project != "partial" {
		t.Errorf("expected 'partial', got %q", cfg.Project)
	}
	// Vault port should still be the default
	if cfg.Vault.Port != DefaultConfig().Vault.Port {
		t.Errorf("vault.port should be default %d, got %d", DefaultConfig().Vault.Port, cfg.Vault.Port)
	}
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

func TestSave_WritesReadableJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	cfg := DefaultConfig()
	cfg.Project = "save-test"

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File must exist and be valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("saved file is not valid JSON")
	}

	var decoded KorvaConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decoding saved file: %v", err)
	}
	if decoded.Project != "save-test" {
		t.Errorf("project not persisted: %q", decoded.Project)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.json")

	original := DefaultConfig()
	original.Project = "round-trip"
	original.Team = "the-team"
	original.Vault.Port = 9000
	original.Vault.AutoSync = true
	original.Lore.ActiveScrolls = []string{"nestjs-hexagonal", "forge-sdd"}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}

	if loaded.Team != "the-team" {
		t.Errorf("team not persisted: %q", loaded.Team)
	}
	if loaded.Vault.Port != 9000 {
		t.Errorf("vault.port not persisted: %d", loaded.Vault.Port)
	}
	if !loaded.Vault.AutoSync {
		t.Error("vault.auto_sync not persisted")
	}
	if len(loaded.Lore.ActiveScrolls) != 2 {
		t.Errorf("active_scrolls not persisted: %v", loaded.Lore.ActiveScrolls)
	}
}

func TestSave_ErrorOnBadPath(t *testing.T) {
	err := Save(DefaultConfig(), "/nonexistent/deep/dir/config.json")
	if err == nil {
		t.Error("expected error saving to nonexistent path")
	}
}

// ---------------------------------------------------------------------------
// LoadTeamProfile
// ---------------------------------------------------------------------------

func TestLoadTeamProfile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team-profile.json")

	profile := TeamProfile{
		Profile: ProfileMeta{
			ID:      "test-team",
			Version: "1.0.0",
			Team:    "Test Team",
		},
	}
	data, _ := json.MarshalIndent(profile, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := LoadTeamProfile(path)
	if err != nil {
		t.Fatalf("LoadTeamProfile: %v", err)
	}
	if loaded.Profile.ID != "test-team" {
		t.Errorf("profile.id: expected 'test-team', got %q", loaded.Profile.ID)
	}
}

func TestLoadTeamProfile_FileNotFound(t *testing.T) {
	_, err := LoadTeamProfile("/nonexistent/team-profile.json")
	if err == nil {
		t.Error("expected error for missing profile file")
	}
}

func TestLoadTeamProfile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadTeamProfile(path)
	if err == nil {
		t.Error("expected error for invalid JSON in team profile")
	}
}

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfig_HasSensibleValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Vault.Port == 0 {
		t.Error("default vault port should not be 0")
	}
	if !cfg.Sentinel.Enabled {
		t.Error("sentinel should be enabled by default")
	}
	if len(cfg.Sentinel.Hooks) == 0 {
		t.Error("sentinel hooks should have at least one entry by default")
	}
	if len(cfg.Vault.PrivatePatterns) == 0 {
		t.Error("private_patterns should have defaults")
	}
	if cfg.Lore.ScrollPriority == "" {
		t.Error("scroll_priority should have a default value")
	}
}
