package harness

import (
	"embed"
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
)

// AllEditors is the canonical list — also the menu the CLI prints in help.
var AllEditors = []Editor{EditorClaude, EditorCursor, EditorWindsurf, EditorContinue, EditorCopilot}

// editorSpec describes one supported editor. `markers` are paths whose
// presence in the target repo indicates the user already uses this editor
// — DetectEditors walks them to auto-pick presets.
type editorSpec struct {
	id      Editor
	markers []string // anything matched here → editor is present
}

var editorSpecs = []editorSpec{
	{id: EditorClaude, markers: []string{".claude", "CLAUDE.md"}},
	{id: EditorCursor, markers: []string{".cursor", ".cursorrules"}},
	{id: EditorWindsurf, markers: []string{".windsurf", ".windsurfrules"}},
	{id: EditorContinue, markers: []string{".continue", ".continuerules"}},
	{id: EditorCopilot, markers: []string{".github/copilot-instructions.md"}},
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
	var hits []Editor
	for _, spec := range editorSpecs {
		for _, m := range spec.markers {
			if exists(filepath.Join(root, filepath.FromSlash(m))) {
				hits = append(hits, spec.id)
				break
			}
		}
	}
	if len(hits) == 0 {
		return []Editor{EditorClaude}
	}
	return hits
}

// InitOptions controls what `korva harness init` produces.
type InitOptions struct {
	Root        string   // target directory
	Project     string   // project name (goes into AGENTS.md + feature_list.json)
	Description string   // short blurb
	Stack       Stack    // chosen preset; empty falls back to Generic
	Editors     []Editor // editor rule templates to install; empty installs none
	Overwrite   bool     // when false (default) we refuse to overwrite existing files
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
		seed := &FeatureList{
			Project:     opts.Project,
			Description: opts.Description,
			Rules:       DefaultRules(),
			Features: []Feature{
				{
					ID:    1,
					Name:  "harness_smoke",
					Title: "Verify the harness is wired correctly",
					Description: "First feature in every new harness: run `./init.sh` and confirm it exits 0. " +
						"Once the smoke passes, replace this feature with the real backlog.",
					Acceptance: []string{
						"`./init.sh` exits with code 0",
						"`feature_list.json` validates",
						"`progress/current.md` exists",
					},
					Status: StatusPending,
				},
			},
		}
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
	}

	return written, nil
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
// template file may reference.
func templateVars(opts InitOptions) map[string]string {
	return map[string]string{
		"Project":     opts.Project,
		"Description": opts.Description,
		"Stack":       string(opts.Stack),
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
