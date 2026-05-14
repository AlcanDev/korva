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

func TestGenerate_EditorClaude_CreatesAgentFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:    dir,
		Project: "x",
		Stack:   StackGeneric,
		Editors: []Editor{EditorClaude},
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, name := range []string{"leader.md", "implementer.md", "reviewer.md"} {
		p := filepath.Join(dir, ".claude", "agents", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing subagent file %s: %v", name, err)
		}
	}
}

func TestGenerate_NoEditors_SkipsAllEditorFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root:    dir,
		Project: "x",
		Stack:   StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	// None of the editor-specific files should be created when Editors is nil.
	for _, p := range []string{
		filepath.Join(dir, ".claude", "agents", "leader.md"),
		filepath.Join(dir, ".cursor", "rules", "korva-harness.mdc"),
		filepath.Join(dir, ".windsurf", "rules", "korva-harness.md"),
		filepath.Join(dir, ".continuerules"),
		filepath.Join(dir, ".github", "copilot-instructions.md"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected no file at %s, got err=%v", p, err)
		}
	}
}

func TestGenerate_PerEditor_CreatesExpectedFile(t *testing.T) {
	cases := map[Editor][]string{
		EditorClaude:   {".claude/agents/leader.md", ".claude/agents/implementer.md", ".claude/agents/reviewer.md"},
		EditorCursor:   {".cursor/rules/korva-harness.mdc"},
		EditorWindsurf: {".windsurf/rules/korva-harness.md"},
		EditorContinue: {".continuerules"},
		EditorCopilot:  {".github/copilot-instructions.md"},
	}
	for editor, paths := range cases {
		editor, paths := editor, paths
		t.Run(string(editor), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Generate(InitOptions{
				Root: dir, Project: "x", Stack: StackGeneric, Editors: []Editor{editor},
			}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			for _, p := range paths {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
					t.Errorf("missing %s: %v", p, err)
				}
			}
		})
	}
}

func TestGenerate_MultipleEditors_InstallsAll(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric,
		Editors: []Editor{EditorCursor, EditorContinue, EditorCopilot},
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, p := range []string{
		".cursor/rules/korva-harness.mdc",
		".continuerules",
		".github/copilot-instructions.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	// Claude wasn't requested → no claude files.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "leader.md")); !os.IsNotExist(err) {
		t.Errorf("claude agents created unexpectedly: %v", err)
	}
}

func TestGenerate_UnknownEditor_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric,
		Editors: []Editor{Editor("rustrover")},
	})
	if err == nil {
		t.Fatal("expected error for unknown editor")
	}
	if !strings.Contains(err.Error(), "rustrover") {
		t.Errorf("error should name the bad editor, got %v", err)
	}
	// Universal layer must NOT have been written when validation fails.
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("Generate wrote files before validating editors: %v", err)
	}
}

func TestIsKnownEditor(t *testing.T) {
	for _, e := range AllEditors {
		if !IsKnownEditor(e) {
			t.Errorf("IsKnownEditor(%q) = false, want true", e)
		}
	}
	if IsKnownEditor(Editor("vim")) {
		t.Error("IsKnownEditor(\"vim\") should be false")
	}
}

func TestDetectEditors_EmptyDirDefaultsClaude(t *testing.T) {
	got := DetectEditors(t.TempDir())
	if len(got) != 1 || got[0] != EditorClaude {
		t.Errorf("empty-dir detect = %v, want [claude]", got)
	}
}

