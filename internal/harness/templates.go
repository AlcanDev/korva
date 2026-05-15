package harness

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Templates that `korva harness init` materializes into a repo.
//
// Layers:
//
//	templates/common/         AGENTS.md, CHECKPOINTS.md, progress/*.md
//	templates/<stack>/        per-stack init.sh + docs/*.md
//	                          stacks: go, typescript, python, generic
//	templates/editors/<id>/   per-editor rule files (claude, cursor,
//	                          windsurf, continue, copilot)
//
// `all:` is required so dotfile-prefixed paths (`.claude/agents/...`,
// `.cursor/rules/...`, `.windsurf/rules/...`) are included; `//go:embed
// templates` alone would silently skip them.
//
//go:embed all:templates
var templateFS embed.FS

// Stack picks the language/runtime preset.
type Stack string

const (
	StackGo      Stack = "go"
	StackTS      Stack = "typescript"
	StackPython  Stack = "python"
	StackGeneric Stack = "generic"
)

// AllStacks lists every preset in the order the CLI prints them.
var AllStacks = []Stack{StackGo, StackTS, StackPython, StackGeneric}

// Editor identifies an AI editor whose rule format we ship a template for.
// Adding a new editor is two steps: append a constant + AllEditors entry,
// and add an `editorSpec` row below describing its on-disk markers.
type Editor string

const (
	EditorClaude   Editor = "claude"
	EditorCursor   Editor = "cursor"
	EditorWindsurf Editor = "windsurf"
	EditorContinue Editor = "continue"
	EditorCopilot  Editor = "copilot"
	EditorAider    Editor = "aider"
	EditorCodex    Editor = "codex"
)

// AllEditors is the canonical list — also the menu the CLI prints in help.
var AllEditors = []Editor{
	EditorClaude,
	EditorCursor,
	EditorWindsurf,
	EditorContinue,
	EditorCopilot,
	EditorAider,
	EditorCodex,
}

// editorSpec describes one supported editor. `markers` are paths whose
// presence in the target repo indicates the user already uses this editor
// — DetectEditors walks them to auto-pick presets.
type editorSpec struct {
	id      Editor
	markers []string // anything matched here → editor is present
}

// editorSpecs is the detection table. Markers are repo-local paths
// whose presence indicates the operator already uses that editor.
// AGENTS.md is intentionally NOT a marker — it's the agent-agnostic
// universal file that ships with every harness, so its presence
// proves nothing about which editor materialized it.
var editorSpecs = []editorSpec{
	{id: EditorClaude, markers: []string{".claude", "CLAUDE.md"}},
	{id: EditorCursor, markers: []string{".cursor", ".cursorrules"}},
	{id: EditorWindsurf, markers: []string{".windsurf", ".windsurfrules"}},
	{id: EditorContinue, markers: []string{".continue", ".continuerules"}},
	{id: EditorCopilot, markers: []string{".github/copilot-instructions.md"}},
	{id: EditorAider, markers: []string{".aider.conf.yml", ".aider.conf", ".aiderignore", "CONVENTIONS.md"}},
	{id: EditorCodex, markers: []string{".codex", ".codex/config.toml"}},
}

// IsKnownEditor reports whether s names one of the editors we ship a
// template for. Used by the CLI / MCP arg validators.
func IsKnownEditor(s Editor) bool {
	for _, e := range AllEditors {
		if e == s {
			return true
		}
	}
	return false
}

// DetectEditors inspects `root` for editor-specific marker files and
// returns the editors that appear to be in use. When none match it
// returns []Editor{EditorClaude} as the default — Korva's primary editor
// and the safest fallback. Order matches AllEditors so the output is
// stable across calls.
func DetectEditors(root string) []Editor {
	hits := DetectEditorsDetailed(root)
	if len(hits) == 0 {
		return []Editor{EditorClaude}
	}
	out := make([]Editor, len(hits))
	for i, h := range hits {
		out[i] = h.Editor
	}
	return out
}

// DetectionHit pairs an editor with the on-disk marker that triggered
// its detection. `korva harness detect` surfaces the marker so the
// operator can audit why each editor was picked.
type DetectionHit struct {
	Editor Editor
	// Marker is the repo-relative path (slash-separated, the same form
	// as in editorSpecs) whose existence proved the editor present.
	Marker string
}

// DetectEditorsDetailed returns the same set of editors as
// DetectEditors but also names which marker matched. Unlike
// DetectEditors it does NOT inject EditorClaude as a fallback when
// nothing matched — callers that want the fallback behavior should
// use DetectEditors instead. This keeps the detect command honest:
// "no markers found" surfaces clearly rather than masquerading as a
// real Claude install.
func DetectEditorsDetailed(root string) []DetectionHit {
	var hits []DetectionHit
	for _, spec := range editorSpecs {
		for _, m := range spec.markers {
			if exists(filepath.Join(root, filepath.FromSlash(m))) {
				hits = append(hits, DetectionHit{Editor: spec.id, Marker: m})
				break
			}
		}
	}
	return hits
}

