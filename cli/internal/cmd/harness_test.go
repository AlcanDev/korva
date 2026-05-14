package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/internal/harness"
)

// captureStdout swaps os.Stdout for a pipe, runs fn, then restores stdout
// and returns whatever fn wrote. The CLI uses fmt.Print* directly so we
// can't pass it a writer — this is how we read it back in tests.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	if err := fn(); err != nil {
		_ = w.Close()
		<-done
		t.Fatalf("fn: %v", err)
	}
	_ = w.Close()
	<-done
	return buf.String()
}

// initHarnessForTest is the common setup: a fresh tmp dir, with a
// minimal harness materialized and cwd switched. Returns the tmp path.
func initHarnessForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root:    dir,
		Project: "demo",
		Stack:   harness.StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Chdir(dir)
	return dir
}

func TestRunHarnessInit_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessInitOpts = harnessInitFlags{
		Root:        ".",
		Project:     "cli-demo",
		Description: "cli test",
		Stack:       "generic",
	}
	t.Cleanup(func() { harnessInitOpts = harnessInitFlags{} })

	out := captureStdout(t, func() error { return runHarnessInit(nil, nil) })

	if !strings.Contains(out, "Harness initialized") {
		t.Errorf("output missing success line: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature_list.json")); err != nil {
		t.Errorf("feature_list.json not created: %v", err)
	}
}

func TestRunHarnessInit_RejectsUnknownStack(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessInitOpts = harnessInitFlags{Root: ".", Project: "x", Stack: "rust"}
	t.Cleanup(func() { harnessInitOpts = harnessInitFlags{} })

	err := runHarnessInit(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown stack") {
		t.Errorf("expected unknown-stack error, got %v", err)
	}
}

func TestRunHarnessStatus_JSON(t *testing.T) {
	_ = initHarnessForTest(t)
	harnessStatusJSON = true
	t.Cleanup(func() { harnessStatusJSON = false })

	out := captureStdout(t, func() error { return runHarnessStatus(nil, nil) })

	var payload statusPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if payload.Project != "demo" {
		t.Errorf("project = %q", payload.Project)
	}
	if payload.Counts.Pending != 1 || payload.Counts.Total != 1 {
		t.Errorf("counts wrong: %+v", payload.Counts)
	}
	if payload.NextID != 1 {
		t.Errorf("next_pending_id = %d, want 1", payload.NextID)
	}
}

func TestRunHarnessStatus_PrettyShowsCounts(t *testing.T) {
	_ = initHarnessForTest(t)
	out := captureStdout(t, func() error { return runHarnessStatus(nil, nil) })

	for _, want := range []string{"Project:", "pending:", "Next pending"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull: %s", want, out)
		}
	}
}

func TestRunHarnessNext_PrintsSeedFeature(t *testing.T) {
	_ = initHarnessForTest(t)
	out := captureStdout(t, func() error { return runHarnessNext(nil, nil) })

	if !strings.Contains(out, "harness_smoke") {
		t.Errorf("expected seed feature in output: %s", out)
	}
	if !strings.Contains(out, "korva harness start 1") {
		t.Errorf("expected start hint: %s", out)
	}
}

func TestTransitionFlow_PendingToDone(t *testing.T) {
	_ = initHarnessForTest(t)

	// pending → in_progress
	start := transitionRunner(harness.StatusInProgress)
	out := captureStdout(t, func() error { return start(harnessStartCmd, []string{"1"}) })
	if !strings.Contains(out, "in_progress") {
		t.Errorf("start output: %s", out)
	}

	// State on disk reflects it.
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if fl.Features[0].Status != harness.StatusInProgress {
		t.Errorf("status after start = %s", fl.Features[0].Status)
	}
	if fl.Features[0].OwnerAgent == "" {
		t.Error("OwnerAgent not recorded")
	}
	if fl.Features[0].UpdatedAt == "" {
		t.Error("UpdatedAt not recorded")
	}

	// in_progress → done
	done := transitionRunner(harness.StatusDone)
	_ = captureStdout(t, func() error { return done(harnessDoneCmd, []string{"1"}) })

	fl2, _ := harness.LoadFeatureList(".")
	if fl2.Features[0].Status != harness.StatusDone {
		t.Errorf("status after done = %s", fl2.Features[0].Status)
	}
}