func TestDetectEditors_RecognizesMarkers(t *testing.T) {
	cases := []struct {
		name   string
		create []string
		want   []Editor
	}{
		{"cursor dir", []string{".cursor"}, []Editor{EditorCursor}},
		{"cursorrules", []string{".cursorrules"}, []Editor{EditorCursor}},
		{"windsurf", []string{".windsurf"}, []Editor{EditorWindsurf}},
		{"continuerules", []string{".continuerules"}, []Editor{EditorContinue}},
		{"copilot instructions", []string{".github/copilot-instructions.md"}, []Editor{EditorCopilot}},
		{"claude md", []string{"CLAUDE.md"}, []Editor{EditorClaude}},
		{"claude + cursor", []string{".claude", ".cursor"}, []Editor{EditorClaude, EditorCursor}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, m := range tc.create {
				full := filepath.Join(dir, filepath.FromSlash(m))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				// Trailing slash convention isn't relevant here; create as a file
				// for non-dir markers, dir otherwise.
				if strings.HasSuffix(m, "/") || !strings.Contains(filepath.Base(m), ".") {
					if err := os.MkdirAll(full, 0o755); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			got := DetectEditors(dir)
			if len(got) != len(tc.want) {
				t.Fatalf("DetectEditors = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("DetectEditors[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
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

// ───────────────────────── Phase 13.1 — SDD mode ─────────────────────────

func TestGenerate_SDD_MaterializesSpecTemplate(t *testing.T) {
	dir := t.TempDir()
	written, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, p := range []string{
		"specs/SPEC-TEMPLATE/requirements.md",
		"specs/SPEC-TEMPLATE/design.md",
		"specs/SPEC-TEMPLATE/tasks.md",
	} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if _, err := os.Stat(full); err != nil {
			t.Errorf("missing SDD template %s: %v", p, err)
		}
		if !slices.Contains(written, p) {
			t.Errorf("Generate did not report %s in written list", p)
		}
	}
}

func TestGenerate_NonSDD_SkipsSpecTemplate(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, // SDD: false
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "specs")); !os.IsNotExist(err) {
		t.Errorf("specs/ should not exist when SDD is off, err=%v", err)
	}
}

func TestGenerate_SDD_SeedFeatureIsSDDFlagged(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, err := LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !fl.Rules.RequireApprovedSpecToImplement {
		t.Error("SDD harness must seed RequireApprovedSpecToImplement=true")
	}
	if len(fl.Features) != 1 || !fl.Features[0].SDD {
		t.Errorf("seed feature should be sdd=true, got %+v", fl.Features)
	}
	// SDD seed gets an extra acceptance bullet about the spec files.
	want := "specs/harness_smoke"
	found := false
	for _, a := range fl.Features[0].Acceptance {
		if strings.Contains(a, want) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SDD seed acceptance should mention %s, got %v", want, fl.Features[0].Acceptance)
	}
}

func TestGenerate_NonSDD_SeedFeatureIsPlain(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	if fl.Rules.RequireApprovedSpecToImplement {
		t.Error("default harness must not enable SDD rule")
	}
	if fl.Features[0].SDD {
		t.Error("default seed feature must not be sdd=true")
	}
}

func TestGenerate_SDD_AddsSpecAuthorForMultiFileEditors(t *testing.T) {
	// claude, cursor, windsurf each get a spec_author rule file when SDD
	// is on. continue + copilot don't (single-file editors).
	cases := map[Editor]string{
		EditorClaude:   ".claude/agents/spec_author.md",
		EditorCursor:   ".cursor/rules/korva-harness-sdd-spec-author.mdc",
		EditorWindsurf: ".windsurf/rules/korva-harness-sdd-spec-author.md",
	}
	for editor, path := range cases {
		editor, path := editor, path
		t.Run(string(editor), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Generate(InitOptions{
				Root: dir, Project: "x", Stack: StackGeneric,
				Editors: []Editor{editor}, SDD: true,
			}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
				t.Errorf("missing spec_author rule for %s at %s: %v", editor, path, err)
			}
		})
	}
}

func TestGenerate_SDD_NoSpecAuthorForSingleFileEditors(t *testing.T) {
	// continue / copilot's rule files already encode the harness flow.
	// No SDD-specific extra file — the SDD steps are universal CLI/MCP
	// verbs the agent uses regardless of editor.
	for _, e := range []Editor{EditorContinue, EditorCopilot} {
		e := e
		t.Run(string(e), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Generate(InitOptions{
				Root: dir, Project: "x", Stack: StackGeneric,
				Editors: []Editor{e}, SDD: true,
			}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			// No spec_author-named file should exist anywhere under dir.
			// Walk errors aren't expected on a freshly-generated harness,
			// but propagate them anyway so the assertion never silently
			// passes after a permission issue.
			extra := false
			if err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.Contains(filepath.Base(p), "spec-author") {
					extra = true
				}
				return nil
			}); err != nil {
				t.Fatalf("walk: %v", err)
			}
			if extra {
				t.Errorf("editor %s should not get a spec_author file", e)
			}
		})
	}
}

func TestGenerate_NonSDD_NoSpecAuthorForAnyEditor(t *testing.T) {
	for _, e := range AllEditors {
		e := e
		t.Run(string(e), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Generate(InitOptions{
				Root: dir, Project: "x", Stack: StackGeneric,
				Editors: []Editor{e},
			}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			candidates := []string{
				".claude/agents/spec_author.md",
				".cursor/rules/korva-harness-sdd-spec-author.mdc",
				".windsurf/rules/korva-harness-sdd-spec-author.md",
			}
			for _, c := range candidates {
				full := filepath.Join(dir, filepath.FromSlash(c))
				if _, err := os.Stat(full); err == nil {
					t.Errorf("non-SDD harness should not contain %s", c)
				}
			}
		})
	}
}

func TestGenerate_SDD_CheckpointsIncludeC6Section(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "CHECKPOINTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, want := range []string{
		"## C6 — Spec-driven contract upheld",
		"EARS notation",
		"korva harness check",
		"C1-C5/C6", // footer mentions C6 too
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("CHECKPOINTS missing %q\nfull body:\n%s", want, body)
		}
	}
	if strings.Contains(string(body), "{{") {
		t.Errorf("CHECKPOINTS still contains unrendered template tags")
	}
}

func TestGenerate_NonSDD_CheckpointsOmitC6Section(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, // SDD: false
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "CHECKPOINTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(body), "## C6") {
		t.Error("non-SDD CHECKPOINTS should not render the C6 section")
	}
	if strings.Contains(string(body), "C1-C5/C6") {
		t.Error("non-SDD footer should not mention C6")
	}
	if strings.Contains(string(body), "{{") {
		t.Errorf("CHECKPOINTS still contains unrendered template tags")
	}
}

func TestGenerate_SDD_TemplatesRenderProjectVar(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "korva-demo", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Spec templates intentionally don't use {{.Project}} — they're per-
	// feature scaffolding the operator copies — but the rest of the SDD
	// harness (AGENTS.md, etc.) does. Verify the universal layer.
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(data), "korva-demo") {
		t.Errorf("AGENTS.md missing project name with SDD on: %s", string(data))
	}
}