// EditorFiles returns the slash-separated relative paths that
// `Generate` would write for editor e (universal layer excluded —
// AGENTS.md / CHECKPOINTS.md are returned by CommonFiles instead).
// In SDD mode the optional spec_author files are included; pass
// sdd=false to see the non-SDD install set.
//
// Returns nil + non-nil error when e is unknown. Returns an empty
// slice (and nil error) for editors whose template tree is empty.
func EditorFiles(e Editor, sdd bool) ([]string, error) {
	if !IsKnownEditor(e) {
		return nil, fmt.Errorf("unknown editor %q", e)
	}
	files, err := listTemplateTree("templates/editors/" + string(e))
	if err != nil {
		return nil, err
	}
	if sdd {
		extra, err := listTemplateTree("templates/editors-sdd/" + string(e))
		if err == nil {
			files = append(files, extra...)
		}
	}
	return files, nil
}

// CommonFiles returns the slash-separated relative paths every
// harness installs regardless of editor (AGENTS.md, CHECKPOINTS.md,
// progress/*). Useful for the detect command's "would install"
// summary.
func CommonFiles() ([]string, error) {
	return listTemplateTree("templates/common")
}

// listTemplateTree walks the embedded template FS and returns the
// destination paths (with .tmpl stripped) under fsDir. Returns an
// empty slice (and nil error) when fsDir doesn't exist in the
// embed, so callers can probe optional trees without a stat guard.
// Any other read failure is surfaced as a real error.
func listTemplateTree(fsDir string) ([]string, error) {
	if _, err := templateFS.ReadDir(fsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	err := fs.WalkDir(templateFS, fsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, fsDir+"/")
		out = append(out, strings.TrimSuffix(rel, ".tmpl"))
		return nil
	})
	return out, err
}

// InitOptions controls what `korva harness init` produces.
type InitOptions struct {
	Root        string   // target directory
	Project     string   // project name (goes into AGENTS.md + feature_list.json)
	Description string   // short blurb
	Stack       Stack    // chosen preset; empty falls back to Generic
	Editors     []Editor // editor rule templates to install; empty installs none
	SDD         bool     // enable Spec-Driven Development mode (Phase 13)
	// RequireApprovedReview is the opt-in Phase 19.C tightening of
	// SDD: an SDD feature can't transition spec_ready → in_progress
	// without an approved review on record. Only consulted when SDD
	// is true; harmless otherwise. The rule is persisted to
	// feature_list.json so it survives `korva harness check`.
	RequireApprovedReview bool
	Overwrite             bool // when false (default) we refuse to overwrite existing files
}

// Generate materializes every template file under opts.Root.
// Returns the list of files actually written (so the CLI can print them).
func Generate(opts InitOptions) ([]string, error) {
	if opts.Root == "" {
		return nil, fmt.Errorf("root is required")
	}
	if strings.TrimSpace(opts.Project) == "" {
		return nil, fmt.Errorf("project is required")
	}
	stack := opts.Stack
	if stack == "" {
		stack = StackGeneric
	}
	stackDir := "templates/" + string(stack)
	if _, err := templateFS.ReadDir(stackDir); err != nil {
		return nil, fmt.Errorf("unknown stack %q: %w", stack, err)
	}
	// Validate editor list up front so we don't write half a harness and
	// then bail out.
	for _, e := range opts.Editors {
		if !IsKnownEditor(e) {
			return nil, fmt.Errorf("unknown editor %q: pick one of %s", e, joinEditors(AllEditors))
		}
	}

	written := make([]string, 0, 16)

	// 1. Per-stack tree.
	if err := walkAndWrite(stackDir, opts, &written); err != nil {
		return nil, err
	}

	// 2. Common files (AGENTS.md, CHECKPOINTS.md, progress/*.md) shared
	// across stacks.
	if err := walkAndWrite("templates/common", opts, &written); err != nil {
		return nil, err
	}

	// 3. feature_list.json — generated from defaults so the seed matches
	// the runtime types exactly.
	flPath := filepath.Join(opts.Root, FeatureListPath)
	if !exists(flPath) || opts.Overwrite {
		seed := buildSeedFeatureList(opts)
		if err := SaveFeatureList(opts.Root, seed); err != nil {
			return nil, fmt.Errorf("seed feature list: %w", err)
		}
		written = append(written, FeatureListPath)
	}

	// 4. Per-editor rule files. Empty Editors slice → no editor-specific
	// files (universal layer above is enough for plain-text editors and
	// any MCP-only client).
	for _, e := range opts.Editors {
		if err := walkAndWrite("templates/editors/"+string(e), opts, &written); err != nil {
			return nil, err
		}
		// SDD adds an extra spec_author rule file for editors with a
		// multi-file convention (claude, cursor, windsurf). Single-file
		// editors (continue, copilot) get no separate file — their one
		// rules document already covers the harness flow; the SDD steps
		// are universal CLI/MCP verbs an agent uses regardless.
		if opts.SDD {
			sddEditorDir := "templates/editors-sdd/" + string(e)
			if _, err := templateFS.ReadDir(sddEditorDir); err == nil {
				if err := walkAndWrite(sddEditorDir, opts, &written); err != nil {
					return nil, err
				}
			}
		}
	}

	// 5. SDD spec templates — only when --sdd is set. They live under
	// templates/sdd/ and materialize into specs/SPEC-TEMPLATE/* so a
	// human can copy them per feature without re-typing the EARS
	// scaffolding.
	if opts.SDD {
		if err := walkAndWrite("templates/sdd", opts, &written); err != nil {
			return nil, err
		}
	}

	return written, nil
}