func TestTransition_RejectsIllegal(t *testing.T) {
	_ = initHarnessForTest(t)

	// Take the feature to done first.
	start := transitionRunner(harness.StatusInProgress)
	_ = captureStdout(t, func() error { return start(harnessStartCmd, []string{"1"}) })
	done := transitionRunner(harness.StatusDone)
	_ = captureStdout(t, func() error { return done(harnessDoneCmd, []string{"1"}) })

	// done → in_progress is illegal.
	again := transitionRunner(harness.StatusInProgress)
	if err := again(harnessStartCmd, []string{"1"}); err == nil {
		t.Error("expected illegal transition error, got nil")
	}
}

func TestTransition_RejectsBadID(t *testing.T) {
	_ = initHarnessForTest(t)
	start := transitionRunner(harness.StatusInProgress)
	if err := start(harnessStartCmd, []string{"not-a-number"}); err == nil {
		t.Error("expected parse error for non-numeric id")
	}
	if err := start(harnessStartCmd, []string{"999"}); err == nil {
		t.Error("expected not-found error for unknown id")
	}
}

func TestRunHarnessAdd_AppendsFeature(t *testing.T) {
	_ = initHarnessForTest(t)
	harnessAddOpts = harnessAddFlags{
		Name:        "new_thing",
		Title:       "Add a new thing",
		Description: "we need it",
		Acceptance:  []string{"thing exists", "test covers it"},
	}
	t.Cleanup(func() { harnessAddOpts = harnessAddFlags{} })

	out := captureStdout(t, func() error { return runHarnessAdd(nil, nil) })

	if !strings.Contains(out, "new_thing") {
		t.Errorf("output missing feature name: %s", out)
	}
	fl, _ := harness.LoadFeatureList(".")
	if len(fl.Features) != 2 {
		t.Fatalf("features = %d, want 2", len(fl.Features))
	}
	added := fl.Features[1]
	if added.ID != 2 {
		t.Errorf("added.ID = %d, want 2", added.ID)
	}
	if len(added.Acceptance) != 2 {
		t.Errorf("Acceptance = %v", added.Acceptance)
	}
	if added.Status != harness.StatusPending {
		t.Errorf("added.Status = %s, want pending", added.Status)
	}
}

func TestRunHarnessAdd_RequiresName(t *testing.T) {
	_ = initHarnessForTest(t)
	harnessAddOpts = harnessAddFlags{Title: "no name"}
	t.Cleanup(func() { harnessAddOpts = harnessAddFlags{} })

	if err := runHarnessAdd(nil, nil); err == nil {
		t.Error("expected --name required error")
	}
}

func TestRunHarnessList_RendersMarkers(t *testing.T) {
	_ = initHarnessForTest(t)
	out := captureStdout(t, func() error { return runHarnessList(nil, nil) })

	if !strings.Contains(out, "pending") {
		t.Errorf("list output should include pending row: %s", out)
	}
	if !strings.Contains(out, "#1") {
		t.Errorf("list output should include id: %s", out)
	}
}

// ───────────────────────── Phase 13.2 — SDD CLI tests ─────────────────────────

// initSDDHarnessForTest is the SDD counterpart of initHarnessForTest:
// fresh tmp dir, materialize an SDD harness, chdir in.
func initSDDHarnessForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root:    dir,
		Project: "sdd-demo",
		Stack:   harness.StackGeneric,
		SDD:     true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Chdir(dir)
	return dir
}

func TestRunHarnessInit_SDDFlagSetsRule(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessInitOpts = harnessInitFlags{
		Root: ".", Project: "sdd-init", Stack: "generic", Editors: "none", SDD: true,
	}
	t.Cleanup(func() { harnessInitOpts = harnessInitFlags{} })

	out := captureStdout(t, func() error { return runHarnessInit(nil, nil) })

	if !strings.Contains(out, "mode: SDD") {
		t.Errorf("output should announce SDD mode: %s", out)
	}
	fl, err := harness.LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !fl.Rules.RequireApprovedSpecToImplement {
		t.Error("--sdd should set RequireApprovedSpecToImplement")
	}
	if !fl.Features[0].SDD {
		t.Error("seed feature should be sdd:true")
	}
}

