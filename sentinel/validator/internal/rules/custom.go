package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// CustomRule is a user-defined regex-based rule loaded from a YAML file.
// It satisfies the Rule interface so the validator treats it identically
// to the built-in rules.
//
// Pattern is compiled lazily on first Check; PathsInclude / PathsExclude use
// doublestar-like path matching via path/filepath's Match (one star matches
// a single path segment, double star is approximated with a regex).
type CustomRule struct {
	IDValue       string   `yaml:"id" json:"id"`
	Description   string   `yaml:"description,omitempty" json:"description,omitempty"`
	SeverityValue Severity `yaml:"severity,omitempty" json:"severity,omitempty"`
	Pattern       string   `yaml:"pattern" json:"pattern"`
	PathsInclude  []string `yaml:"paths_include,omitempty" json:"paths_include,omitempty"`
	PathsExclude  []string `yaml:"paths_exclude,omitempty" json:"paths_exclude,omitempty"`
	Message       string   `yaml:"message,omitempty" json:"message,omitempty"`

	compiled *regexp.Regexp // memoized compiled regex
}

// ID returns the unique identifier of the rule.
func (r *CustomRule) ID() string { return r.IDValue }

// Severity returns the rule severity, defaulting to "error" when unset.
func (r *CustomRule) Severity() Severity {
	if r.SeverityValue == "" {
		return SeverityError
	}
	return r.SeverityValue
}

// Applies reports whether this rule should run on the given file path.
// When PathsInclude is empty the rule applies to all files; PathsExclude
// always wins over PathsInclude.
func (r *CustomRule) Applies(path string) bool {
	if matchesAny(path, r.PathsExclude) {
		return false
	}
	if len(r.PathsInclude) == 0 {
		return true
	}
	return matchesAny(path, r.PathsInclude)
}

// Check returns the violations found in `lines`, compiling the regex on first
// call. Compilation errors are surfaced as a single rule violation pointing at
// line 0 so the operator notices the misconfigured rule.
func (r *CustomRule) Check(filePath string, lines []string) []Violation {
	if r.compiled == nil {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return []Violation{{
				File:     filePath,
				Line:     0,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("custom rule has invalid regex %q: %v", r.Pattern, err),
			}}
		}
		r.compiled = re
	}

	var out []Violation
	msg := r.Message
	if msg == "" {
		msg = fmt.Sprintf("matched %q", r.Pattern)
	}
	for i, line := range lines {
		if r.compiled.MatchString(line) {
			out = append(out, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: r.Severity(),
				Message:  msg,
			})
		}
	}
	return out
}

// Validate ensures the rule fields are well-formed before persisting.
// Caller-friendly messages are returned suitable for surfacing in an HTTP 400.
func (r *CustomRule) Validate() error {
	if !validRuleID(r.IDValue) {
		return fmt.Errorf("rule id %q must match [A-Z][A-Z0-9-]{2,30}", r.IDValue)
	}
	if r.Pattern == "" {
		return fmt.Errorf("rule %q: pattern is required", r.IDValue)
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("rule %q: invalid regex pattern: %w", r.IDValue, err)
	}
	if r.SeverityValue != "" {
		switch r.SeverityValue {
		case SeverityError, SeverityWarning, SeverityInfo:
		default:
			return fmt.Errorf("rule %q: severity must be error|warning|info (got %q)", r.IDValue, r.SeverityValue)
		}
	}
	for i, glob := range append(append([]string(nil), r.PathsInclude...), r.PathsExclude...) {
		if _, err := filepath.Match(globToFilepathPattern(glob), ""); err != nil {
			return fmt.Errorf("rule %q: invalid glob at index %d (%q): %w", r.IDValue, i, glob, err)
		}
	}
	return nil
}

var validIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,30}$`)

func validRuleID(id string) bool {
	return validIDPattern.MatchString(id)
}

// matchesAny returns true when `path` matches any of the glob patterns.
// `**` is approximated to "match any sequence of characters across separators";
// `*` matches any sequence within a single path segment.
func matchesAny(path string, globs []string) bool {
	clean := filepath.ToSlash(path)
	for _, g := range globs {
		pattern := globToRegex(g)
		if matched, _ := regexp.MatchString(pattern, clean); matched {
			return true
		}
	}
	return false
}

// globToRegex converts a doublestar-style glob to an anchored regex.
//
//   - "**/" → "(.*/)?"  (zero or more path segments, including their trailing slash)
//   - "**"  → ".*"      (any sequence — fallback when not followed by /)
//   - "*"   → "[^/]*"   (any sequence within a single path segment)
//   - "?"   → "."
//   - other → escaped literal
//
// The two-step approach below preserves doublestar semantics: `src/**/*.ts`
// must match both `src/app.ts` (zero intermediate dirs) and `src/utils/x.ts`
// (one or more intermediate dirs).
func globToRegex(glob string) string {
	glob = filepath.ToSlash(glob)

	const (
		dsSlash = "\x00DSS\x00" // sentinel for "**/"
		ds      = "\x00DS\x00"  // sentinel for "**"
		single  = "\x00S\x00"   // sentinel for "*"
	)
	// Step 1: tokenize stars so escaping doesn't touch them.
	withTokens := strings.ReplaceAll(glob, "**/", dsSlash)
	withTokens = strings.ReplaceAll(withTokens, "**", ds)
	withTokens = strings.ReplaceAll(withTokens, "*", single)

	// Step 2: escape regex metacharacters in the literal portions.
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(withTokens); i++ {
		c := withTokens[i]
		switch c {
		case '?':
			b.WriteByte('.')
		case '.', '+', '(', ')', '{', '}', '[', ']', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")

	// Step 3: replace tokens (which survived the escape pass intact) with regex.
	out := b.String()
	out = strings.ReplaceAll(out, dsSlash, "(?:.*/)?")
	out = strings.ReplaceAll(out, ds, ".*")
	out = strings.ReplaceAll(out, single, "[^/]*")
	return out
}

// globToFilepathPattern is a best-effort fallback for filepath.Match validation.
// We strip "**" before passing to filepath.Match (which doesn't support it).
func globToFilepathPattern(g string) string {
	return strings.ReplaceAll(g, "**", "*")
}
