package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot finds the repository root by walking up from the current package
// until it hits the directory containing go.work. We intentionally don't use
// runtime.Caller because the test should fail fast if the layout changes.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root (go.work) starting from %s", dir)
	return ""
}

// TestBehaviorMDExists guards against the canonical guidelines file being
// removed or renamed without updating its references.
func TestBehaviorMDExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "BEHAVIOR.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("BEHAVIOR.md missing at repo root: %v", err)
	}

	// Sanity: each of the four canonical principles must be present so we
	// notice if someone gutted the file. Headings exist as section numbers.
	required := []string{
		"Think before coding",
		"Simplicity first",
		"Surgical changes",
		"Goal-driven execution",
	}
	for _, want := range required {
		if !strings.Contains(string(body), want) {
			t.Errorf("BEHAVIOR.md missing required principle %q", want)
		}
	}
}

// TestRootCLAUDELinksBehavior ensures the root CLAUDE.md references BEHAVIOR.md
// so AI agents that load CLAUDE.md discover the behavioral discipline.
func TestRootCLAUDELinksBehavior(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md missing: %v", err)
	}
	if !strings.Contains(string(body), "BEHAVIOR.md") {
		t.Error("CLAUDE.md should reference BEHAVIOR.md so agents discover the guidelines")
	}
}

// TestIntegrationsReferenceBehavior verifies that each IDE manifest under
// integrations/* references BEHAVIOR.md. Without this anti-drift test it is
// easy for an integration to fall out of sync when guidelines evolve.
func TestIntegrationsReferenceBehavior(t *testing.T) {
	root := repoRoot(t)

	// Map: relative path → human-friendly IDE name (used in error messages).
	manifests := map[string]string{
		"integrations/README.md":                       "integrations index",
		"integrations/claude-code/CLAUDE.md":           "Claude Code",
		"integrations/cursor/.cursorrules":             "Cursor",
		"integrations/windsurf/global_rules.md":        "Windsurf",
		"integrations/codex/.codex-plugin.json":        "OpenAI Codex",
		"integrations/copilot/copilot-instructions.md": "GitHub Copilot",
		"integrations/opencode/opencode.json":          "OpenCode",
		"integrations/gemini/GEMINI.md":                "Gemini CLI",
		"integrations/vscode/settings.json":            "VS Code",
	}

	for rel, name := range manifests {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("missing manifest %s: %v", rel, err)
			}
			content := string(body)
			// Accept either an explicit "BEHAVIOR.md" mention or a verbatim
			// embedding of one of the four principle headings.
			if !strings.Contains(content, "BEHAVIOR.md") &&
				!strings.Contains(content, "think before coding") &&
				!strings.Contains(content, "Think before coding") {
				t.Errorf("%s manifest (%s) does not reference BEHAVIOR.md or its principles — agents using this IDE will miss the discipline", name, rel)
			}
		})
	}
}
