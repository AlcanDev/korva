package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck_NoIssuesOnFreshHarness(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.OK || len(r.Issues) != 0 {
		t.Errorf("fresh harness should be clean, got %+v", r)
	}
	if r.SDDMode {
		t.Error("default harness should not advertise SDD mode")
	}
}

func TestCheck_NoIssuesOnFreshSDDHarness(t *testing.T) {
	// Fresh SDD harness has only the seed feature in `pending`. No spec
	// files are required yet (only past pending), so the report is clean.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.OK || len(r.Issues) != 0 {
		t.Errorf("fresh SDD harness should be clean (only pending feature), got %+v", r)
	}
	if !r.SDDMode {
		t.Error("SDD harness should advertise SDD mode")
	}
}

func TestCheck_FlagsSDDFeatureMissingSpecs(t *testing.T) {
	// SDD feature in spec_ready without spec files is a contract
	// violation: the state machine has it past the design gate, but the
	// design isn't on disk.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Promote the seed to spec_ready by hand (skipping MaterializeSpec).
	fl, _ := LoadFeatureList(dir)
	fl.Features[0].Status = StatusSpecReady
	if err := SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}

	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.OK {
		t.Error("expected OK=false")
	}
	if len(r.Issues) != 1 || r.Issues[0].Code != "sdd_spec_missing" {
		t.Errorf("expected single sdd_spec_missing issue, got %+v", r.Issues)
	}
	if r.Issues[0].Severity != SeverityError {
		t.Errorf("missing-spec should be error severity, got %v", r.Issues[0].Severity)
	}
	if r.Issues[0].FeatureID != 1 {
		t.Errorf("FeatureID should be 1, got %d", r.Issues[0].FeatureID)
	}
	if !strings.Contains(r.Issues[0].Hint, "korva harness spec 1") {
		t.Errorf("hint should suggest the next CLI step, got %q", r.Issues[0].Hint)
	}
}

func TestCheck_DoesNotRequireSpecsForPendingFeatures(t *testing.T) {
	// pending features don't need specs yet — they're not past the gate.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.OK {
		t.Errorf("pending SDD feature should not require specs: %+v", r.Issues)
	}
}

func TestCheck_PassesWhenSpecsArePresent(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	if _, err := MaterializeSpec(dir, &fl.Features[0], false); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	fl.Features[0].Status = StatusSpecReady
	if err := SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.OK || len(r.Issues) != 0 {
		t.Errorf("expected clean report with specs present, got %+v", r)
	}
}

func TestCheck_WarnsOnSDDFeatureOutsideSDDMode(t *testing.T) {
	// Standard (non-SDD) harness with an sdd:true feature is suspicious
	// — the state machine routes it correctly but the operator probably
	// meant to enable the SDD rule.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, // SDD: false
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	fl.Features[0].SDD = true
	if err := SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.HasErrors() {
		t.Error("warning-only report should not flag HasErrors")
	}
	if len(r.Issues) != 1 || r.Issues[0].Code != "sdd_feature_outside_sdd_mode" {
		t.Errorf("expected the sdd-mode warning, got %+v", r.Issues)
	}
	if r.Issues[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", r.Issues[0].Severity)
	}
	// OK must remain true: warnings don't fail the gate.
	if !r.OK {
		t.Error("warnings alone should not flip OK to false")
	}
}

func TestCheck_MissingFeatureListReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := Check(dir)
	if err == nil {
		t.Error("expected I/O error for missing feature_list.json")
	}
}

func TestCheck_AllStatesPastPendingNeedSpecs(t *testing.T) {
	// Re-run the missing-spec check across spec_ready / in_progress /
	// done. Each should flag the violation.
	for _, status := range []FeatureStatus{StatusSpecReady, StatusInProgress, StatusDone} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Generate(InitOptions{
				Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
			}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			fl, _ := LoadFeatureList(dir)
			fl.Features[0].Status = status
			if err := SaveFeatureList(dir, fl); err != nil {
				t.Fatalf("save: %v", err)
			}
			r, err := Check(dir)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if r.OK {
				t.Errorf("status=%s without specs should fail check", status)
			}
		})
	}
}

func TestCheck_BlockedDoesNotRequireSpecs(t *testing.T) {
	// `blocked` is a sideline state — operator may have flagged a
	// pending feature as blocked because they couldn't even draft the
	// spec. Don't punish that.
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	fl.Features[0].Status = StatusBlocked
	if err := SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}
	r, err := Check(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.OK {
		t.Errorf("blocked SDD feature should not need specs, got %+v", r.Issues)
	}
}

func TestFormatReport_HumanReadable(t *testing.T) {
	r := &CheckReport{
		Project: "x", SDDMode: true,
		Issues: []CheckIssue{
			{Severity: SeverityError, Code: "sdd_spec_missing", Message: "missing", Hint: "do thing"},
			{Severity: SeverityWarning, Code: "warn_x", Message: "warn"},
		},
	}
	out := FormatReport(r)
	for _, want := range []string{"mode=SDD", "✗", "⚠", "→ do thing"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatReport missing %q\nfull:\n%s", want, out)
		}
	}
}

func TestFormatReport_OKWhenNoIssues(t *testing.T) {
	r := &CheckReport{Project: "x"}
	out := FormatReport(r)
	if !strings.Contains(out, "no issues") {
		t.Errorf("clean report should say so: %s", out)
	}
}

// Sanity: I/O of the spec dir works through filepath helpers so the
// tests don't depend on exact platform separators.
func TestCheck_SpecsDirIsResolvable(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(InitOptions{
		Root: dir, Project: "x", Stack: StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	fl, _ := LoadFeatureList(dir)
	specDir := SpecDir(dir, fl.Features[0].Name)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range SpecFiles {
		if err := os.WriteFile(filepath.Join(specDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fl.Features[0].Status = StatusSpecReady
	_ = SaveFeatureList(dir, fl)
	r, _ := Check(dir)
	if !r.OK {
		t.Errorf("manually-created spec files should satisfy the check: %+v", r.Issues)
	}
}
