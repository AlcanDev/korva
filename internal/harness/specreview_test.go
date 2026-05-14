package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sddHarness sets up an SDD-enabled harness with one feature flagged
// `sdd: true`, materializes the spec scaffolding from the templates,
// and returns the root path. Tests then mutate the spec files to
// exercise specific linter paths.
func sddHarnessWithSpec(t *testing.T, featureName string, acceptance []string) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "demo", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Replace the seed harness_smoke feature with the test's own.
	fl, _ := LoadFeatureList(dir)
	fl.Features = []Feature{
		{
			ID: 1, Name: featureName, Title: featureName, Status: StatusSpecReady,
			SDD:        true,
			Acceptance: acceptance,
		},
	}
	if err := SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := MaterializeSpec(dir, &fl.Features[0], false); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	return dir
}

// writeSpecFile is a small helper that overwrites one of the three
// per-feature spec files with arbitrary content (used to set up
// scenarios for the linter).
func writeSpecFile(t *testing.T, root, feature, name, body string) {
	t.Helper()
	path := filepath.Join(root, "specs", feature, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestReviewSpec_FreshScaffoldFailsOnPlaceholders(t *testing.T) {
	// The shipped templates have many `<placeholder>` strings — a
	// freshly-materialized spec must fail the lint until the operator
	// fills them.
	dir := sddHarnessWithSpec(t, "auth", []string{"login works"})
	report, err := ReviewSpec(dir, 1)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if report.OK {
		t.Error("fresh scaffold should fail the lint")
	}
	codes := issueCodes(report.Issues)
	if !contains(codes, "sdd_spec_placeholder") {
		t.Errorf("expected sdd_spec_placeholder issue, got %v", codes)
	}
}

func TestReviewSpec_HappyPathPasses(t *testing.T) {
	dir := sddHarnessWithSpec(t, "auth", []string{"login works"})
	// Replace with clean content that satisfies every rule.
	writeSpecFile(t, dir, "auth", "requirements.md", `# Requirements

## R1
WHEN the user submits valid credentials THE SYSTEM SHALL mint a session.

## Traceability

| feature_list.json acceptance | Covered by |
|---|---|
| login works | R1 |
`)
	writeSpecFile(t, dir, "auth", "tasks.md", `# Tasks

## Implementation
- [x] T1 — Implement login handler *(Covers: R1)*
## Tests
- [x] T2 — test_login_happy_path *(Covers: R1)*
`)
	report, err := ReviewSpec(dir, 1)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !report.OK {
		t.Errorf("expected OK report, got issues: %+v", report.Issues)
	}
	if len(report.RIDs) != 1 || report.RIDs[0] != "R1" {
		t.Errorf("RIDs = %v", report.RIDs)
	}
	if got := report.Coverage["R1"]; len(got) != 2 {
		t.Errorf("R1 coverage = %v, want 2 T-ids", got)
	}
}

func TestReviewSpec_UncoveredRIDFailsLint(t *testing.T) {
	dir := sddHarnessWithSpec(t, "auth", []string{"login works"})
	writeSpecFile(t, dir, "auth", "requirements.md", `# Requirements
## R1
WHEN the user logs in THE SYSTEM SHALL mint a session.
## R2
WHEN the session expires THE SYSTEM SHALL prompt re-auth.

## Traceability
| acceptance | Covered by |
| login works | R1, R2 |
`)
	writeSpecFile(t, dir, "auth", "tasks.md", `# Tasks
- [ ] T1 — handler *(Covers: R1)*
`)
	report, err := ReviewSpec(dir, 1)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if report.OK {
		t.Error("uncovered R-id should fail")
	}
	codes := issueCodes(report.Issues)
	if !contains(codes, "sdd_spec_rid_uncovered") {
		t.Errorf("expected uncovered issue, got %v", codes)
	}
	// R1 is covered, R2 isn't.
	if len(report.Coverage["R1"]) == 0 {
		t.Errorf("R1 should be covered")
	}
	if len(report.Coverage["R2"]) != 0 {
		t.Errorf("R2 should have empty coverage, got %v", report.Coverage["R2"])
	}
}

func TestReviewSpec_DanglingTIDIsWarning(t *testing.T) {
	// A T-id citing an R-id that doesn't exist is a typo — surface as
	// warning, not error (the implementer might just have a typo and
	// will fix on re-render).
	dir := sddHarnessWithSpec(t, "auth", []string{"login works"})
	writeSpecFile(t, dir, "auth", "requirements.md", `# Requirements
## R1
WHEN x THE SYSTEM SHALL y.
## Traceability
| login works | R1 |
`)
	writeSpecFile(t, dir, "auth", "tasks.md", `# Tasks
- [x] T1 — covers R1 *(Covers: R1)*
- [x] T2 — typo cites R99 *(Covers: R99)*
`)
	report, _ := ReviewSpec(dir, 1)
	codes := issueCodes(report.Issues)
	if !contains(codes, "sdd_spec_tid_dangling") {
		t.Errorf("expected dangling T-id warning, got %v", codes)
	}
	// Warnings alone shouldn't flip OK to false.
	if !report.OK {
		t.Errorf("warning-only report should keep OK=true")
	}
}

func TestReviewSpec_EARSMissingFails(t *testing.T) {
	dir := sddHarnessWithSpec(t, "auth", []string{"login works"})
	writeSpecFile(t, dir, "auth", "requirements.md", `# Requirements
## R1
The system shall log in users. (no EARS keyword here)

## Traceability
| login works | R1 |
`)
	writeSpecFile(t, dir, "auth", "tasks.md", `# Tasks
- [ ] T1 — login *(Covers: R1)*
`)
	report, _ := ReviewSpec(dir, 1)
	codes := issueCodes(report.Issues)
	if !contains(codes, "sdd_spec_ears_missing") {
		t.Errorf("expected EARS-missing issue, got %v", codes)
	}
}

func TestReviewSpec_AcceptanceWithoutTraceabilityRowFails(t *testing.T) {
	// feature_list has two acceptance bullets but only one is mapped
	// in the traceability table.
	dir := sddHarnessWithSpec(t, "auth", []string{"login works", "logout works"})
	writeSpecFile(t, dir, "auth", "requirements.md", `# Requirements
## R1
WHEN the user logs in THE SYSTEM SHALL mint a session.

## Traceability
| feature_list.json acceptance | Covered by |
|---|---|
| login works | R1 |
`)
	writeSpecFile(t, dir, "auth", "tasks.md", `# Tasks
- [ ] T1 — login *(Covers: R1)*
`)
	report, _ := ReviewSpec(dir, 1)
	codes := issueCodes(report.Issues)
	if !contains(codes, "sdd_spec_acceptance_untraced") {
		t.Errorf("expected untraced-acceptance error, got %v", codes)
	}
}

func TestReviewSpec_RejectsNonSDDFeature(t *testing.T) {
	// Standard (non-SDD) harness → review must refuse.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{Root: dir, Project: "x", Stack: StackGeneric}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, err := ReviewSpec(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "not SDD-flagged") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestReviewSpec_RejectsMissingFeature(t *testing.T) {
	dir := sddHarnessWithSpec(t, "auth", nil)
	_, err := ReviewSpec(dir, 99)
	if err == nil || !strings.Contains(err.Error(), "feature 99 not found") {
		t.Errorf("expected feature-not-found error, got %v", err)
	}
}

func TestReviewSpec_RejectsMissingSpecFiles(t *testing.T) {
	// Promote a feature past pending without scaffolding specs.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{Root: dir, Project: "x", Stack: StackGeneric, SDD: true}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	fl.Features[0].Status = StatusSpecReady
	_ = SaveFeatureList(dir, fl)
	_, err := ReviewSpec(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "spec files missing") {
		t.Errorf("expected missing-spec error, got %v", err)
	}
}

// ── parser unit tests ──────────────────────────────────────────────────────

func TestParseRequirements_ExtractsRIDsAndFlags(t *testing.T) {
	body := `# Requirements

## R1 — login
WHEN the user logs in THE SYSTEM SHALL mint a session.

## R2 — logout
The system shall log out. (no EARS)

## R3 — with-placeholder
WHEN <event> THE SYSTEM SHALL <do thing>.
`
	got := parseRequirements(body)
	if len(got) != 3 {
		t.Fatalf("R-id count = %d, want 3", len(got))
	}
	if !got[0].hasEARSForm || got[0].hasPlaceholder {
		t.Errorf("R1 should be EARS+clean, got %+v", got[0])
	}
	if got[1].hasEARSForm {
		t.Errorf("R2 should NOT have EARS, got %+v", got[1])
	}
	if !got[2].hasPlaceholder {
		t.Errorf("R3 should have placeholder, got %+v", got[2])
	}
}

func TestParseTasks_ExtractsTIDsAndCovers(t *testing.T) {
	body := `# Tasks
- [x] T1 — implement *(Covers: R1, R2)*
- [ ] T2 — test *(Covers: R3)*
- [x] T3 — closure (no covers tag)
`
	got := parseTasks(body)
	if len(got) != 3 {
		t.Fatalf("got %d tasks", len(got))
	}
	if len(got[0].Covers) != 2 || got[0].Covers[0] != "R1" || got[0].Covers[1] != "R2" {
		t.Errorf("T1 covers wrong: %+v", got[0])
	}
	if len(got[2].Covers) != 0 {
		t.Errorf("T3 should have no covers, got %+v", got[2])
	}
}

func TestParseTraceabilityTable_PicksRows(t *testing.T) {
	body := `## Traceability

| feature_list.json acceptance | Covered by |
|---|---|
| login works | R1 |
| logout works | R2, R3 |
`
	known := map[string]bool{"R1": true, "R2": true, "R3": true}
	got := parseTraceabilityTable(body, known)
	if rids := got["login works"]; len(rids) != 1 || rids[0] != "R1" {
		t.Errorf("login works coverage = %v", rids)
	}
	if rids := got["logout works"]; len(rids) != 2 {
		t.Errorf("logout works coverage = %v", rids)
	}
}

func TestParseTraceabilityTable_FiltersUnknownRIDs(t *testing.T) {
	body := `| login works | R1, R99 |`
	known := map[string]bool{"R1": true}
	got := parseTraceabilityTable(body, known)
	if len(got["login works"]) != 1 || got["login works"][0] != "R1" {
		t.Errorf("R99 should be filtered as unknown, got %v", got["login works"])
	}
}

func TestNormalizeAcceptance_StripsBackticks(t *testing.T) {
	if got := normalizeAcceptance("`./init.sh` exits 0"); got != "./init.sh exits 0" {
		t.Errorf("got %q", got)
	}
}

func TestFormatSpecReviewReport_HumanReadable(t *testing.T) {
	r := &SpecReviewReport{
		Project: "x", FeatureID: 1, Feature: "f",
		RIDs: []string{"R1"}, TIDs: []string{"T1"},
		Issues: []CheckIssue{{Severity: SeverityError, Code: "x", Message: "boom", Hint: "fix"}},
	}
	out := FormatSpecReviewReport(r)
	for _, want := range []string{"feature=#1", "R-ids: R1", "T-ids: T1", "✗", "→ fix"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatSpecReviewReport missing %q\nfull:\n%s", want, out)
		}
	}
}

// ─────────────────────────── Phase 18.A — verdict derivation ────────────────

func TestSpecReviewReport_Verdict(t *testing.T) {
	cases := []struct {
		name   string
		issues []CheckIssue
		want   ReviewVerdict
	}{
		{
			name:   "clean → approve",
			issues: nil,
			want:   VerdictApprove,
		},
		{
			name:   "only warnings → needs_fixes",
			issues: []CheckIssue{{Severity: SeverityWarning, Code: "x"}},
			want:   VerdictNeedsFixes,
		},
		{
			name:   "any error → reject",
			issues: []CheckIssue{{Severity: SeverityError, Code: "x"}},
			want:   VerdictReject,
		},
		{
			name: "errors win over warnings",
			issues: []CheckIssue{
				{Severity: SeverityWarning, Code: "w"},
				{Severity: SeverityError, Code: "e"},
			},
			want: VerdictReject,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := &SpecReviewReport{Issues: tc.issues}
			if got := r.Verdict(); got != tc.want {
				t.Errorf("Verdict() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpecReviewReport_CountBySeverity(t *testing.T) {
	r := &SpecReviewReport{Issues: []CheckIssue{
		{Severity: SeverityError},
		{Severity: SeverityWarning},
		{Severity: SeverityWarning},
		{Severity: SeverityError},
	}}
	errs, warns := r.CountBySeverity()
	if errs != 2 || warns != 2 {
		t.Errorf("errs=%d warns=%d, want 2/2", errs, warns)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func issueCodes(issues []CheckIssue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Code)
	}
	return out
}

func contains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}
