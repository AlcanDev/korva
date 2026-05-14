package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Phase 15.B — Spec review automation.
//
// `korva harness ready <id>` and the spec_author subagent ensure the
// three spec files exist; this layer asserts the *quality* of what they
// contain. The reviewer subagent (or a human) can call ReviewSpec
// before approving a feature to enforce:
//
//   1. requirements.md uses EARS notation — at least one of the four
//      canonical patterns (WHEN / WHILE / WHERE / IF…THEN) shows up per
//      R-id, with non-placeholder content.
//   2. Every R-id in requirements.md is covered by at least one T-id
//      in tasks.md (the "Covers: R<n>" tag is parsed verbatim).
//   3. Every acceptance bullet from feature_list.json maps to ≥ 1 R-id
//      via the traceability table in requirements.md.
//   4. No raw `<placeholder>` strings remain — operators sometimes
//      forget to fill them; the linter catches that.
//
// The output mirrors check.go's CheckReport shape: structured Issue
// rows with severity, code, message + hint. Callers (CLI / MCP) decide
// whether to gate on errors.

// SpecReviewReport is the structured outcome of ReviewSpec.
type SpecReviewReport struct {
	Project   string       `json:"project"`
	FeatureID int          `json:"feature_id"`
	Feature   string       `json:"feature"`
	SpecDir   string       `json:"spec_dir"`
	Issues    []CheckIssue `json:"issues"`
	OK        bool         `json:"ok"`
	// RIDs found in requirements.md and TIDs in tasks.md, exposed so
	// the dashboard / debugging can render the coverage matrix.
	RIDs []string `json:"r_ids"`
	TIDs []string `json:"t_ids"`
	// Coverage maps R-id → list of T-ids that cover it. R-ids with an
	// empty slice surface as `sdd_spec_rid_uncovered` issues.
	Coverage map[string][]string `json:"coverage"`
}

// HasErrors reports whether the report contains at least one error-
// severity issue. Mirrors CheckReport.HasErrors for symmetry.
func (r *SpecReviewReport) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// CountBySeverity returns (errors, warnings) — used by the verdict
// derivation and by the CLI summary line.
func (r *SpecReviewReport) CountBySeverity() (errors, warnings int) {
	for _, i := range r.Issues {
		switch i.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}
	return
}

// Verdict derives the high-level outcome from the issue list using
// the same precedence rule the reviewer subagent prompt teaches:
//
//	any error → reject
//	any warning → needs_fixes
//	clean → approve
//
// Phase 18.A — the reviewer can override this in their `--verdict`
// flag (e.g. when the spec linter passes but the human spot-checked
// a design hole the linter can't see). Verdict() is what the CLI
// suggests when no override is provided.
func (r *SpecReviewReport) Verdict() ReviewVerdict {
	errors, warnings := r.CountBySeverity()
	switch {
	case errors > 0:
		return VerdictReject
	case warnings > 0:
		return VerdictNeedsFixes
	default:
		return VerdictApprove
	}
}