func TestRunHarnessAdd_SDDFlagPersists(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	harnessAddOpts = harnessAddFlags{
		Name: "auth_layer", Title: "Auth", Acceptance: []string{"works"}, SDD: true,
	}
	t.Cleanup(func() { harnessAddOpts = harnessAddFlags{} })

	out := captureStdout(t, func() error { return runHarnessAdd(nil, nil) })

	if !strings.Contains(out, "SDD-gated") {
		t.Errorf("output should announce SDD gating: %s", out)
	}
	fl, _ := harness.LoadFeatureList(".")
	if len(fl.Features) != 2 {
		t.Fatalf("features = %d, want 2", len(fl.Features))
	}
	added := fl.Features[1]
	if !added.SDD {
		t.Error("added feature should be sdd:true")
	}
}

func TestRunHarnessSpec_CreatesAllFiles(t *testing.T) {
	dir := initSDDHarnessForTest(t)

	out := captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	if !strings.Contains(out, "Spec materialized") {
		t.Errorf("expected success line: %s", out)
	}
	for _, f := range harness.SpecFiles {
		if _, err := os.Stat(filepath.Join(dir, "specs", "harness_smoke", f)); err != nil {
			t.Errorf("spec file %s not created: %v", f, err)
		}
	}
}

func TestRunHarnessSpec_RejectsNonSDDFeature(t *testing.T) {
	// Standard (non-SDD) harness → harness spec should refuse.
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root: dir, Project: "plain", Stack: harness.StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Chdir(dir)

	err := runHarnessSpec(nil, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "not SDD-flagged") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestRunHarnessSpec_RejectsBadID(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	if err := runHarnessSpec(nil, []string{"not-a-number"}); err == nil {
		t.Error("expected parse error for non-numeric id")
	}
	if err := runHarnessSpec(nil, []string{"999"}); err == nil {
		t.Error("expected not-found error for unknown id")
	}
}

func TestRunHarnessSpec_IdempotentByDefault(t *testing.T) {
	dir := initSDDHarnessForTest(t)
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	// Overwrite requirements.md with operator content.
	reqPath := filepath.Join(dir, "specs", "harness_smoke", "requirements.md")
	if err := os.WriteFile(reqPath, []byte("OPERATOR"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })
	if !strings.Contains(out, "Kept existing") {
		t.Errorf("second call should report kept files: %s", out)
	}
	body, _ := os.ReadFile(reqPath)
	if string(body) != "OPERATOR" {
		t.Errorf("operator content was overwritten: %s", body)
	}
}

func TestRunHarnessReady_RejectsWithoutSpecFiles(t *testing.T) {
	_ = initSDDHarnessForTest(t)

	err := runHarnessReady(harnessReadyCmd, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "spec files missing") {
		t.Errorf("expected spec-missing error, got %v", err)
	}
}

func TestRunHarnessReady_HappyPath(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	// Scaffold + transition.
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	out := captureStdout(t, func() error { return runHarnessReady(harnessReadyCmd, []string{"1"}) })
	if !strings.Contains(out, "spec_ready") {
		t.Errorf("expected spec_ready transition: %s", out)
	}

	fl, _ := harness.LoadFeatureList(".")
	if fl.Features[0].Status != harness.StatusSpecReady {
		t.Errorf("status = %s, want spec_ready", fl.Features[0].Status)
	}
	if fl.Features[0].OwnerAgent == "" {
		t.Error("OwnerAgent should be recorded")
	}
}

func TestRunHarnessReady_RejectsNonSDDFeature(t *testing.T) {
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root: dir, Project: "plain", Stack: harness.StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Chdir(dir)

	err := runHarnessReady(harnessReadyCmd, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "not SDD-flagged") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestRunHarnessStatus_SDDOutputShowsSpecReadyRow(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })
	_ = captureStdout(t, func() error { return runHarnessReady(harnessReadyCmd, []string{"1"}) })

	out := captureStdout(t, func() error { return runHarnessStatus(nil, nil) })

	if !strings.Contains(out, "Mode:       SDD") {
		t.Errorf("SDD harness status should mention Mode: SDD: %s", out)
	}
	if !strings.Contains(out, "spec_ready:  1") {
		t.Errorf("spec_ready row missing: %s", out)
	}
	if !strings.Contains(out, "Awaiting approval: #1") {
		t.Errorf("should hint at the awaiting approval row: %s", out)
	}
}

