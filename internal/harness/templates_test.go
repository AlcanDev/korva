package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// commonFiles are the files every stack must produce regardless of preset.
var commonFiles = []string{
	"AGENTS.md",
	"CHECKPOINTS.md",
	"feature_list.json",
	"init.sh",
	"progress/current.md",
	"progress/history.md",
	"docs/architecture.md",
	"docs/conventions.md",
	"docs/verification.md",
}

func TestGenerate_RejectsEmptyRoot(t *testing.T) {
	if _, err := Generate(InitOptions{Project: "x"}); err == nil {
		t.Error("expected error when Root is empty")
	}
}

func TestGenerate_RejectsEmptyProject(t *testing.T) {
	if _, err := Generate(InitOptions{Root: t.TempDir()}); err == nil {
		t.Error("expected error when Project is empty")
	}
}

func TestGenerate_RejectsUnknownStack(t *testing.T) {
	_, err := Generate(InitOptions{Root: t.TempDir(), Project: "x", Stack: Stack("rust")})
	if err == nil || !strings.Contains(err.Error(), "unknown stack") {
		t.Errorf("expected unknown-stack error, got %v", err)
	}
}

func TestGenerate_AllStacksProduceCommonFiles(t *testing.T) {
	for _, s := range AllStacks {
		s := s
		t.Run(string(s), func(t *testing.T) {
			dir := t.TempDir()
			written, err := Generate(InitOptions{
				Root:        dir,
				Project:     "harness-test",
				Description: "smoke",
				Stack:       s,
			})
			if err != nil {
				t.Fatalf("generate %s: %v", s, err)
			}
			for _, f := range commonFiles {
				if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
					t.Errorf("missing %s: %v", f, err)
				}
				if !slices.Contains(written, f) {
					t.Errorf("Generate did not report %s in written list", f)
				}
			}
		})
	}
}

func TestGenerate_TemplateVarsAreSubstituted(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:        dir,
		Project:     "korva-demo",
		Description: "demo run",
		Stack:       StackGo,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "korva-demo") {
		t.Errorf("AGENTS.md missing project substitution: %s", body)
	}
	if strings.Contains(body, "{{.Project}}") {
		t.Error("AGENTS.md still has unrendered template directive")
	}
}

func TestGenerate_SeedsFeatureList(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:    dir,
		Project: "harness-test",
		Stack:   StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "feature_list.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var fl FeatureList
	if err := json.Unmarshal(data, &fl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fl.Project != "harness-test" {
		t.Errorf("Project = %q, want harness-test", fl.Project)
	}
	if len(fl.Features) != 1 || fl.Features[0].Name != "harness_smoke" {
		t.Errorf("seed feature wrong: %+v", fl.Features)
	}
	if fl.Features[0].Status != StatusPending {
		t.Errorf("seed status = %q, want pending", fl.Features[0].Status)
	}
}

func TestGenerate_DoesNotOverwriteByDefault(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed AGENTS.md with custom content.
	custom := "PRESERVED"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(custom), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Generate(InitOptions{
		Root:    dir,
		Project: "x",
		Stack:   StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != custom {
		t.Errorf("AGENTS.md was overwritten: %s", string(got))
	}
}

func TestGenerate_OverwriteFlagReplacesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("OLD"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Generate(InitOptions{
		Root:      dir,
		Project:   "x",
		Stack:     StackGeneric,
		Overwrite: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) == "OLD" {
		t.Error("AGENTS.md was NOT overwritten despite Overwrite=true")
	}
}

func TestGenerate_InitShIsExecutable(t *testing.T) {
	// Unix-only: Windows file modes don't carry executable bits the same way.
	if runtime.GOOS == "windows" {
		t.Skip("file mode semantics differ on Windows")
	}
	dir := t.TempDir()
	if _, err := Generate(InitOptions{Root: dir, Project: "x", Stack: StackGo}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	st, err := os.Stat(filepath.Join(dir, "init.sh"))
	if err != nil {
		t.Fatalf("stat init.sh: %v", err)
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Errorf("init.sh is not executable: mode=%v", st.Mode().Perm())
	}
}

func TestGenerate_WithSubagentsCreatesAgentFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:          dir,
		Project:       "x",
		Stack:         StackGeneric,
		WithSubagents: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, name := range []string{"leader.md", "implementer.md", "reviewer.md"} {
		path := filepath.Join(dir, ".claude", "agents", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing subagent file %s: %v", name, err)
		}
	}
}

func TestGenerate_WithoutSubagentsSkipsAgentFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:    dir,
		Project: "x",
		Stack:   StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "leader.md")); !os.IsNotExist(err) {
		t.Errorf("expected no subagent files when WithSubagents=false, got err=%v", err)
	}
}

func TestGenerate_FeatureListSurvivesValidate(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{Root: dir, Project: "x", Stack: StackPython}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := LoadFeatureList(dir); err != nil {
		t.Errorf("LoadFeatureList rejected the seed: %v", err)
	}
}
