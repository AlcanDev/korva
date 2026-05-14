package mcp

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/internal/harness"
	"github.com/alcandev/korva/vault/internal/store"
)

// newHarnessTestServer is a minimal Server with admin profile and an
// in-memory store. The tools we exercise don't touch the store, but the
// dispatch logger expects one.
func newHarnessTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return &Server{
		store:   s,
		reader:  bufio.NewReader(strings.NewReader("")),
		writer:  &bytes.Buffer{},
		logger:  log.New(bytes.NewBuffer(nil), "", 0),
		profile: ProfileAdmin,
	}
}

// initHarness lays down a fresh harness in a tmp dir and returns the path.
// Used as the starting state for every transition / read test.
func initHarness(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root:    dir,
		Project: "mcp-test",
		Stack:   harness.StackGeneric,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return dir
}

func TestToolHarnessInit_WritesFiles(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()

	res, err := srv.toolHarnessInit(map[string]any{
		"root":        dir,
		"project":     "via-mcp",
		"description": "from MCP",
		"stack":       "go",
		"editors":     []any{"claude"},
	})
	if err != nil {
		t.Fatalf("toolHarnessInit: %v", err)
	}
	resp := res.(map[string]any)
	if resp["project"] != "via-mcp" {
		t.Errorf("project = %v", resp["project"])
	}
	files := resp["files_written"].([]string)
	if len(files) == 0 {
		t.Errorf("no files reported written")
	}
	editors := resp["editors"].([]string)
	if len(editors) != 1 || editors[0] != "claude" {
		t.Errorf("editors = %v, want [claude]", editors)
	}

	// Verify the seed feature_list.json is real.
	fl, err := harness.LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if fl.Project != "via-mcp" {
		t.Errorf("seed project = %q", fl.Project)
	}
}

func TestToolHarnessInit_EditorsAutoString(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	// Plant a Cursor marker so auto picks it up.
	if err := os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := srv.toolHarnessInit(map[string]any{
		"root":    dir,
		"project": "p",
		"editors": "auto",
	})
	if err != nil {
		t.Fatalf("toolHarnessInit: %v", err)
	}
	editors := res.(map[string]any)["editors"].([]string)
	if len(editors) != 1 || editors[0] != "cursor" {
		t.Errorf("auto editors = %v, want [cursor]", editors)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "korva-harness.mdc")); err != nil {
		t.Errorf("cursor rule not materialized: %v", err)
	}
}

func TestToolHarnessInit_EditorsNone(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	res, err := srv.toolHarnessInit(map[string]any{
		"root":    dir,
		"project": "p",
		"editors": "none",
	})
	if err != nil {
		t.Fatalf("toolHarnessInit: %v", err)
	}
	editors := res.(map[string]any)["editors"].([]string)
	if len(editors) != 0 {
		t.Errorf("'none' should produce empty editors slice, got %v", editors)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "leader.md")); !os.IsNotExist(err) {
		t.Errorf("claude agent file unexpectedly created: %v", err)
	}
}

func TestToolHarnessInit_EditorsCSVString(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	if _, err := srv.toolHarnessInit(map[string]any{
		"root":    dir,
		"project": "p",
		"editors": "cursor,copilot",
	}); err != nil {
		t.Fatalf("toolHarnessInit: %v", err)
	}
	for _, p := range []string{
		".cursor/rules/korva-harness.mdc",
		".github/copilot-instructions.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestToolHarnessInit_EditorsUnknownErrors(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	_, err := srv.toolHarnessInit(map[string]any{
		"root":    dir,
		"project": "p",
		"editors": []any{"emacs"},
	})
	if err == nil || !strings.Contains(err.Error(), "emacs") {
		t.Errorf("expected error naming the unknown editor, got %v", err)
	}
}

func TestParseEditorsArg_Shapes(t *testing.T) {
	root := t.TempDir()
	// Empty / missing arg → auto-detect.
	if _, err := parseEditorsArg(map[string]any{}, root); err != nil {
		t.Errorf("missing arg should not error: %v", err)
	}
	// Nil value → auto-detect.
	if _, err := parseEditorsArg(map[string]any{"editors": nil}, root); err != nil {
		t.Errorf("nil arg should not error: %v", err)
	}
	// Array of strings.
	got, err := parseEditorsArg(map[string]any{"editors": []any{"claude", "cursor"}}, root)
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("array got %v", got)
	}
	// Unsupported type.
	if _, err := parseEditorsArg(map[string]any{"editors": 42}, root); err == nil {
		t.Error("numeric editors should error")
	}
}