func TestRunHarnessStatus_StandardOutputOmitsSpecReadyRow(t *testing.T) {
	_ = initHarnessForTest(t)
	out := captureStdout(t, func() error { return runHarnessStatus(nil, nil) })

	if strings.Contains(out, "spec_ready") {
		t.Errorf("standard harness status should NOT mention spec_ready: %s", out)
	}
	if strings.Contains(out, "Mode:") {
		t.Errorf("standard harness should not advertise a mode line: %s", out)
	}
}

func TestRunHarnessStatus_SDD_JSONIncludesNextSpecReadyID(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })
	_ = captureStdout(t, func() error { return runHarnessReady(harnessReadyCmd, []string{"1"}) })

	harnessStatusJSON = true
	t.Cleanup(func() { harnessStatusJSON = false })

	out := captureStdout(t, func() error { return runHarnessStatus(nil, nil) })

	var payload statusPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if !payload.SDD {
		t.Error("payload.sdd should be true")
	}
	if payload.NextSpecReadyID != 1 {
		t.Errorf("next_spec_ready_id = %d, want 1", payload.NextSpecReadyID)
	}
	if payload.Counts.SpecReady != 1 {
		t.Errorf("counts.spec_ready = %d, want 1", payload.Counts.SpecReady)
	}
}

func TestTransition_StartRespectsSDDGate(t *testing.T) {
	// In SDD mode, `harness start <id>` against a pending feature should
	// surface the friendly "run harness ready first" error.
	_ = initSDDHarnessForTest(t)

	start := transitionRunner(harness.StatusInProgress)
	err := start(harnessStartCmd, []string{"1"})
	if err == nil {
		t.Fatal("expected SDD gate error")
	}
	if !strings.Contains(err.Error(), "harness ready") && !strings.Contains(err.Error(), "spec_ready") {
		t.Errorf("error should hint at the missing step: %v", err)
	}
}

func TestStatusMarker_SpecReady(t *testing.T) {
	if got := statusMarker(harness.StatusSpecReady); got != "✎" {
		t.Errorf("statusMarker(spec_ready) = %q, want ✎", got)
	}
}

func TestRunHarnessCheck_PassesOnFreshHarness(t *testing.T) {
	_ = initHarnessForTest(t)
	out := captureStdout(t, func() error { return runHarnessCheck(nil, nil) })
	if !strings.Contains(out, "no issues") {
		t.Errorf("expected clean check output: %s", out)
	}
}

func TestRunHarnessCheck_FailsOnSDDSpecMissing(t *testing.T) {
	dir := initSDDHarnessForTest(t)
	// Promote pending → spec_ready by hand without the spec files.
	fl, _ := harness.LoadFeatureList(dir)
	fl.Features[0].Status = harness.StatusSpecReady
	if err := harness.SaveFeatureList(dir, fl); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := runHarnessCheck(nil, nil)
	if err == nil {
		t.Fatal("expected non-zero exit when SDD specs are missing")
	}
	if !strings.Contains(err.Error(), "harness check failed") {
		t.Errorf("error should be the sentinel, got %v", err)
	}
}

func TestRunHarnessCheck_JSON(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	harnessCheckJSON = true
	t.Cleanup(func() { harnessCheckJSON = false })

	out := captureStdout(t, func() error { return runHarnessCheck(nil, nil) })

	var report harness.CheckReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if !report.OK {
		t.Errorf("fresh SDD harness should be OK: %+v", report)
	}
	if !report.SDDMode {
		t.Error("sdd_mode flag should be true")
	}
}

