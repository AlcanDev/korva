// Package analyzer orchestrates Korva Sentinel rule checks.
package analyzer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alcandev/korva/sentinel/validator/internal/rules"
)

// Report is the full output of an analysis run.
type Report struct {
	Files      int               `json:"files_analyzed"`
	Violations []rules.Violation `json:"violations"`
	Errors     int               `json:"errors"`
	Warnings   int               `json:"warnings"`
}

// Analyzer runs configured rules against files.
type Analyzer struct {
	rules []rules.Rule
}

// New creates an Analyzer with the given rules (defaults to AllRules if nil).
func New(rs []rules.Rule) *Analyzer {
	if rs == nil {
		rs = rules.AllRules()
	}
	return &Analyzer{rules: rs}
}

// AnalyzeFiles checks each file path and returns a combined Report.
func (a *Analyzer) AnalyzeFiles(paths []string) Report {
	report := Report{}

	for _, path := range paths {
		if path == "" {
			continue
		}
		lines, err := readLines(path)
		if err != nil {
			// File may have been deleted (e.g. in a delete commit) — skip silently
			continue
		}

		report.Files++
		for _, rule := range a.rules {
			if !rule.Applies(path) {
				continue
			}
			vs := rule.Check(path, lines)
			for _, v := range vs {
				report.Violations = append(report.Violations, v)
				switch v.Severity {
				case rules.SeverityError:
					report.Errors++
				case rules.SeverityWarning:
					report.Warnings++
				}
			}
		}
	}

	return report
}

// PrintText writes a human-readable report to w.
func PrintText(w io.Writer, r Report) {
	if len(r.Violations) == 0 {
		fmt.Fprintf(w, "✓ Korva Sentinel: %d file(s) analyzed, no violations.\n", r.Files)
		return
	}

	fmt.Fprintf(w, "✗ Korva Sentinel: %d file(s) analyzed — %d error(s), %d warning(s)\n\n",
		r.Files, r.Errors, r.Warnings)

	for _, v := range r.Violations {
		icon := "⚠"
		if v.Severity == rules.SeverityError {
			icon = "✗"
		}
		fmt.Fprintf(w, "  %s [%s] %s:%d\n     %s\n\n",
			icon, v.Rule, v.File, v.Line, v.Message)
	}
}

// PrintJSON writes a JSON report to w.
func PrintJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ReadPathsFromStdin reads newline-separated file paths from stdin.
// Used when the binary is called from a git hook via `git diff --name-only | korva-sentinel`.
func ReadPathsFromStdin() []string {
	var paths []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p := strings.TrimSpace(scanner.Text())
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// --- helpers ---

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