func TestToolHarnessInit_RequiresProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	_, err := srv.toolHarnessInit(map[string]any{"root": t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "project is required") {
		t.Errorf("expected project-required error, got %v", err)
	}
}

func TestToolHarnessStatus_ReturnsCounts(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	res, err := srv.toolHarnessStatus(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	resp := res.(map[string]any)
	if resp["project"] != "mcp-test" {
		t.Errorf("project = %v", resp["project"])
	}
	c := resp["counts"].(harness.Counts)
	if c.Pending != 1 || c.Total != 1 {
		t.Errorf("counts wrong: %+v", c)
	}
	next := resp["next_pending"].(map[string]any)
	if next["id"].(int) != 1 {
		t.Errorf("next_pending id = %v", next["id"])
	}
}

func TestToolHarnessNext_ReturnsSeedFeature(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	res, err := srv.toolHarnessNext(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	resp := res.(map[string]any)
	next := resp["next_pending"].(map[string]any)
	if next["name"].(string) != "harness_smoke" {
		t.Errorf("name = %v", next["name"])
	}
	accept, ok := next["acceptance"].([]string)
	if !ok || len(accept) == 0 {
		t.Errorf("acceptance missing: %+v", next)
	}
}

func TestToolHarnessNext_NullWhenEmpty(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	// Take the only feature to done so the backlog is clear.
	_, _ = srv.toolHarnessTransition(harness.StatusInProgress)(map[string]any{"root": root, "id": float64(1)})
	_, _ = srv.toolHarnessTransition(harness.StatusDone)(map[string]any{"root": root, "id": float64(1)})

	res, err := srv.toolHarnessNext(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	resp := res.(map[string]any)
	if resp["next_pending"] != nil {
		t.Errorf("expected next_pending=nil, got %+v", resp["next_pending"])
	}
}

func TestToolHarnessList_FilterByStatus(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	// Add a second feature.
	if _, err := srv.toolHarnessAdd(map[string]any{
		"root":       root,
		"name":       "second",
		"title":      "Second",
		"acceptance": []any{"works"},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	// All features
	all, err := srv.toolHarnessList(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if got := len(all.(map[string]any)["features"].([]map[string]any)); got != 2 {
		t.Errorf("unfiltered list = %d features, want 2", got)
	}

	// Filter to pending — both are pending, should still be 2.
	pending, err := srv.toolHarnessList(map[string]any{"root": root, "status": "pending"})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if got := len(pending.(map[string]any)["features"].([]map[string]any)); got != 2 {
		t.Errorf("pending list = %d features, want 2", got)
	}

	// Filter to done — none are done.
	done, _ := srv.toolHarnessList(map[string]any{"root": root, "status": "done"})
	if got := len(done.(map[string]any)["features"].([]map[string]any)); got != 0 {
		t.Errorf("done list = %d features, want 0", got)
	}
}

func TestToolHarnessTransition_FullFlow(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	// pending → in_progress
	r, err := srv.toolHarnessTransition(harness.StatusInProgress)(map[string]any{
		"root":  root,
		"id":    float64(1),
		"agent": "test-agent",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if r.(map[string]any)["status"] != "in_progress" {
		t.Errorf("status = %v", r.(map[string]any)["status"])
	}
	if r.(map[string]any)["owner"] != "test-agent" {
		t.Errorf("owner = %v", r.(map[string]any)["owner"])
	}

	// in_progress → done
	r2, err := srv.toolHarnessTransition(harness.StatusDone)(map[string]any{
		"root": root,
		"id":   float64(1),
	})
	if err != nil {
		t.Fatalf("done: %v", err)
	}
	if r2.(map[string]any)["status"] != "done" {
		t.Errorf("done status = %v", r2.(map[string]any)["status"])
	}
}

func TestToolHarnessTransition_RejectsIllegal(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	// Take the feature to done.
	_, _ = srv.toolHarnessTransition(harness.StatusInProgress)(map[string]any{"root": root, "id": float64(1)})
	_, _ = srv.toolHarnessTransition(harness.StatusDone)(map[string]any{"root": root, "id": float64(1)})

	// done → in_progress should fail.
	_, err := srv.toolHarnessTransition(harness.StatusInProgress)(map[string]any{"root": root, "id": float64(1)})
	if err == nil {
		t.Error("expected illegal-transition error")
	}
}

func TestToolHarnessTransition_RequiresID(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	_, err := srv.toolHarnessTransition(harness.StatusInProgress)(map[string]any{"root": root})
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Errorf("expected id-required error, got %v", err)
	}
}

func TestReadIDArg_AcceptsStringAndNumber(t *testing.T) {
	// JSON-RPC clients normally pass float64; string is the fallback path
	// for agents that send the id as text. Validate both work.
	cases := []struct {
		name string
		args map[string]any
		want int
	}{
		{"float64", map[string]any{"id": float64(42)}, 42},
		{"int", map[string]any{"id": 17}, 17},
		{"string", map[string]any{"id": "9"}, 9},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := readIDArg(tc.args)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Errorf("readIDArg = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReadIDArg_RejectsBadInput(t *testing.T) {
	if _, err := readIDArg(map[string]any{}); err == nil {
		t.Error("expected error when id missing")
	}
	if _, err := readIDArg(map[string]any{"id": "abc"}); err == nil {
		t.Error("expected error for non-numeric string")
	}
}

func TestToolHarnessAdd_AppendsFeature(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	res, err := srv.toolHarnessAdd(map[string]any{
		"root":        root,
		"name":        "auth_layer",
		"title":       "Build auth layer",
		"description": "JWT + refresh",
		"acceptance":  []any{"login works", "refresh works"},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	feature := res.(map[string]any)["feature"].(map[string]any)
	if feature["id"] != 2 {
		t.Errorf("id = %v, want 2", feature["id"])
	}
	accept := feature["acceptance"].([]string)
	if len(accept) != 2 {
		t.Errorf("acceptance = %v", accept)
	}

	// Persistence: reload from disk and confirm the row stuck.
	fl, _ := harness.LoadFeatureList(root)
	if len(fl.Features) != 2 {
		t.Errorf("features on disk = %d", len(fl.Features))
	}
	if fl.Features[1].Status != harness.StatusPending {
		t.Errorf("added feature status = %s", fl.Features[1].Status)
	}
}

func TestToolHarnessAdd_RequiresName(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	_, err := srv.toolHarnessAdd(map[string]any{"root": root})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected name-required error, got %v", err)
	}
}

func TestResolveHarnessRoot_PrefersArg(t *testing.T) {
	got := resolveHarnessRoot(map[string]any{"root": "/from-arg"})
	if got != "/from-arg" {
		t.Errorf("resolveHarnessRoot = %q, want /from-arg", got)
	}
}

func TestResolveHarnessRoot_FallsBackToEnv(t *testing.T) {
	t.Setenv("KORVA_HARNESS_ROOT", "/from-env")
	got := resolveHarnessRoot(map[string]any{})
	if got != "/from-env" {
		t.Errorf("resolveHarnessRoot = %q, want /from-env", got)
	}
}

func TestProfileWiring_HarnessToolsAvailableUnderAgent(t *testing.T) {
	// Read tools should be available in every profile; init/start/done in
	// agent + admin only.
	for _, tool := range []string{"vault_harness_status", "vault_harness_list", "vault_harness_next"} {
		if !isAllowed(ProfileReadonly, tool) {
			t.Errorf("%s should be allowed under readonly profile", tool)
		}
	}
	for _, tool := range []string{
		"vault_harness_init", "vault_harness_add",
		"vault_harness_start", "vault_harness_done", "vault_harness_block", "vault_harness_reopen",
	} {
		if !isAllowed(ProfileAgent, tool) {
			t.Errorf("%s should be allowed under agent profile", tool)
		}
		if isAllowed(ProfileReadonly, tool) {
			t.Errorf("%s should NOT be allowed under readonly profile", tool)
		}
	}
}

func TestDispatch_HarnessToolsRegistered(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)

	// Round-trip through dispatch so we exercise the case branch.
	res, err := srv.dispatch("vault_harness_status", map[string]any{"root": root})
	if err != nil {
		t.Fatalf("dispatch status: %v", err)
	}
	if _, ok := res.(map[string]any)["project"]; !ok {
		t.Errorf("dispatch did not return harness payload: %+v", res)
	}
}

// ───────────────────────── Phase 13.2 — SDD MCP tools ─────────────────────────

// initSDDHarnessMCP wires an SDD harness for the MCP tests.
func initSDDHarnessMCP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := harness.Generate(harness.InitOptions{
		Root: dir, Project: "sdd-mcp", Stack: harness.StackGeneric, SDD: true,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return dir
}

func TestToolHarnessInit_SDDFlag(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	res, err := srv.toolHarnessInit(map[string]any{
		"root":    dir,
		"project": "via-mcp-sdd",
		"sdd":     true,
		"editors": "none",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if res.(map[string]any)["sdd"] != true {
		t.Errorf("response sdd = %v", res.(map[string]any)["sdd"])
	}
	fl, err := harness.LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !fl.Rules.RequireApprovedSpecToImplement {
		t.Error("SDD rule not applied")
	}
	if !fl.Features[0].SDD {
		t.Error("seed feature not sdd:true")
	}
	if _, err := os.Stat(filepath.Join(dir, "specs", "SPEC-TEMPLATE", "requirements.md")); err != nil {
		t.Errorf("SPEC-TEMPLATE not materialized: %v", err)
	}
}

func TestToolHarnessAdd_SDDFlag(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)

	res, err := srv.toolHarnessAdd(map[string]any{
		"root": root,
		"name": "auth_layer",
		"sdd":  true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	feat := res.(map[string]any)["feature"].(map[string]any)
	if feat["sdd"] != true {
		t.Errorf("added feature missing sdd:true wire field, got %+v", feat)
	}
}

func TestToolHarnessSpec_CreatesFiles(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)

	res, err := srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	resp := res.(map[string]any)
	if !resp["complete"].(bool) {
		t.Errorf("complete should be true after first call: %+v", resp)
	}
	written := resp["written"].([]string)
	if len(written) != 3 {
		t.Errorf("written = %v, want 3", written)
	}
	for _, f := range harness.SpecFiles {
		if _, err := os.Stat(filepath.Join(root, "specs", "harness_smoke", f)); err != nil {
			t.Errorf("spec file %s not created: %v", f, err)
		}
	}
}

func TestToolHarnessSpec_RejectsNonSDDFeature(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t) // standard harness, sdd=false

	_, err := srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})
	if err == nil || !strings.Contains(err.Error(), "not SDD") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestToolHarnessSpec_IdempotentByDefault(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})

	// Overwrite operator content.
	reqPath := filepath.Join(root, "specs", "harness_smoke", "requirements.md")
	if err := os.WriteFile(reqPath, []byte("OPERATOR"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})
	if err != nil {
		t.Fatalf("second spec: %v", err)
	}
	skipped := res.(map[string]any)["skipped"].([]string)
	if len(skipped) != 3 {
		t.Errorf("expected all 3 files skipped, got %v", skipped)
	}
	body, _ := os.ReadFile(reqPath)
	if string(body) != "OPERATOR" {
		t.Errorf("operator content was overwritten: %s", body)
	}
}

func TestToolHarnessReady_RejectsWithoutSpecFiles(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)

	_, err := srv.toolHarnessReady(map[string]any{"root": root, "id": float64(1)})
	if err == nil || !strings.Contains(err.Error(), "spec files missing") {
		t.Errorf("expected spec-missing error, got %v", err)
	}
}

func TestToolHarnessReady_HappyPath(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})

	res, err := srv.toolHarnessReady(map[string]any{
		"root": root, "id": float64(1), "agent": "spec_author",
	})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	resp := res.(map[string]any)
	if resp["status"] != string(harness.StatusSpecReady) {
		t.Errorf("status = %v", resp["status"])
	}
	if resp["owner"] != "spec_author" {
		t.Errorf("owner = %v", resp["owner"])
	}
}

func TestToolHarnessReady_RejectsNonSDDFeature(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	_, err := srv.toolHarnessReady(map[string]any{"root": root, "id": float64(1)})
	if err == nil || !strings.Contains(err.Error(), "not SDD") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestProfileWiring_SDDToolsAvailableUnderAgent(t *testing.T) {
	for _, tool := range []string{"vault_harness_spec", "vault_harness_ready"} {
		if !isAllowed(ProfileAgent, tool) {
			t.Errorf("%s should be allowed under agent profile", tool)
		}
		if isAllowed(ProfileReadonly, tool) {
			t.Errorf("%s should NOT be allowed under readonly profile", tool)
		}
	}
}

func TestDispatch_SDDToolsRegistered(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	if _, err := srv.dispatch("vault_harness_spec",
		map[string]any{"root": root, "id": float64(1)}); err != nil {
		t.Errorf("dispatch spec: %v", err)
	}
	if _, err := srv.dispatch("vault_harness_ready",
		map[string]any{"root": root, "id": float64(1)}); err != nil {
		t.Errorf("dispatch ready: %v", err)
	}
}

func TestFeatureToMap_IncludesSDDFlag(t *testing.T) {
	got := featureToMap(harness.Feature{ID: 1, Name: "x", SDD: true})
	if got["sdd"] != true {
		t.Errorf("featureToMap missing sdd:true, got %+v", got)
	}
	plain := featureToMap(harness.Feature{ID: 2, Name: "y"})
	if _, ok := plain["sdd"]; ok {
		t.Errorf("featureToMap should omit sdd when false, got %+v", plain)
	}
}

// ───────────────────────── Phase 13.3 — Check + bridge ─────────────────────────

func TestToolHarnessCheck_OnFreshHarness(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	res, err := srv.toolHarnessCheck(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	report := res.(*harness.CheckReport)
	if !report.OK || len(report.Issues) != 0 {
		t.Errorf("fresh harness should be OK, got %+v", report)
	}
}

func TestToolHarnessCheck_FlagsSDDViolation(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	// Promote pending → spec_ready by hand without spec files.
	fl, _ := harness.LoadFeatureList(root)
	fl.Features[0].Status = harness.StatusSpecReady
	_ = harness.SaveFeatureList(root, fl)

	res, err := srv.toolHarnessCheck(map[string]any{"root": root})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	report := res.(*harness.CheckReport)
	if report.OK {
		t.Errorf("expected non-OK report")
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != "sdd_spec_missing" {
		t.Errorf("expected single sdd_spec_missing issue, got %+v", report.Issues)
	}
}

func TestHarnessToSDDPhase_Mapping(t *testing.T) {
	cases := map[harness.FeatureStatus]struct {
		want store.SDDPhase
		ok   bool
	}{
		harness.StatusSpecReady:  {store.SDDSpec, true},
		harness.StatusInProgress: {store.SDDApply, true},
		harness.StatusDone:       {store.SDDVerify, true},
		harness.StatusPending:    {"", false},
		harness.StatusBlocked:    {"", false},
	}
	for status, want := range cases {
		got, ok := harnessToSDDPhase(status)
		if ok != want.ok {
			t.Errorf("ok(%s) = %v, want %v", status, ok, want.ok)
		}
		if got != want.want {
			t.Errorf("phase(%s) = %s, want %s", status, got, want.want)
		}
	}
}

func TestBridgeSDDPhase_SkipsWhenProjectEmpty(t *testing.T) {
	srv := newHarnessTestServer(t)
	if got := srv.bridgeSDDPhase(map[string]any{}, harness.StatusInProgress); got {
		t.Error("missing project arg should skip the bridge")
	}
}

func TestBridgeSDDPhase_SkipsForUnmappableStatus(t *testing.T) {
	srv := newHarnessTestServer(t)
	for _, status := range []harness.FeatureStatus{harness.StatusPending, harness.StatusBlocked} {
		if got := srv.bridgeSDDPhase(map[string]any{"project": "p"}, status); got {
			t.Errorf("status %s should not bridge", status)
		}
	}
}

func TestBridgeSDDPhase_PushesPhaseToStore(t *testing.T) {
	srv := newHarnessTestServer(t)
	if !srv.bridgeSDDPhase(map[string]any{"project": "team-x"}, harness.StatusSpecReady) {
		t.Fatal("bridge should succeed when project + mappable status are present")
	}
	state, err := srv.store.GetSDDPhase("team-x")
	if err != nil {
		t.Fatalf("GetSDDPhase: %v", err)
	}
	if state.Phase != store.SDDSpec {
		t.Errorf("phase pushed = %s, want %s", state.Phase, store.SDDSpec)
	}
}

func TestToolHarnessReady_BridgesWhenProjectGiven(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})

	res, err := srv.toolHarnessReady(map[string]any{
		"root": root, "id": float64(1), "project": "team-x",
	})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	resp := res.(map[string]any)
	if resp["sdd_phase_synced"] != true {
		t.Errorf("sdd_phase_synced = %v, want true", resp["sdd_phase_synced"])
	}
	state, _ := srv.store.GetSDDPhase("team-x")
	if state.Phase != store.SDDSpec {
		t.Errorf("vault SDD phase = %s, want spec", state.Phase)
	}
}

func TestToolHarnessTransition_BridgesOnStart(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t) // standard harness — non-SDD start is fine

	start := srv.toolHarnessTransition(harness.StatusInProgress)
	res, err := start(map[string]any{
		"root": root, "id": float64(1), "project": "team-y",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.(map[string]any)["sdd_phase_synced"] != true {
		t.Error("start with project should bridge")
	}
	state, _ := srv.store.GetSDDPhase("team-y")
	if state.Phase != store.SDDApply {
		t.Errorf("vault SDD phase = %s, want apply", state.Phase)
	}
}

func TestToolHarnessTransition_NoBridgeWithoutProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	start := srv.toolHarnessTransition(harness.StatusInProgress)
	res, err := start(map[string]any{"root": root, "id": float64(1)})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.(map[string]any)["sdd_phase_synced"] != false {
		t.Errorf("sdd_phase_synced should be false without project arg, got %v", res.(map[string]any)["sdd_phase_synced"])
	}
}

func TestProfileWiring_CheckToolEverywhere(t *testing.T) {
	// vault_harness_check is read-only — available under every profile.
	for _, profile := range []Profile{ProfileReadonly, ProfileAgent, ProfileAdmin} {
		if !isAllowed(profile, "vault_harness_check") {
			t.Errorf("vault_harness_check should be allowed under %s profile", profile)
		}
	}
}

func TestDispatch_CheckToolRegistered(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	if _, err := srv.dispatch("vault_harness_check",
		map[string]any{"root": root}); err != nil {
		t.Errorf("dispatch check: %v", err)
	}
}

// ───────────────────────── Phase 14.1 + 14.2 — vault-side mirror ─────────────────────────

// withTestSession installs a fake authenticated session on the test
// server so MCP write tools record snapshots/transitions under a known
// team_id. Without this every helper writes with team_id="" (anonymous
// MCP) and the team-scoped store reads return nothing.
func withTestSession(srv *Server, teamID string) {
	srv.session = &mcpSession{teamID: teamID, email: "test@x", role: "admin"}
}

// countRowsInTable is a direct-DB helper for assertions where we want
// to verify "no rows landed at all" (the team-scoped store reads
// suppress orphan rows by design).
func countRowsInTable(t *testing.T, srv *Server, table string) int {
	t.Helper()
	var n int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestPersistHarnessSnapshot_SkipsWithoutProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	if got := srv.persistHarnessSnapshot(map[string]any{}, root); got {
		t.Error("missing project arg should skip")
	}
	if n := countRowsInTable(t, srv, "harness_snapshots"); n != 0 {
		t.Errorf("no snapshot should be written, got %d", n)
	}
}

func TestPersistHarnessSnapshot_WritesToStore(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initHarness(t)
	if !srv.persistHarnessSnapshot(map[string]any{"project": "p"}, root) {
		t.Fatal("expected successful persist")
	}
	snap, err := srv.store.GetHarnessSnapshot("team-x", "p", root)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if !strings.Contains(snap.Payload, "harness_smoke") {
		t.Errorf("payload missing seed feature: %s", snap.Payload)
	}
	if snap.TeamID != "team-x" {
		t.Errorf("team_id = %q, want team-x", snap.TeamID)
	}
}

func TestPersistHarnessSnapshot_AnonymousSessionPersistsWithEmptyTeam(t *testing.T) {
	// MCP without auth still persists, but the row is orphaned (invisible
	// to team-scoped reads). The harness CLI works offline through this
	// path.
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	if !srv.persistHarnessSnapshot(map[string]any{"project": "p"}, root) {
		t.Fatal("anonymous persist should succeed")
	}
	// Direct DB read: row exists with team_id=''.
	var team string
	if err := srv.store.DB().QueryRow(
		`SELECT team_id FROM harness_snapshots WHERE project='p' AND root=?`, root,
	).Scan(&team); err != nil {
		t.Fatalf("direct read: %v", err)
	}
	if team != "" {
		t.Errorf("anonymous persist should land team_id='', got %q", team)
	}
	// Team-scoped read on the empty-team query returns nothing — orphan
	// rows are invisible to the public API.
	if snaps, _ := srv.store.ListHarnessSnapshotsForTeam(""); len(snaps) != 0 {
		t.Errorf("anonymous list should return nothing, got %v", snaps)
	}
}

func TestRecordHarnessTransition_SkipsWithoutProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	if got := srv.recordHarnessTransition(map[string]any{}, "/r", 1,
		harness.StatusPending, harness.StatusInProgress, "alice"); got {
		t.Error("missing project should skip")
	}
}

func TestRecordHarnessTransition_LogsWhenProjectGiven(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	if !srv.recordHarnessTransition(map[string]any{"project": "p"}, "/r", 1,
		harness.StatusPending, harness.StatusInProgress, "alice") {
		t.Fatal("expected log to succeed")
	}
	rows, _ := srv.store.ListHarnessTransitionsForTeam("team-x", "p", 10)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].FromStatus != "pending" || rows[0].ToStatus != "in_progress" || rows[0].Owner != "alice" {
		t.Errorf("row mismatch: %+v", rows[0])
	}
	if rows[0].TeamID != "team-x" {
		t.Errorf("team_id = %q", rows[0].TeamID)
	}
}

func TestToolHarnessTransition_PersistsOnStartWithProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initHarness(t)
	start := srv.toolHarnessTransition(harness.StatusInProgress)
	res, err := start(map[string]any{
		"root": root, "id": float64(1), "project": "p",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	resp := res.(map[string]any)
	if resp["snapshot_synced"] != true {
		t.Errorf("snapshot_synced = %v, want true", resp["snapshot_synced"])
	}
	if _, err := srv.store.GetHarnessSnapshot("team-x", "p", root); err != nil {
		t.Errorf("snapshot not persisted: %v", err)
	}
	rows, _ := srv.store.ListHarnessTransitionsForTeam("team-x", "p", 10)
	if len(rows) != 1 {
		t.Errorf("transitions logged = %d, want 1", len(rows))
	}
	if rows[0].FromStatus != "pending" || rows[0].ToStatus != "in_progress" {
		t.Errorf("transition row wrong: %+v", rows[0])
	}
}

func TestToolHarnessTransition_NoMirrorWithoutProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initHarness(t)
	start := srv.toolHarnessTransition(harness.StatusInProgress)
	res, err := start(map[string]any{"root": root, "id": float64(1)})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.(map[string]any)["snapshot_synced"] != false {
		t.Errorf("snapshot_synced should be false without project")
	}
	if n := countRowsInTable(t, srv, "harness_snapshots"); n != 0 {
		t.Errorf("no snapshot should land without project, got %d", n)
	}
	if n := countRowsInTable(t, srv, "harness_transitions"); n != 0 {
		t.Errorf("no transition should log without project, got %d", n)
	}
}

func TestToolHarnessReady_PersistsWithProject(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})

	res, err := srv.toolHarnessReady(map[string]any{
		"root": root, "id": float64(1), "project": "p",
	})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if res.(map[string]any)["snapshot_synced"] != true {
		t.Errorf("snapshot_synced = %v, want true", res.(map[string]any)["snapshot_synced"])
	}
	rows, _ := srv.store.ListHarnessTransitionsForTeam("team-x", "p", 10)
	if len(rows) != 1 || rows[0].ToStatus != "spec_ready" {
		t.Errorf("ready transition not logged: %+v", rows)
	}
}

func TestToolHarnessTransition_DoneAfterStart_LogsBothTransitions(t *testing.T) {
	// Round-trip: start → done with project arg should produce two rows
	// in the transition log, with correct from/to chain.
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initHarness(t)

	start := srv.toolHarnessTransition(harness.StatusInProgress)
	if _, err := start(map[string]any{"root": root, "id": float64(1), "project": "p"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := srv.toolHarnessTransition(harness.StatusDone)
	if _, err := done(map[string]any{"root": root, "id": float64(1), "project": "p"}); err != nil {
		t.Fatalf("done: %v", err)
	}

	rows, _ := srv.store.ListHarnessTransitionsForTeam("team-x", "p", 10)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].ToStatus != "done" || rows[0].FromStatus != "in_progress" {
		t.Errorf("done row wrong: %+v", rows[0])
	}
	if rows[1].ToStatus != "in_progress" || rows[1].FromStatus != "pending" {
		t.Errorf("start row wrong: %+v", rows[1])
	}
}

func TestToolHarnessTransition_SnapshotIsLatestPayload(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	root := initHarness(t)
	start := srv.toolHarnessTransition(harness.StatusInProgress)
	_, _ = start(map[string]any{"root": root, "id": float64(1), "project": "p"})
	done := srv.toolHarnessTransition(harness.StatusDone)
	_, _ = done(map[string]any{"root": root, "id": float64(1), "project": "p"})

	snap, err := srv.store.GetHarnessSnapshot("team-x", "p", root)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(snap.Payload, `"status": "done"`) {
		t.Errorf("snapshot should reflect latest status, got: %s", snap.Payload)
	}
}

// Phase 14.2 specific: cross-team isolation through the MCP write path.
func TestToolHarnessTransition_CrossTeamCannotSeeOtherSnapshot(t *testing.T) {
	// Team A writes; team B reads — must look like the snapshot doesn't
	// exist (sql.ErrNoRows surfaces as 404 in the REST layer).
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-A")
	root := initHarness(t)
	start := srv.toolHarnessTransition(harness.StatusInProgress)
	if _, err := start(map[string]any{"root": root, "id": float64(1), "project": "p"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Team-A sees its own snapshot.
	if _, err := srv.store.GetHarnessSnapshot("team-A", "p", root); err != nil {
		t.Errorf("team-A should see its own snapshot: %v", err)
	}
	// Team-B sees nothing.
	if _, err := srv.store.GetHarnessSnapshot("team-B", "p", root); err == nil {
		t.Error("team-B should not see team-A's snapshot")
	}
}

func TestCallerTeamID_ReturnsEmptyWhenNoSession(t *testing.T) {
	srv := newHarnessTestServer(t)
	if got := srv.callerTeamID(); got != "" {
		t.Errorf("callerTeamID with no session = %q, want empty", got)
	}
}

func TestCallerTeamID_ReturnsSessionTeam(t *testing.T) {
	srv := newHarnessTestServer(t)
	withTestSession(srv, "team-x")
	if got := srv.callerTeamID(); got != "team-x" {
		t.Errorf("callerTeamID = %q, want team-x", got)
	}
}

// ───────────────────────── Phase 15.B — `vault_harness_spec_review` ─────────────────────────

func TestToolHarnessSpecReview_FreshScaffoldFails(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})

	res, err := srv.toolHarnessSpecReview(map[string]any{"root": root, "id": float64(1)})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	report := res.(*harness.SpecReviewReport)
	if report.OK {
		t.Errorf("fresh scaffold should fail the lint, got %+v", report.Issues)
	}
}

func TestToolHarnessSpecReview_RejectsNonSDDFeature(t *testing.T) {
	srv := newHarnessTestServer(t)
	root := initHarness(t)
	_, err := srv.toolHarnessSpecReview(map[string]any{"root": root, "id": float64(1)})
	if err == nil || !strings.Contains(err.Error(), "not SDD-flagged") {
		t.Errorf("expected non-SDD rejection, got %v", err)
	}
}

func TestToolHarnessSpecReview_RegisteredInProfilesAndDispatch(t *testing.T) {
	srv := newHarnessTestServer(t)
	// Read-only → every profile including readonly.
	for _, p := range []Profile{ProfileReadonly, ProfileAgent, ProfileAdmin} {
		if !isAllowed(p, "vault_harness_spec_review") {
			t.Errorf("spec_review should be in %s profile", p)
		}
	}
	// Dispatch wiring.
	root := initSDDHarnessMCP(t)
	_, _ = srv.toolHarnessSpec(map[string]any{"root": root, "id": float64(1)})
	if _, err := srv.dispatch("vault_harness_spec_review",
		map[string]any{"root": root, "id": float64(1)}); err != nil {
		t.Errorf("dispatch: %v", err)
	}
}

// ───────────────────────── Phase 15.A — `vault_harness_ci_install` ─────────────────────────

func TestToolHarnessCIInstall_GitHubActions(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	res, err := srv.toolHarnessCIInstall(map[string]any{
		"root": dir, "provider": "github-actions",
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	result := res.(*harness.InstallCIResult)
	if result.Provider != harness.CIGitHubActions {
		t.Errorf("provider = %q", result.Provider)
	}
	if len(result.Written) == 0 {
		t.Errorf("expected files written, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "harness.yml")); err != nil {
		t.Errorf("workflow file missing: %v", err)
	}
}

func TestToolHarnessCIInstall_GitLab(t *testing.T) {
	srv := newHarnessTestServer(t)
	dir := t.TempDir()
	if _, err := srv.toolHarnessCIInstall(map[string]any{
		"root": dir, "provider": "gitlab-ci",
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.harness.yml")); err != nil {
		t.Errorf("gitlab yml missing: %v", err)
	}
}

func TestToolHarnessCIInstall_RequiresProvider(t *testing.T) {
	srv := newHarnessTestServer(t)
	_, err := srv.toolHarnessCIInstall(map[string]any{"root": t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "provider is required") {
		t.Errorf("expected provider-required error, got %v", err)
	}
}

func TestToolHarnessCIInstall_RejectsUnknownProvider(t *testing.T) {
	srv := newHarnessTestServer(t)
	_, err := srv.toolHarnessCIInstall(map[string]any{
		"root": t.TempDir(), "provider": "circleci",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown-provider error, got %v", err)
	}
}

func TestToolHarnessCIInstall_RegisteredInProfilesAndDispatch(t *testing.T) {
	srv := newHarnessTestServer(t)
	// Profile gating — only agent + admin (write op).
	if !isAllowed(ProfileAgent, "vault_harness_ci_install") {
		t.Error("ci_install should be in agent profile")
	}
	if isAllowed(ProfileReadonly, "vault_harness_ci_install") {
		t.Error("ci_install should NOT be in readonly profile")
	}
	// Dispatch wiring.
	dir := t.TempDir()
	if _, err := srv.dispatch("vault_harness_ci_install", map[string]any{
		"root": dir, "provider": "github-actions",
	}); err != nil {
		t.Errorf("dispatch: %v", err)
	}
}