// ReviewSpec runs the spec linter on the feature at `featureID` inside
// the harness rooted at `root`. Returns sql-style structured report
// regardless of outcome; only returns an error for genuine I/O failures
// (missing feature, missing files).
//
// The feature must be SDD-flagged AND have all three spec files on
// disk — both preconditions are surfaced as I/O-style errors rather
// than report issues, because they indicate a broken precondition for
// running the lint at all.
func ReviewSpec(root string, featureID int) (*SpecReviewReport, error) {
	fl, err := LoadFeatureList(root)
	if err != nil {
		return nil, fmt.Errorf("load feature list: %w", err)
	}
	f := fl.FindByID(featureID)
	if f == nil {
		return nil, fmt.Errorf("feature %d not found", featureID)
	}
	if !f.SDD {
		return nil, fmt.Errorf("feature %d (%s) is not SDD-flagged — spec review only applies to SDD features", featureID, f.Name)
	}
	if !SpecComplete(root, f.Name) {
		return nil, fmt.Errorf("spec files missing for %s — run `korva harness spec %d` first", f.Name, featureID)
	}

	dir := SpecDir(root, f.Name)
	requirements, err := os.ReadFile(filepath.Join(dir, "requirements.md"))
	if err != nil {
		return nil, fmt.Errorf("read requirements.md: %w", err)
	}
	tasks, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
	if err != nil {
		return nil, fmt.Errorf("read tasks.md: %w", err)
	}

	report := &SpecReviewReport{
		Project:   fl.Project,
		FeatureID: featureID,
		Feature:   f.Name,
		SpecDir:   dir,
		Coverage:  map[string][]string{},
	}

	// 1. Extract R-ids from requirements.md.
	parsedReqs := parseRequirements(string(requirements))
	for _, r := range parsedReqs {
		report.RIDs = append(report.RIDs, r.ID)
	}

	// 2. Validate EARS form + placeholders per R-id.
	for _, r := range parsedReqs {
		if r.hasPlaceholder {
			report.Issues = append(report.Issues, CheckIssue{
				Severity:  SeverityError,
				Code:      "sdd_spec_placeholder",
				FeatureID: featureID,
				Message:   fmt.Sprintf("%s in requirements.md still contains a `<placeholder>` — replace with real content", r.ID),
				Hint:      "edit specs/" + f.Name + "/requirements.md and fill the placeholders",
			})
		} else if !r.hasEARSForm {
			report.Issues = append(report.Issues, CheckIssue{
				Severity:  SeverityError,
				Code:      "sdd_spec_ears_missing",
				FeatureID: featureID,
				Message:   fmt.Sprintf("%s in requirements.md does not use an EARS form (WHEN / WHILE / WHERE / IF…THEN)", r.ID),
				Hint:      "rewrite the requirement using one of the four canonical EARS patterns",
			})
		}
	}

	// 3. Extract T-ids + their R-id coverage from tasks.md.
	parsedTasks := parseTasks(string(tasks))
	for _, tk := range parsedTasks {
		report.TIDs = append(report.TIDs, tk.ID)
	}

	// 4. Build coverage map: R-id → T-ids that cite it.
	for _, r := range parsedReqs {
		report.Coverage[r.ID] = nil
	}
	for _, tk := range parsedTasks {
		for _, ref := range tk.Covers {
			report.Coverage[ref] = append(report.Coverage[ref], tk.ID)
		}
	}
	// 5. Flag uncovered R-ids.
	for _, r := range parsedReqs {
		if len(report.Coverage[r.ID]) == 0 {
			report.Issues = append(report.Issues, CheckIssue{
				Severity:  SeverityError,
				Code:      "sdd_spec_rid_uncovered",
				FeatureID: featureID,
				Message:   fmt.Sprintf("%s is not covered by any task in tasks.md", r.ID),
				Hint:      "add a task `- [ ] T<n> — … *(Covers: " + r.ID + ")*` to specs/" + f.Name + "/tasks.md",
			})
		}
	}
	// 6. Flag T-ids that reference non-existent R-ids (typo guard).
	knownRIDs := map[string]bool{}
	for _, r := range parsedReqs {
		knownRIDs[r.ID] = true
	}
	for _, tk := range parsedTasks {
		for _, ref := range tk.Covers {
			if !knownRIDs[ref] {
				report.Issues = append(report.Issues, CheckIssue{
					Severity:  SeverityWarning,
					Code:      "sdd_spec_tid_dangling",
					FeatureID: featureID,
					Message:   fmt.Sprintf("%s references %s in tasks.md but that R-id is not defined in requirements.md", tk.ID, ref),
					Hint:      "fix the typo or add the missing R-id to requirements.md",
				})
			}
		}
	}

	// 7. Cross-check the feature_list acceptance bullets against the
	// traceability table inside requirements.md. The table is parsed
	// loosely — every cell in the second column is split on commas /
	// spaces and matched against the known R-ids. A bullet whose row
	// is empty (or absent) surfaces as an error.
	if len(f.Acceptance) > 0 {
		traceCoverage := parseTraceabilityTable(string(requirements), knownRIDs)
		for i, bullet := range f.Acceptance {
			bulletKey := normalizeAcceptance(bullet)
			covered := false
			for tracedKey, rids := range traceCoverage {
				if normalizeAcceptance(tracedKey) == bulletKey && len(rids) > 0 {
					covered = true
					break
				}
			}
			if !covered {
				report.Issues = append(report.Issues, CheckIssue{
					Severity:  SeverityError,
					Code:      "sdd_spec_acceptance_untraced",
					FeatureID: featureID,
					Message:   fmt.Sprintf("acceptance #%d (%q) is not mapped to any R-id in the traceability table", i+1, truncate(bullet, 60)),
					Hint:      "add a row in the Traceability section of specs/" + f.Name + "/requirements.md",
				})
			}
		}
	}

	// Sort issues by code+feature-id so reports are reproducible (helps
	// snapshot tests + diffing reports across runs).
	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Code != report.Issues[j].Code {
			return report.Issues[i].Code < report.Issues[j].Code
		}
		return report.Issues[i].Message < report.Issues[j].Message
	})

	report.OK = !report.HasErrors()
	return report, nil
}