// ───────────────────────── Phase 15.B — `harness review` ─────────────────────────

func TestRunHarnessReview_PassesOnCleanSpec(t *testing.T) {
	dir := initSDDHarnessForTest(t)
	// Materialize the per-feature spec dir first (the SDD harness seed
	// only creates specs/SPEC-TEMPLATE/, not specs/harness_smoke/).
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	// Replace placeholder-laden templates with a clean spec covering
	// every acceptance bullet the SDD seed carries (4 bullets).
	specDir := filepath.Join(dir, "specs", "harness_smoke")
	if err := os.WriteFile(filepath.Join(specDir, "requirements.md"), []byte(`# Requirements
## R1
WHEN init.sh runs THE SYSTEM SHALL exit 0.
## R2
WHEN feature_list.json loads THE SYSTEM SHALL validate.
## R3
WHEN the harness starts THE SYSTEM SHALL render progress/current.md.
## R4
WHEN the SDD harness initializes THE SYSTEM SHALL materialize the three spec files.

## Traceability
| acceptance | Covered by |
|---|---|
| `+"`./init.sh` exits with code 0"+` | R1 |
| `+"`feature_list.json` validates"+` | R2 |
| `+"`progress/current.md` exists"+` | R3 |
| `+"`specs/harness_smoke/{requirements,design,tasks}.md` exist"+` | R4 |
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(`# Tasks
- [x] T1 — init.sh exits 0 *(Covers: R1)*
- [x] T2 — feature_list validates *(Covers: R2)*
- [x] T3 — progress exists *(Covers: R3)*
- [x] T4 — specs materialized *(Covers: R4)*
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error { return runHarnessReview(nil, []string{"1"}) })
	if !strings.Contains(out, "no issues") {
		t.Errorf("clean spec should pass: %s", out)
	}
}

func TestRunHarnessReview_FailsOnFreshScaffold(t *testing.T) {
	// Right after MaterializeSpec the placeholders are still there.
	_ = initSDDHarnessForTest(t)
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	err := runHarnessReview(nil, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "spec review failed") {
		t.Errorf("expected fail-fast error, got %v", err)
	}
}

func TestRunHarnessReview_JSONShape(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	_ = captureStdout(t, func() error { return runHarnessSpec(nil, []string{"1"}) })

	harnessReviewJSON = true
	t.Cleanup(func() { harnessReviewJSON = false })

	out := captureStdout(t, func() error {
		// Errors return non-nil but stdout has the JSON anyway.
		_ = runHarnessReview(nil, []string{"1"})
		return nil
	})
	var report harness.SpecReviewReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if report.FeatureID != 1 {
		t.Errorf("feature_id = %d", report.FeatureID)
	}
	if report.OK {
		t.Errorf("fresh scaffold should report OK=false")
	}
}

func TestRunHarnessReview_RejectsBadID(t *testing.T) {
	_ = initSDDHarnessForTest(t)
	if err := runHarnessReview(nil, []string{"not-a-number"}); err == nil {
		t.Error("expected parse error")
	}
	if err := runHarnessReview(nil, []string{"999"}); err == nil {
		t.Error("expected not-found error")
	}
}

// ───────────────────────── Phase 15.A — `harness ci install` ─────────────────────────

func TestRunHarnessCIInstall_GitHubActions(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{
		Root: ".", Provider: "github-actions",
	}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	out := captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })

	if !strings.Contains(out, "CI installed") {
		t.Errorf("expected success line: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "harness.yml")); err != nil {
		t.Errorf("workflow not on disk: %v", err)
	}
}

func TestRunHarnessCIInstall_GitLab(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{
		Root: ".", Provider: "gitlab-ci",
	}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	out := captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })

	if !strings.Contains(out, "CI installed") {
		t.Errorf("expected success line: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.harness.yml")); err != nil {
		t.Errorf("gitlab yml not on disk: %v", err)
	}
	// GitLab variant tells the operator to set the access token.
	if !strings.Contains(out, "KORVA_GITLAB_TOKEN") {
		t.Errorf("gitlab post-install hint missing: %s", out)
	}
}

