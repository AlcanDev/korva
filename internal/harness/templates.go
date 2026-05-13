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

// Phase 11 — Templates that `korva harness init` materializes into a repo.
//
// Each stack ships a complete harness:
//   AGENTS.md, init.sh, CHECKPOINTS.md
//   docs/{architecture, conventions, verification}.md
//   progress/{current,history}.md
//   .gitignore additions (progress/*.tmp, etc.)
//   .claude/agents/{leader,implementer,reviewer}.md  (optional via --with-subagents)
//
// Templates live under templates/<stack>/ and are embedded at build time
// so the binary is self-contained — no runtime dependency on the source
// tree shipping with the install.

// `all:` includes dotfile-prefixed paths (notably `.claude/agents/...`),
// which `//go:embed templates` would otherwise silently skip.
//
//go:embed all:templates
var templateFS embed.FS

// Stack picks the language/runtime preset. Currently we ship three with
// thoughtful defaults; the generic preset is the catch-all when an operator
// doesn't pick one explicitly.
type Stack string

const (
	StackGo      Stack = "go"
	StackTS      Stack = "typescript"
	StackPython  Stack = "python"
	StackGeneric Stack = "generic"
)

// AllStacks lists every preset, in the order the CLI prints them.
var AllStacks = []Stack{StackGo, StackTS, StackPython, StackGeneric}

// InitOptions controls what `korva harness init` produces.
type InitOptions struct {
	Root          string // target directory
	Project       string // project name (goes into AGENTS.md + feature_list.json)
	Description   string // short blurb
	Stack         Stack  // chosen preset; empty falls back to Generic
	WithSubagents bool   // also lay down .claude/agents/{leader,implementer,reviewer}.md
	Overwrite     bool   // when false (default) we refuse to overwrite existing files
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

	written := make([]string, 0, 16)

	// 1. Materialize the per-stack template tree.
	if err := walkAndWrite(stackDir, opts, &written); err != nil {
		return nil, err
	}

	// 2. Common files — AGENTS.md, CHECKPOINTS.md, progress/*.md — live in
	// templates/common/ so they aren't duplicated per stack.
	if err := walkAndWrite("templates/common", opts, &written); err != nil {
		return nil, err
	}

	// 3. feature_list.json — generated from defaults rather than a static
	// template so the seed matches the runtime types exactly.
	flPath := filepath.Join(opts.Root, FeatureListPath)
	if exists(flPath) && !opts.Overwrite {
		// don't clobber an existing backlog
	} else {
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

	// 4. Optional subagent templates.
	if opts.WithSubagents {
		if err := walkAndWrite("templates/subagents", opts, &written); err != nil {
			return nil, err
		}
	}

	return written, nil
}

// walkAndWrite copies every file under fsDir into opts.Root, processing
// `.tmpl` files through text/template. Skips writes that would overwrite
// an existing file unless opts.Overwrite is set.
func walkAndWrite(fsDir string, opts InitOptions, written *[]string) error {
	tmplVars := templateVars(opts)
	return fs.WalkDir(templateFS, fsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(fsDir, path)
		if err != nil {
			return err
		}
		// Strip the .tmpl suffix used to mark template files.
		outRel := strings.TrimSuffix(rel, ".tmpl")
		outPath := filepath.Join(opts.Root, outRel)
		if exists(outPath) && !opts.Overwrite {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", outPath, err)
		}
		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
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
