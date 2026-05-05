// Package detect infers the active project name from the working directory.
// It tries six strategies in order, stopping at the first conclusive result.
package detect

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result is the outcome of project auto-detection.
type Result struct {
	// Project is the resolved project name (empty only when Source=="ambiguous").
	Project string
	// Source describes which detection strategy succeeded.
	// One of: "config", "git_remote", "git_root", "git_child", "dir_basename", "ambiguous".
	Source string
	// AvailableProjects is populated only when Source=="ambiguous" (2+ git repos
	// found directly inside the working directory). The caller should surface this
	// list to the user so they can pick one explicitly.
	AvailableProjects []string
	// Warning carries a non-fatal advisory (e.g. "auto-promoted from child repo").
	Warning string
}

// Project runs all detection strategies for workingDir and returns the first
// conclusive result. If workingDir is empty, the process's current directory
// is used.
func Project(workingDir string) Result {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return Result{Project: "unknown", Source: "dir_basename"}
		}
	}

	// 1. .korva/config.json with explicit "project" key.
	if r, ok := fromConfigFile(workingDir); ok {
		return r
	}

	// 2. cwd is a git root with a remote origin URL.
	if r, ok := fromGitRemote(workingDir); ok {
		return r
	}

	// 3. cwd is inside a git repo — use the repo root's basename.
	if r, ok := fromGitRoot(workingDir); ok {
		return r
	}

	// 4. cwd is the parent of exactly one git sub-directory — auto-promote.
	if r, ok := fromGitChild(workingDir); ok {
		return r
	}

	// 5. cwd is the parent of 2+ git sub-directories — ambiguous, surface the list.
	if r, ok := fromAmbiguous(workingDir); ok {
		return r
	}

	// 6. Last resort: directory basename.
	return Result{
		Project: filepath.Base(workingDir),
		Source:  "dir_basename",
	}
}

// ── strategy helpers ──────────────────────────────────────────────────────────

func fromConfigFile(dir string) (Result, bool) {
	configPath := filepath.Join(dir, ".korva", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Result{}, false
	}
	var cfg struct {
		Project string `json:"project"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Project == "" {
		return Result{}, false
	}
	return Result{Project: cfg.Project, Source: "config"}, true
}

func fromGitRemote(dir string) (Result, bool) {
	if !isGitRoot(dir) {
		return Result{}, false
	}
	remote := gitRemoteOrigin(dir)
	if remote == "" {
		return Result{}, false
	}
	return Result{Project: repoNameFromURL(remote), Source: "git_remote"}, true
}

func fromGitRoot(dir string) (Result, bool) {
	root := gitRootOf(dir)
	if root == "" {
		return Result{}, false
	}
	return Result{Project: filepath.Base(root), Source: "git_root"}, true
}

func fromGitChild(dir string) (Result, bool) {
	children := gitChildDirs(dir)
	if len(children) != 1 {
		return Result{}, false
	}
	name := filepath.Base(children[0])
	return Result{
		Project: name,
		Source:  "git_child",
		Warning: "project auto-detected from child directory " + name + "; pass project explicitly to override",
	}, true
}

func fromAmbiguous(dir string) (Result, bool) {
	children := gitChildDirs(dir)
	if len(children) < 2 {
		return Result{}, false
	}
	names := make([]string, len(children))
	for i, c := range children {
		names[i] = filepath.Base(c)
	}
	return Result{
		Source:            "ambiguous",
		AvailableProjects: names,
	}, true
}

// ── git helpers ───────────────────────────────────────────────────────────────

// isGitRoot returns true if dir itself is a git repository root.
func isGitRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// gitRootOf returns the absolute path of the git repository that contains dir,
// or "" if dir is not inside any git repository.
func gitRootOf(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRemoteOrigin returns the remote origin URL for the repository rooted at
// dir, or "" if none is configured.
func gitRemoteOrigin(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitChildDirs returns the list of immediate subdirectories of dir that are
// themselves git repository roots.
func gitChildDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if isGitRoot(child) {
			result = append(result, child)
		}
	}
	return result
}

// repoNameFromURL extracts a clean repository name from a remote URL.
// Works for SSH (git@github.com:org/repo.git) and HTTPS formats.
func repoNameFromURL(rawURL string) string {
	// Trim trailing .git
	rawURL = strings.TrimSuffix(rawURL, ".git")
	// For SSH-style: git@github.com:org/repo → last segment after /
	if idx := strings.LastIndex(rawURL, "/"); idx >= 0 {
		rawURL = rawURL[idx+1:]
	}
	// For colon-style without slash: git@github.com:repo
	if idx := strings.LastIndex(rawURL, ":"); idx >= 0 {
		rawURL = rawURL[idx+1:]
	}
	if rawURL == "" {
		return "unknown"
	}
	return rawURL
}
