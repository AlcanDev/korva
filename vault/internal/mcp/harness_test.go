package mcp

import (
	"bufio"
	"bytes"
	"log"
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
		"root":           dir,
		"project":        "via-mcp",
		"description":    "from MCP",
		"stack":          "go",
		"with_subagents": true,
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

	// Verify the seed feature_list.json is real.
	fl, err := harness.LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if fl.Project != "via-mcp" {
		t.Errorf("seed project = %q", fl.Project)
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
