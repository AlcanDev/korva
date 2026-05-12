package detect

import (
	"strings"
	"unicode"
)

// NormalizeProjectName produces a stable, comparison-friendly form of a project
// name so two spellings that should refer to the same project ("My Project",
// "my-project", "my_project") fold to the same key.
//
// The transform is:
//  1. lowercase everything
//  2. replace any run of non-alphanumeric characters with a single "-"
//  3. trim leading/trailing dashes
//
// Empty input returns "" so callers can decide how to handle it.
func NormalizeProjectName(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSep := true // treats the start as a separator so leading non-alnum is dropped
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSep = false
			continue
		}
		if !prevSep {
			b.WriteByte('-')
			prevSep = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// FindSimilarProjects returns project names whose normalized form matches the
// normalized form of `target`, excluding `target` itself. The result is the
// caller-friendly canonical spelling (whatever appeared in `known`).
//
// Used by the project-resolution flow to warn "you saved under 'my_proj' but
// 'my-proj' already exists in the vault — same canonical name." Returns nil
// when no near-collisions exist.
func FindSimilarProjects(target string, known []string) []string {
	if target == "" {
		return nil
	}
	wantNorm := NormalizeProjectName(target)
	if wantNorm == "" {
		return nil
	}
	var matches []string
	for _, k := range known {
		if k == target {
			continue
		}
		if NormalizeProjectName(k) == wantNorm {
			matches = append(matches, k)
		}
	}
	return matches
}
