package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/internal/config"
)

// ---------------------------------------------------------------------------
// MergeInstructions — tests de filesystem puro (sin git)
// ---------------------------------------------------------------------------

func TestMergeInstructions_AppendsCopilotExtensions(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	projectDir := filepath.Join(dir, "project")
	githubDir := filepath.Join(projectDir, ".github")

	if err := os.MkdirAll(filepath.Join(profileDir, "instructions"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(githubDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write extension content
	extContent := "## Team context\n\n### Stack\n- NestJS + Fastify\n"
	os.WriteFile(
		filepath.Join(profileDir, "instructions", "copilot-extensions.md"),
		[]byte(extContent), 0644,
	)

	// Write base copilot-instructions.md
	base := "# Korva\n\n## Architecture rules\n\n<!-- korva:team-extensions:begin -->\n<!-- korva:team-extensions:end -->\n"
	copilotFile := filepath.Join(githubDir, "copilot-instructions.md")
	os.WriteFile(copilotFile, []byte(base), 0644)

	paths := &config.Paths{HomeDir: dir}
	mgr := NewManager(paths)

	profile := validProfile()
	profile.Overrides.Instructions = &config.InstructionsOverride{
		CopilotExtensions: "instructions/copilot-extensions.md",
		MergeStrategy:     "append",
	}

	// Write profile JSON so MergeInstructions can load it
	profileJSON := `{
		"profile": {"id": "test-team", "version": "1.0.0", "team": "Test Team"},
		"overrides": {
			"instructions": {
				"copilot_extensions": "instructions/copilot-extensions.md",
				"merge_strategy": "append"
			}
		}
	}`
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(profileJSON), 0644)

	if err := mgr.MergeInstructions(profileDir, projectDir); err != nil {
		t.Fatalf("MergeInstructions: %v", err)
	}

	result, err := os.ReadFile(copilotFile)
	if err != nil {
		t.Fatalf("reading result file: %v", err)
	}

	out := string(result)
	if !strings.Contains(out, "NestJS + Fastify") {
		t.Errorf("extension content not appended, got:\n%s", out)
	}
	if !strings.Contains(out, "korva:team-extensions:test-team:begin") {
		t.Errorf("begin marker not found in output")
	}
	if !strings.Contains(out, "korva:team-extensions:test-team:end") {
		t.Errorf("end marker not found in output")
	}
}

func TestMergeInstructions_Idempotent(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	projectDir := filepath.Join(dir, "project")
	githubDir := filepath.Join(projectDir, ".github")

	os.MkdirAll(filepath.Join(profileDir, "instructions"), 0755)
	os.MkdirAll(githubDir, 0755)

	extContent := "## Team\n- NestJS\n"
	os.WriteFile(filepath.Join(profileDir, "instructions", "copilot-extensions.md"),
		[]byte(extContent), 0644)

	copilotFile := filepath.Join(githubDir, "copilot-instructions.md")
	os.WriteFile(copilotFile, []byte("# Base\n"), 0644)

	profileJSON := `{"profile":{"id":"idempotent-team","version":"1.0.0","team":"T"},
		"overrides":{"instructions":{"copilot_extensions":"instructions/copilot-extensions.md","merge_strategy":"append"}}}`
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(profileJSON), 0644)

	paths := &config.Paths{HomeDir: dir}
	mgr := NewManager(paths)

	// Run twice
	if err := mgr.MergeInstructions(profileDir, projectDir); err != nil {
		t.Fatalf("first MergeInstructions: %v", err)
	}
	if err := mgr.MergeInstructions(profileDir, projectDir); err != nil {
		t.Fatalf("second MergeInstructions: %v", err)
	}

	result, _ := os.ReadFile(copilotFile)
	out := string(result)

	// "NestJS" should appear exactly once (idempotent replace, not double-append)
	count := strings.Count(out, "NestJS")
	if count != 1 {
		t.Errorf("'NestJS' appeared %d times after 2 runs (expected 1 — idempotent)", count)
	}
}

func TestMergeInstructions_NoExtensions(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	projectDir := filepath.Join(dir, "project")

	os.MkdirAll(profileDir, 0755)
	os.MkdirAll(projectDir, 0755)

	// Profile with no instructions override
	profileJSON := `{"profile":{"id":"no-ext","version":"1.0.0","team":"T"},"overrides":{}}`
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(profileJSON), 0644)

	paths := &config.Paths{HomeDir: dir}
	mgr := NewManager(paths)

	// Should return nil — nothing to do
	if err := mgr.MergeInstructions(profileDir, projectDir); err != nil {
		t.Fatalf("MergeInstructions with no extensions: %v", err)
	}
}

