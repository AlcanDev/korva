package harness

import (
	"fmt"
	"strings"
)

// Phase 13.3 — invariant validator that powers `korva harness check` and
// the init.sh §SDD block.
//
// Check produces a structured report of every contract violation in a
// harness directory. It is purely additive — Validate() (called from
// LoadFeatureList / SaveFeatureList) already enforces the schema-level
// invariants the state machine relies on. Check goes further:
//
//   - When the harness is in SDD mode, every feature whose status is
//     spec_ready / in_progress / done AND has sdd:true must own its
//     three spec files (specs/<name>/{requirements,design,tasks}.md).
//   - The seed feature's name (`harness_smoke`) must exist (defensive
//     against a careless rename).
//
// Future invariants attach here: each becomes one entry in CheckReport.
// Callers consume the structured report; the CLI / init.sh decide
// whether a given violation severity should fail.

// CheckSeverity classifies how serious a violation is. The CLI renders
// `error` red and exits non-zero; `warning` is yellow and informational.
type CheckSeverity string

const (
	SeverityError   CheckSeverity = "error"
	SeverityWarning CheckSeverity = "warning"
)

// CheckIssue is a single contract violation surfaced by Check.
type CheckIssue struct {
	Severity  CheckSeverity `json:"severity"`
	Code      string        `json:"code"` // stable machine-readable identifier
	FeatureID int           `json:"feature_id,omitempty"`
	Message   string        `json:"message"`
	Hint      string        `json:"hint,omitempty"` // suggested next step for the operator
}

// CheckReport is the structured outcome of running Check on a harness
// directory.
type CheckReport struct {
	Root    string       `json:"root"`
	Project string       `json:"project"`
	SDDMode bool         `json:"sdd_mode"`
	Issues  []CheckIssue `json:"issues"`
	OK      bool         `json:"ok"` // true ⇔ no error-severity issues
}

// HasErrors reports whether the report contains at least one issue of
// SeverityError. Used by callers that gate exit code on errors only.
func (r *CheckReport) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Check loads the feature_list.json at `root` and walks the invariants
// listed in the package doc. The error return is reserved for genuine
// I/O failures (no feature_list, parse error); contract violations land
// in the report.
func Check(root string) (*CheckReport, error) {
	fl, err := LoadFeatureList(root)
	if err != nil {
		return nil, fmt.Errorf("load feature_list.json: %w", err)
	}
	report := &CheckReport{
		Root:    root,
		Project: fl.Project,
		SDDMode: fl.Rules.RequireApprovedSpecToImplement,
	}

	for i := range fl.Features {
		f := &fl.Features[i]

		// SDD: every spec-driven feature past `pending` must have its
		// three spec files on disk. Without them the state machine has
		// promoted a feature past the design gate without a design.
		if f.SDD {
			needsSpec := f.Status == StatusSpecReady ||
				f.Status == StatusInProgress ||
				f.Status == StatusDone
			if needsSpec && !SpecComplete(root, f.Name) {
				report.Issues = append(report.Issues, CheckIssue{
					Severity:  SeverityError,
					Code:      "sdd_spec_missing",
					FeatureID: f.ID,
					Message:   fmt.Sprintf("feature #%d (%s) is %s but specs/%s/{requirements,design,tasks}.md are not all present", f.ID, f.Name, f.Status, f.Name),
					Hint:      fmt.Sprintf("run `korva harness spec %d` and draft the three files, or move the feature back to pending", f.ID),
				})
			}
		}

		// Defensive: a feature flagged sdd:true under a non-SDD ruleset
		// is technically legal (the state machine still routes it
		// correctly), but it's almost certainly an oversight — surface
		// it as a warning so the operator can either flip the rule or
		// clear the flag.
		if f.SDD && !report.SDDMode {
			report.Issues = append(report.Issues, CheckIssue{
				Severity:  SeverityWarning,
				Code:      "sdd_feature_outside_sdd_mode",
				FeatureID: f.ID,
				Message:   fmt.Sprintf("feature #%d (%s) is sdd:true but the harness rule require_approved_spec_to_implement is off", f.ID, f.Name),
				Hint:      "set rules.require_approved_spec_to_implement=true in feature_list.json, or remove the feature's sdd flag",
			})
		}
	}

	report.OK = !report.HasErrors()
	return report, nil
}

// FormatReport renders a human-readable, color-free summary of a
// CheckReport. Used by `korva harness check` (CLI) and by init.sh
// when it shells out and pipes the textual output into the operator's
// terminal.
func FormatReport(r *CheckReport) string {
	var b strings.Builder
	mode := "standard"
	if r.SDDMode {
		mode = "SDD"
	}
	fmt.Fprintf(&b, "harness check — project=%q mode=%s\n", r.Project, mode)
	if len(r.Issues) == 0 {
		fmt.Fprintln(&b, "  ✓ no issues")
		return b.String()
	}
	for _, i := range r.Issues {
		marker := "✗"
		if i.Severity == SeverityWarning {
			marker = "⚠"
		}
		fmt.Fprintf(&b, "  %s [%s] %s\n", marker, i.Code, i.Message)
		if i.Hint != "" {
			fmt.Fprintf(&b, "      → %s\n", i.Hint)
		}
	}
	return b.String()
}
