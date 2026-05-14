package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Phase 13.2 — per-feature spec scaffolding.
//
// The Phase 13.1 generator drops a single `specs/SPEC-TEMPLATE/`
// directory the operator copies per feature. That's fine for humans
// but tedious for agents driving the harness from a CLI / MCP call.
//
// `MaterializeSpec` reads the SPEC-TEMPLATE templates from the embed
// FS, renders them with the feature's project + name, and writes them
// to `specs/<feature.Name>/{requirements,design,tasks}.md` — only when
// the destination files don't already exist. This is the
// "specs-scaffolding-on-demand" primitive that both `korva harness spec`
// (CLI) and `vault_harness_spec` (MCP) call into.

// SpecFiles is the canonical list of files that make up one feature's
// spec — kept here so callers don't have to hard-code the names.
var SpecFiles = []string{"requirements.md", "design.md", "tasks.md"}

// SpecDir returns the conventional location of a feature's spec
// directory: `<root>/specs/<feature-name>`. Pure path join — does not
// touch disk.
func SpecDir(root, featureName string) string {
	return filepath.Join(root, "specs", featureName)
}

// SpecComplete reports whether all three canonical spec files exist
// for the feature. Used by 13.3's init.sh validator and by SetStatus
// (Phase 13.3) to refuse `pending → spec_ready` transitions when the
// drafts haven't been written.
func SpecComplete(root, featureName string) bool {
	dir := SpecDir(root, featureName)
	for _, f := range SpecFiles {
		if !exists(filepath.Join(dir, f)) {
			return false
		}
	}
	return true
}

// MaterializeSpecResult describes the outcome of MaterializeSpec.
type MaterializeSpecResult struct {
	Dir     string   // absolute or relative path the files were written into
	Written []string // file names actually written this call (subset of SpecFiles)
	Skipped []string // file names that already existed and weren't overwritten
}

// MaterializeSpec writes the three canonical spec files for a feature
// to `specs/<feature.Name>/`. Templates come from the embed FS so the
// binary is self-contained.
//
// Behavior:
//   - Files that already exist are left alone (Skipped list grows) —
//     idempotent, safe to re-run.
//   - When `overwrite` is true, existing files ARE replaced (use for
//     scaffolding a brand-new feature deliberately).
//   - Returns an error only when the underlying I/O fails — a missing
//     SPEC-TEMPLATE in the embed FS is a programmer bug, not a user
//     error.
func MaterializeSpec(root string, feature *Feature, overwrite bool) (*MaterializeSpecResult, error) {
	if feature == nil {
		return nil, fmt.Errorf("nil feature")
	}
	if strings.TrimSpace(feature.Name) == "" {
		return nil, fmt.Errorf("feature has no name — spec dir cannot be derived")
	}
	dir := SpecDir(root, feature.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	vars := map[string]string{
		"Project":     "",
		"FeatureName": feature.Name,
		"Title":       feature.Title,
		"Description": feature.Description,
	}

	result := &MaterializeSpecResult{Dir: dir}
	for _, name := range SpecFiles {
		dest := filepath.Join(dir, name)
		if exists(dest) && !overwrite {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		src := "templates/sdd/specs/SPEC-TEMPLATE/" + name + ".tmpl"
		body, err := fs.ReadFile(templateFS, src)
		if err != nil {
			return nil, fmt.Errorf("read spec template %s: %w", src, err)
		}
		t, err := template.New(name).Parse(string(body))
		if err != nil {
			return nil, fmt.Errorf("parse spec template %s: %w", src, err)
		}
		f, err := os.Create(dest)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", dest, err)
		}
		if err := t.Execute(f, vars); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("render %s: %w", dest, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("close %s: %w", dest, err)
		}
		result.Written = append(result.Written, name)
	}
	return result, nil
}