func TestRunHarnessCIInstall_AutoDetectFromGitHubDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{Root: "."}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	_ = captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "harness.yml")); err != nil {
		t.Errorf("auto-detect should have picked github-actions: %v", err)
	}
}

func TestRunHarnessCIInstall_AutoDetectFromGitLabYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{Root: "."}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	_ = captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.harness.yml")); err != nil {
		t.Errorf("auto-detect should have picked gitlab-ci: %v", err)
	}
}

func TestRunHarnessCIInstall_AutoDetectFailsWithoutSignal(t *testing.T) {
	// Neither .github/ nor .gitlab-ci.yml present → error.
	dir := t.TempDir()
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{Root: "."}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	err := runHarnessCIInstall(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "auto-detect") {
		t.Errorf("expected auto-detect error, got %v", err)
	}
}

func TestRunHarnessCIInstall_RejectsUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{Root: ".", Provider: "circleci"}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })

	err := runHarnessCIInstall(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected unknown-provider error, got %v", err)
	}
}

func TestRunHarnessCIInstall_IdempotentKeepsOperatorEdits(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessCIInstallOpts = harnessCIInstallFlags{Root: ".", Provider: "github-actions"}
	t.Cleanup(func() { harnessCIInstallOpts = harnessCIInstallFlags{} })
	_ = captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })

	dest := filepath.Join(dir, ".github", "workflows", "harness.yml")
	if err := os.WriteFile(dest, []byte("OPERATOR"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error { return runHarnessCIInstall(nil, nil) })
	if !strings.Contains(out, "Kept existing") {
		t.Errorf("re-run should report kept files: %s", out)
	}
	body, _ := os.ReadFile(dest)
	if string(body) != "OPERATOR" {
		t.Errorf("operator content was overwritten: %s", body)
	}
}

