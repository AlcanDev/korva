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