// FormatSpecReviewReport renders a human-readable summary used by
// `korva harness review`. Same look-and-feel as FormatReport but with
// the per-feature header so an operator reviewing many features can
// scan output linearly.
func FormatSpecReviewReport(r *SpecReviewReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec review — project=%q feature=#%d (%s)\n", r.Project, r.FeatureID, r.Feature)
	fmt.Fprintf(&b, "  R-ids: %s\n", strings.Join(r.RIDs, ", "))
	fmt.Fprintf(&b, "  T-ids: %s\n", strings.Join(r.TIDs, ", "))
	if len(r.Issues) == 0 {
		fmt.Fprintln(&b, "  ✓ no issues — spec is ready for human approval")
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

// ── parsers ─────────────────────────────────────────────────────────────────

type parsedRequirement struct {
	ID             string
	hasEARSForm    bool
	hasPlaceholder bool
}

// rRequirementHeading matches "## R1 — …" or "## R12 (anything)" so we
// can extract R-ids from the requirements doc. The dash after the id
// is encouraged but not strictly required.
var rRequirementHeading = regexp.MustCompile(`(?m)^##\s+(R\d+)\b`)

// earsForm matches the four canonical EARS patterns. Case-insensitive
// because operators write "When" / "WHEN" interchangeably.
var earsForm = regexp.MustCompile(`(?i)\b(WHEN|WHILE|WHERE|IF\b[^\n]*\bTHEN)\b`)

// placeholderPattern matches any unreplaced `<placeholder>` token. The
// templates ship with many; operators must fill them all.
var placeholderPattern = regexp.MustCompile(`<[a-z][^>\n]{0,80}>`)

// parseRequirements extracts the R-ids defined in a requirements.md
// body, plus per-R-id flags (EARS form present, placeholders remaining).
// Sections are demarcated by the next "## R<n>" heading.
func parseRequirements(body string) []parsedRequirement {
	idxs := rRequirementHeading.FindAllStringSubmatchIndex(body, -1)
	out := make([]parsedRequirement, 0, len(idxs))
	for i, m := range idxs {
		// m = [matchStart, matchEnd, group1Start, group1End]
		id := body[m[2]:m[3]]
		bodyStart := m[1] // end of the heading line
		bodyEnd := len(body)
		if i+1 < len(idxs) {
			bodyEnd = idxs[i+1][0]
		}
		section := body[bodyStart:bodyEnd]
		out = append(out, parsedRequirement{
			ID:             id,
			hasEARSForm:    earsForm.MatchString(section),
			hasPlaceholder: placeholderPattern.MatchString(section),
		})
	}
	return out
}

type parsedTask struct {
	ID     string
	Covers []string
}

// taskLine matches a markdown checkbox row that introduces a task:
//
//   - [x] T1 — implement *(Covers: R1, R2)*
//   - [ ] T3 — closure
//
// Greedy `.*$` swallows the rest of the line so the optional covers
// annotation is parsed in a separate pass — combining them into one
// regex tripped the non-greedy + optional-group corner where the
// matcher prefers the empty match.
var taskLine = regexp.MustCompile(`(?m)^-\s+\[[ xX]\]\s+(T\d+)\b.*$`)

// coversAnnotation extracts the comma-separated R-ids out of a trailing
// `*(Covers: R1, R2)*` token. Applied to the full matched task line.
var coversAnnotation = regexp.MustCompile(`\(Covers:\s*([^)]+)\)`)

// rIDExtract pulls "R12" tokens out of the covers cell, ignoring
// surrounding commas / spaces / whitespace.
var rIDExtract = regexp.MustCompile(`R\d+`)

// parseTasks extracts every T-id from tasks.md along with the R-ids it
// covers (parsed from the trailing `*(Covers: R1, R2)*` annotation).
func parseTasks(body string) []parsedTask {
	matches := taskLine.FindAllStringSubmatch(body, -1)
	out := make([]parsedTask, 0, len(matches))
	for _, m := range matches {
		tk := parsedTask{ID: m[1]}
		if cov := coversAnnotation.FindStringSubmatch(m[0]); len(cov) > 1 {
			tk.Covers = rIDExtract.FindAllString(cov[1], -1)
		}
		out = append(out, tk)
	}
	return out
}

// traceTableRow matches a row in the Traceability markdown table:
//
//	| <acceptance bullet> | R1, R2 |
//
// The header / separator rows are filtered out by requiring the second
// cell to contain at least one R-id.
var traceTableRow = regexp.MustCompile(`(?m)^\|\s*([^|\n]+?)\s*\|\s*([^|\n]+?)\s*\|`)

// parseTraceabilityTable returns a map of acceptance-bullet → R-ids.
// The acceptance text is the verbatim first cell; the caller normalizes
// before comparing against feature_list.json bullets.
func parseTraceabilityTable(body string, knownRIDs map[string]bool) map[string][]string {
	out := map[string][]string{}
	for _, m := range traceTableRow.FindAllStringSubmatch(body, -1) {
		acceptance := strings.TrimSpace(m[1])
		coveredCell := strings.TrimSpace(m[2])
		// Filter out the header row "| feature_list.json acceptance | Covered by |"
		// and separator rows like "|---|---|"
		if acceptance == "" || strings.HasPrefix(acceptance, "-") {
			continue
		}
		rids := rIDExtract.FindAllString(coveredCell, -1)
		valid := make([]string, 0, len(rids))
		for _, r := range rids {
			if knownRIDs[r] {
				valid = append(valid, r)
			}
		}
		if len(valid) > 0 {
			out[acceptance] = valid
		}
	}
	return out
}

// normalizeAcceptance lowercases + strips backticks / leading symbols
// so a bullet in feature_list.json ("`./init.sh` exits 0") matches the
// same text inside the traceability table even when the operator pasted
// it with slightly different formatting.
func normalizeAcceptance(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
