package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Phase 15.A — Continuous Integration templates.
//
// `korva harness ci install --provider=<X>` materializes a ready-to-use
// CI workflow that runs `korva harness check` on every PR/MR and posts
// the backlog summary as a comment. Two providers ship out of the box:
//
//   github-actions  → .github/workflows/harness.yml
//   gitlab-ci       → .gitlab-ci.harness.yml
//
// Both templates are embedded verbatim (no Go template substitution
// needed — the workflows use environment variables from the CI runner
// to fill in repo / PR / token / etc.). That keeps them readable inside
// the binary and avoids a delimiter clash with GitHub Actions' own
// `${{ … }}` syntax.

// CIProvider identifies the CI vendor a template targets.
type CIProvider string

const (
	CIGitHubActions CIProvider = "github-actions"
	CIGitLab        CIProvider = "gitlab-ci"
)

// AllCIProviders lists every preset in CLI help / error messages.
var AllCIProviders = []CIProvider{CIGitHubActions, CIGitLab}

// IsKnownCIProvider reports whether `p` names one of the shipped
// providers. Mirrors IsKnownStack / IsKnownEditor for consistency.
func IsKnownCIProvider(p CIProvider) bool {
	for _, x := range AllCIProviders {
		if x == p {
			return true
		}
	}
	return false
}

// ciSpec describes one provider's template tree + destination.
type ciSpec struct {
	provider CIProvider
	srcDir   string // embed FS directory rooted at `templates/ci/<vendor>/`
}

var ciSpecs = []ciSpec{
	{provider: CIGitHubActions, srcDir: "templates/ci/github-actions"},
	{provider: CIGitLab, srcDir: "templates/ci/gitlab"},
}

// InstallCIResult is the structured outcome of InstallCI. Returned so
// the CLI can print one line per file, and so callers (MCP, tests) can
// assert on what landed.
type InstallCIResult struct {
	Provider CIProvider `json:"provider"`
	Root     string     `json:"root"`
	Written  []string   `json:"written"`
	Skipped  []string   `json:"skipped"` // files that already existed and weren't overwritten
}

// InstallCI materializes the templates for `provider` into `root`.
// Files are written verbatim (raw bytes, no text/template processing)
// to preserve native CI syntax. Existing files are left alone unless
// `overwrite` is true — the operator's local edits survive a re-run.
func InstallCI(root string, provider CIProvider, overwrite bool) (*InstallCIResult, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("root is required")
	}
	if !IsKnownCIProvider(provider) {
		return nil, fmt.Errorf("unknown CI provider %q: pick one of %s", provider, joinCIProviders())
	}

	var spec ciSpec
	for _, s := range ciSpecs {
		if s.provider == provider {
			spec = s
			break
		}
	}
	if _, err := templateFS.ReadDir(spec.srcDir); err != nil {
		// Should never happen — embed FS is build-time validated. If it
		// does, surface a clear message rather than a confusing
		// io.ReadDir error string.
		return nil, fmt.Errorf("template tree missing for provider %q: %w", provider, err)
	}

	res := &InstallCIResult{Provider: provider, Root: root}
	if err := fs.WalkDir(templateFS, spec.srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Slash-form path within the embed tree.
		rel := strings.TrimPrefix(p, spec.srcDir+"/")
		dest := filepath.Join(root, filepath.FromSlash(rel))
		if exists(dest) && !overwrite {
			res.Skipped = append(res.Skipped, rel)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dest, err)
		}
		body, err := templateFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", p, err)
		}
		if err := os.WriteFile(dest, body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		res.Written = append(res.Written, rel)
		return nil
	}); err != nil {
		return nil, err
	}
	return res, nil
}

// joinCIProviders stringifies AllCIProviders for help / error messages.
func joinCIProviders() string {
	parts := make([]string, 0, len(AllCIProviders))
	for _, p := range AllCIProviders {
		parts = append(parts, string(p))
	}
	return strings.Join(parts, ", ")
}
