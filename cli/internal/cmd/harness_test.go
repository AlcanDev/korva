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