func TestDetectStack(t *testing.T) {
	tests := []struct {
		name string
		seed map[string]string
		want harness.Stack
	}{
		{"go.mod", map[string]string{"go.mod": "module x"}, harness.StackGo},
		{"package.json", map[string]string{"package.json": "{}"}, harness.StackTS},
		{"pyproject.toml", map[string]string{"pyproject.toml": ""}, harness.StackPython},
		{"empty dir", nil, harness.StackGeneric},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.seed {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := detectStack(dir); got != tc.want {
				t.Errorf("detectStack = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseEditorsFlag_AutoUsesDetect(t *testing.T) {
	dir := t.TempDir()
	// Drop a Cursor marker so auto-detect picks it up.
	if err := os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseEditorsFlag("auto", dir)
	if err != nil {
		t.Fatalf("parseEditorsFlag: %v", err)
	}
	if len(got) != 1 || got[0] != harness.EditorCursor {
		t.Errorf("auto detect = %v, want [cursor]", got)
	}
}

func TestParseEditorsFlag_EmptySameAsAuto(t *testing.T) {
	dir := t.TempDir()
	got, err := parseEditorsFlag("", dir)
	if err != nil {
		t.Fatalf("parseEditorsFlag: %v", err)
	}
	// Empty dir → fallback to claude.
	if len(got) != 1 || got[0] != harness.EditorClaude {
		t.Errorf("empty flag = %v, want [claude]", got)
	}
}

func TestParseEditorsFlag_None(t *testing.T) {
	got, err := parseEditorsFlag("none", t.TempDir())
	if err != nil {
		t.Fatalf("parseEditorsFlag: %v", err)
	}
	if got != nil {
		t.Errorf("'none' should produce nil slice, got %v", got)
	}
}

func TestParseEditorsFlag_CSV(t *testing.T) {
	got, err := parseEditorsFlag("claude,cursor,continue", t.TempDir())
	if err != nil {
		t.Fatalf("parseEditorsFlag: %v", err)
	}
	want := []harness.Editor{harness.EditorClaude, harness.EditorCursor, harness.EditorContinue}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseEditorsFlag_Dedupes(t *testing.T) {
	got, err := parseEditorsFlag("cursor, cursor ,claude,cursor", t.TempDir())
	if err != nil {
		t.Fatalf("parseEditorsFlag: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected dedupe to 2 entries, got %v", got)
	}
}

func TestParseEditorsFlag_RejectsUnknown(t *testing.T) {
	_, err := parseEditorsFlag("emacs", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "emacs") {
		t.Errorf("expected unknown-editor error, got %v", err)
	}
}

// ───────────────────────── Phase 16.B — harness detect ─────────────────────

func TestRunHarnessDetect_EmptyDirShowsFallback(t *testing.T) {
	dir := t.TempDir()
	harnessDetectOpts = harnessDetectFlags{Root: dir}
	t.Cleanup(func() { harnessDetectOpts = harnessDetectFlags{} })

	out := captureStdout(t, func() error { return runHarnessDetect(nil, nil) })
	if !strings.Contains(out, "falling back to claude default") {
		t.Errorf("missing fallback hint in:\n%s", out)
	}
	// The fallback branch still lists what init would install.
	if !strings.Contains(out, "AGENTS.md") {
		t.Errorf("output should preview AGENTS.md install:\n%s", out)
	}
}

func TestRunHarnessDetect_TextEnumeratesHitsAndFiles(t *testing.T) {
	dir := t.TempDir()
	// Seed two markers: aider + codex.
	if err := os.WriteFile(filepath.Join(dir, ".aider.conf.yml"), []byte("read: AGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	harnessDetectOpts = harnessDetectFlags{Root: dir}
	t.Cleanup(func() { harnessDetectOpts = harnessDetectFlags{} })

	out := captureStdout(t, func() error { return runHarnessDetect(nil, nil) })
	for _, want := range []string{
		"aider", "codex",
		".aider.conf.yml",     // marker for aider
		".codex",              // marker for codex
		".codex/config.toml",  // would-install path for codex
		"AGENTS.md",           // common layer always listed
		"feature_list.json",   // generated seed
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// "fallback" must NOT appear when at least one marker matched.
	if strings.Contains(out, "falling back") {
		t.Errorf("non-empty detection should not show fallback hint:\n%s", out)
	}
}

func TestRunHarnessDetect_JSONShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	harnessDetectOpts = harnessDetectFlags{Root: dir, JSON: true}
	t.Cleanup(func() { harnessDetectOpts = harnessDetectFlags{} })

	out := captureStdout(t, func() error { return runHarnessDetect(nil, nil) })

	var payload struct {
		Root        string `json:"root"`
		SDD         bool   `json:"sdd"`
		DefaultUsed bool   `json:"default_used"`
		Hits        []struct {
			Editor string `json:"editor"`
			Marker string `json:"marker"`
		} `json:"hits"`
		CommonFiles []string `json:"common_files"`
		Editors     []struct {
			Editor string   `json:"editor"`
			Files  []string `json:"files"`
		} `json:"editors"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if payload.DefaultUsed {
		t.Errorf("DefaultUsed should be false when a marker hit")
	}
	if len(payload.Hits) != 1 || payload.Hits[0].Editor != "cursor" {
		t.Errorf("hits = %v, want [{cursor .cursorrules}]", payload.Hits)
	}
	if payload.Hits[0].Marker != ".cursorrules" {
		t.Errorf("marker = %q, want .cursorrules", payload.Hits[0].Marker)
	}
	if len(payload.Editors) != 1 || payload.Editors[0].Editor != "cursor" {
		t.Errorf("editors preview = %v", payload.Editors)
	}
	// Universal layer must always be listed regardless of editor.
	var sawAgentsMd bool
	for _, p := range payload.CommonFiles {
		if p == "AGENTS.md" {
			sawAgentsMd = true
		}
	}
	if !sawAgentsMd {
		t.Errorf("common_files missing AGENTS.md: %v", payload.CommonFiles)
	}
}

func TestRunHarnessDetect_JSONFallbackBranch(t *testing.T) {
	dir := t.TempDir()
	harnessDetectOpts = harnessDetectFlags{Root: dir, JSON: true}
	t.Cleanup(func() { harnessDetectOpts = harnessDetectFlags{} })

	out := captureStdout(t, func() error { return runHarnessDetect(nil, nil) })

	var payload struct {
		DefaultUsed  bool   `json:"default_used"`
		DefaultLabel string `json:"default_label"`
		Hits         []any  `json:"hits"`
		Editors      []struct {
			Editor string `json:"editor"`
		} `json:"editors"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !payload.DefaultUsed {
		t.Error("DefaultUsed should be true for empty dir")
	}
	if payload.DefaultLabel != "claude" {
		t.Errorf("DefaultLabel = %q, want claude", payload.DefaultLabel)
	}
	if len(payload.Hits) != 0 {
		t.Errorf("Hits should be empty in fallback, got %v", payload.Hits)
	}
	if len(payload.Editors) != 1 || payload.Editors[0].Editor != "claude" {
		t.Errorf("fallback editors preview = %v, want [{claude ...}]", payload.Editors)
	}
}

func TestRunHarnessDetect_SDDFlagAddsSpecAuthor(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	harnessDetectOpts = harnessDetectFlags{Root: dir, SDD: true, JSON: true}
	t.Cleanup(func() { harnessDetectOpts = harnessDetectFlags{} })

	out := captureStdout(t, func() error { return runHarnessDetect(nil, nil) })

	var payload struct {
		Editors []struct {
			Editor string   `json:"editor"`
			Files  []string `json:"files"`
		} `json:"editors"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(payload.Editors) != 1 {
		t.Fatalf("editors = %v", payload.Editors)
	}
	var hasSpecAuthor bool
	for _, p := range payload.Editors[0].Files {
		if strings.Contains(p, "spec_author") {
			hasSpecAuthor = true
		}
	}
	if !hasSpecAuthor {
		t.Errorf("--sdd should include spec_author file in preview, got %v",
			payload.Editors[0].Files)
	}
}

// TestDetectHelpListsAllEditors guards against drift between
// allEditorSpecsForHelp() and harness.AllEditors: every editor in the
// canonical list must appear in the help text the fallback branch
// shows when nothing was detected.
func TestDetectHelpListsAllEditors(t *testing.T) {
	help := allEditorSpecsForHelp()
	if len(help) != len(harness.AllEditors) {
		t.Fatalf("allEditorSpecsForHelp has %d entries, harness.AllEditors has %d — keep them in sync",
			len(help), len(harness.AllEditors))
	}
	seen := make(map[string]bool, len(help))
	for _, h := range help {
		seen[h.editor] = true
	}
	for _, e := range harness.AllEditors {
		if !seen[string(e)] {
			t.Errorf("editor %q missing from allEditorSpecsForHelp", e)
		}
	}
}

func TestRunHarnessInit_EditorsAutoDetectsCursor(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// Seed a Cursor marker.
	if err := os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	harnessInitOpts = harnessInitFlags{
		Root: ".", Project: "p", Stack: "generic", Editors: "auto",
	}
	t.Cleanup(func() { harnessInitOpts = harnessInitFlags{} })

	_ = captureStdout(t, func() error { return runHarnessInit(nil, nil) })

	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "korva-harness.mdc")); err != nil {
		t.Errorf("auto-detect should have installed cursor rule: %v", err)
	}
}

func TestRunHarnessInit_EditorsNoneSkipsAll(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	harnessInitOpts = harnessInitFlags{
		Root: ".", Project: "p", Stack: "generic", Editors: "none",
	}
	t.Cleanup(func() { harnessInitOpts = harnessInitFlags{} })

	_ = captureStdout(t, func() error { return runHarnessInit(nil, nil) })

	// Universal layer still landed.
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md missing: %v", err)
	}
	// Editor-specific files did not.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "leader.md")); !os.IsNotExist(err) {
		t.Errorf("'none' should have skipped claude agents: %v", err)
	}
}