func TestMergeInstructions_MissingExtensionFile(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	projectDir := filepath.Join(dir, "project")

	os.MkdirAll(profileDir, 0755)
	os.MkdirAll(projectDir, 0755)

	// Points to a file that doesn't exist — should NOT error (graceful skip)
	profileJSON := `{"profile":{"id":"skip-test","version":"1.0.0","team":"T"},
		"overrides":{"instructions":{"copilot_extensions":"instructions/missing.md"}}}`
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(profileJSON), 0644)

	paths := &config.Paths{HomeDir: dir}
	mgr := NewManager(paths)

	if err := mgr.MergeInstructions(profileDir, projectDir); err != nil {
		t.Fatalf("should gracefully skip missing extension file, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ActiveProfileID
// ---------------------------------------------------------------------------

func TestActiveProfileID_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	paths := &config.Paths{HomeDir: dir, ConfigFile: filepath.Join(dir, "config.json")}
	mgr := NewManager(paths)

	id, err := mgr.ActiveProfileID()
	if err != nil {
		t.Fatalf("unexpected error when config missing: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty id when no config, got %q", id)
	}
}

func TestActiveProfileID_WithConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.json")
	os.WriteFile(configFile, []byte(`{"_active_profile_id": "my-team"}`), 0644)

	paths := &config.Paths{HomeDir: dir, ConfigFile: configFile}
	mgr := NewManager(paths)

	id, err := mgr.ActiveProfileID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-team" {
		t.Errorf("expected 'my-team', got %q", id)
	}
}

// ---------------------------------------------------------------------------
// Apply
// ---------------------------------------------------------------------------

func TestApply_MergesAndSavesConfig(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	os.MkdirAll(profileDir, 0755)

	profileJSON := `{
		"profile": {"id": "test-apply", "version": "1.0.0", "team": "TestTeam",
		             "source_repo": "https://github.com/test/profile.git"},
		"overrides": {
			"vault": {"sync_repo": "https://github.com/test/vault.git"}
		}
	}`
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(profileJSON), 0644)

	configFile := filepath.Join(dir, "config.json")
	paths := &config.Paths{
		HomeDir:    dir,
		ConfigFile: configFile,
	}
	mgr := NewManager(paths)

	base := config.DefaultConfig()
	merged, err := mgr.Apply(profileDir, base)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if merged.Vault.SyncRepo != "https://github.com/test/vault.git" {
		t.Errorf("vault.sync_repo not applied, got %q", merged.Vault.SyncRepo)
	}

	// Verify it was saved to disk
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Apply should have saved config to disk")
	}
}

func TestApply_InvalidProfile(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	os.MkdirAll(profileDir, 0755)

	// Missing required fields
	os.WriteFile(filepath.Join(profileDir, "team-profile.json"), []byte(`{}`), 0644)

	paths := &config.Paths{HomeDir: dir, ConfigFile: filepath.Join(dir, "config.json")}
	mgr := NewManager(paths)

	_, err := mgr.Apply(profileDir, config.DefaultConfig())
	if err == nil {
		t.Error("Apply with invalid profile should return error")
	}
}

// ---------------------------------------------------------------------------
// InstallScrolls
// ---------------------------------------------------------------------------

func TestInstallScrolls_CopiesScrolls(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	scrollsDir := filepath.Join(profileDir, "scrolls")
	os.MkdirAll(filepath.Join(scrollsDir, "my-scroll"), 0755)
	os.WriteFile(filepath.Join(scrollsDir, "my-scroll", "SCROLL.md"),
		[]byte("# My Scroll\n\nContent here."), 0644)

	loreDir := filepath.Join(dir, "lore")
	paths := &config.Paths{
		HomeDir: dir,
		LoreDir: loreDir,
	}
	mgr := NewManager(paths)

	if err := mgr.InstallScrolls(profileDir); err != nil {
		t.Fatalf("InstallScrolls: %v", err)
	}

	dest := filepath.Join(loreDir, "private", "my-scroll", "SCROLL.md")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Errorf("scroll not installed at %s", dest)
	}
}

func TestInstallScrolls_NoScrollsDir(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profile")
	os.MkdirAll(profileDir, 0755)
	// No scrolls/ subdir

	paths := &config.Paths{HomeDir: dir, LoreDir: filepath.Join(dir, "lore")}
	mgr := NewManager(paths)

	// Should not error — gracefully skip
	if err := mgr.InstallScrolls(profileDir); err != nil {
		t.Fatalf("InstallScrolls with no scrolls dir: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sync — error path (no git, no dir)
// ---------------------------------------------------------------------------

func TestSync_ProfileNotFound(t *testing.T) {
	dir := t.TempDir()
	paths := &config.Paths{
		HomeDir:     dir,
		ProfilesDir: filepath.Join(dir, "profiles"),
		ConfigFile:  filepath.Join(dir, "config.json"),
	}
	mgr := NewManager(paths)

	_, err := mgr.Sync("nonexistent-profile", config.DefaultConfig())
	if err == nil {
		t.Error("Sync on nonexistent profile should return error")
	}
}

// ---------------------------------------------------------------------------
// Clone — error path (invalid URL)
// ---------------------------------------------------------------------------

func TestClone_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	paths := &config.Paths{
		HomeDir:     dir,
		ProfilesDir: filepath.Join(dir, "profiles"),
	}
	mgr := NewManager(paths)

	// Non-existent git URL should fail fast
	_, err := mgr.Clone("https://github.com/nonexistent-999999/no-such-repo-xyz.git")
	if err == nil {
		t.Error("Clone with invalid URL should return error")
	}
}

// ---------------------------------------------------------------------------
// copyDir
// ---------------------------------------------------------------------------

func TestCopyDir_CopiesFilesRecursively(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create nested files
	os.MkdirAll(filepath.Join(src, "subdir"), 0755)
	os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for _, rel := range []string{"root.txt", filepath.Join("subdir", "nested.txt")} {
		if _, err := os.Stat(filepath.Join(dst, rel)); os.IsNotExist(err) {
			t.Errorf("expected %s to be copied to dst", rel)
		}
	}
}