// buildSeedFeatureList produces the smoke feature list used to bootstrap
// a fresh harness. In SDD mode the seed feature is `sdd: true` so the
// new operator immediately exercises the spec workflow.
func buildSeedFeatureList(opts InitOptions) *FeatureList {
	rules := DefaultRules()
	smokeAcceptance := []string{
		"`./init.sh` exits with code 0",
		"`feature_list.json` validates",
		"`progress/current.md` exists",
	}
	smokeDescription := "First feature in every new harness: run `./init.sh` and confirm it exits 0. " +
		"Once the smoke passes, replace this feature with the real backlog."

	if opts.SDD {
		rules = SDDRules()
		smokeAcceptance = append(smokeAcceptance,
			"`specs/harness_smoke/{requirements,design,tasks}.md` exist",
		)
		smokeDescription += " In SDD mode this feature also exercises the spec workflow — " +
			"draft the three spec files, run `korva harness ready 1`, and a human approves " +
			"the spec by transitioning to in_progress."
	}
	// Phase 19.C — flip on the review-gated transition when the
	// operator opts in. Independent of SDD-vs-not because it's a
	// stricter version of the same workflow; we still gate it on
	// `f.SDD == true` at the state-machine level so non-SDD
	// features are unaffected.
	if opts.RequireApprovedReview {
		rules.RequireApprovedReview = true
	}

	return &FeatureList{
		Project:     opts.Project,
		Description: opts.Description,
		Rules:       rules,
		Features: []Feature{
			{
				ID:          1,
				Name:        "harness_smoke",
				Title:       "Verify the harness is wired correctly",
				Description: smokeDescription,
				Acceptance:  smokeAcceptance,
				Status:      StatusPending,
				SDD:         opts.SDD,
			},
		},
	}
}

// joinEditors stringifies a slice for error messages / help text.
func joinEditors(editors []Editor) string {
	parts := make([]string, 0, len(editors))
	for _, e := range editors {
		parts = append(parts, string(e))
	}
	return strings.Join(parts, ", ")
}

// walkAndWrite copies every file under fsDir into opts.Root, processing
// `.tmpl` files through text/template. Skips writes that would overwrite
// an existing file unless opts.Overwrite is set.
func walkAndWrite(fsDir string, opts InitOptions, written *[]string) error {
	tmplVars := templateVars(opts)
	return fs.WalkDir(templateFS, fsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// embed.FS paths use forward slashes on every OS. Stay in slash-land
		// for the logical `outRel` (so the `written` slice and template names
		// look the same on Linux/macOS/Windows) and only switch to OS-native
		// separators when materializing the destination file.
		rel := strings.TrimPrefix(p, fsDir+"/")
		// Strip the .tmpl suffix used to mark template files.
		outRel := strings.TrimSuffix(rel, ".tmpl")
		outPath := filepath.Join(opts.Root, filepath.FromSlash(outRel))
		if exists(outPath) && !opts.Overwrite {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", outPath, err)
		}
		content, err := templateFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", p, err)
		}
		// Process only .tmpl files; raw .md/.sh files are written verbatim
		// so `{{` in code samples doesn't get interpreted.
		if strings.HasSuffix(rel, ".tmpl") {
			t, err := template.New(rel).Parse(string(content))
			if err != nil {
				return fmt.Errorf("parse template %s: %w", rel, err)
			}
			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create %s: %w", outPath, err)
			}
			if err := t.Execute(f, tmplVars); err != nil {
				_ = f.Close()
				return fmt.Errorf("render %s: %w", rel, err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(outPath, content, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
		}
		// init.sh must be executable.
		if filepath.Base(outPath) == "init.sh" {
			_ = os.Chmod(outPath, 0o755)
		}
		*written = append(*written, outRel)
		return nil
	})
}

// templateVars feeds the text/template engine the variables every
// template file may reference. Returning `any` (rather than `string`)
// lets templates use `{{if .SDDMode}}` for conditional sections without
// the empty-string-is-falsy hack.
func templateVars(opts InitOptions) map[string]any {
	return map[string]any{
		"Project":     opts.Project,
		"Description": opts.Description,
		"Stack":       string(opts.Stack),
		"SDDMode":     opts.SDD,
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
