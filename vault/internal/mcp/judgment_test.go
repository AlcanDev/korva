package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// dispatchOn runs a tool call directly against an in-memory Server, sharing
// the same store across calls. The standard sendAndReceive helper allocates
// a fresh store per request, which masks the auto-scan behaviour because it
// cannot reach across observations.
func dispatchOn(t *testing.T, srv *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	out, err := srv.dispatchInner(name, args)
	if err != nil {
		t.Fatalf("dispatch %s: %v", name, err)
	}
	asMap, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("%s should return map[string]any, got %T", name, out)
	}
	return asMap
}

// newSharedServer returns a Server with a real store the caller can re-use
// across many tool dispatches. ProfileAdmin so every tool is reachable.
func newSharedServer(t *testing.T) *Server {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &Server{
		store:   s,
		reader:  bufio.NewReader(strings.NewReader("")),
		writer:  &bytes.Buffer{},
		logger:  log.New(bytes.NewBuffer(nil), "", 0),
		profile: ProfileAdmin,
	}
}

// TestToolCall_VaultSave_AutoScanSurfacesPendingJudgments confirms that
// vault_save returns judgment_required + pending_judgments when a saved
// observation overlaps with existing knowledge in the same project.
func TestToolCall_VaultSave_AutoScanSurfacesPendingJudgments(t *testing.T) {
	srv := newSharedServer(t)

	dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "decision",
		"title": "adopt ULID for primary keys", "content": "ULIDs sort lexicographically",
	})

	resp := dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "pattern",
		"title": "ULID encoding format", "content": "Crockford base32 keeps observation IDs URL-safe",
	})

	if resp["judgment_required"] != true {
		t.Fatalf("expected judgment_required:true, got %v", resp["judgment_required"])
	}
	raw, _ := json.Marshal(resp["pending_judgments"])
	if !strings.Contains(string(raw), `"judgment_id"`) {
		t.Errorf("pending_judgments shape is missing judgment_id: %s", raw)
	}
}

// TestToolCall_VaultJudge_HappyPath verifies the end-to-end flow: save two
// related observations, the auto-scan surfaces a judgment, and vault_judge
// flips it to status=judged.
func TestToolCall_VaultJudge_HappyPath(t *testing.T) {
	srv := newSharedServer(t)

	dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "decision",
		"title": "shared topic alpha", "content": "first observation",
	})
	dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "pattern",
		"title": "shared topic beta", "content": "second observation",
	})

	pending, err := srv.store.ListPendingJudgments("smoke", 10)
	if err != nil {
		t.Fatalf("ListPendingJudgments: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("expected at least one pending judgment after the auto-scan")
	}
	judgmentID := pending[0].ID

	resp := dispatchOn(t, srv, "vault_judge", map[string]any{
		"judgment_id": judgmentID,
		"relation":    "related",
		"reason":      "shared topic",
		"confidence":  0.9,
	})
	if resp["status"] != "judged" {
		t.Errorf("expected status=judged, got %v", resp["status"])
	}
	if resp["judgment_id"] != judgmentID {
		t.Errorf("response judgment_id mismatch")
	}

	updated, _ := srv.store.GetJudgment(judgmentID)
	if string(updated.JudgmentStatus) != "judged" || string(updated.Relation) != "related" {
		t.Errorf("store state after judge: status=%q relation=%q", updated.JudgmentStatus, updated.Relation)
	}
}

// TestToolCall_VaultCompare_UpsertsLLMVerdict skips the pending step and
// stores an LLM-evaluated verdict directly.
func TestToolCall_VaultCompare_UpsertsLLMVerdict(t *testing.T) {
	srv := newSharedServer(t)

	srcResp := dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "decision",
		"title": "distinct alpha", "content": "first", "skip_scan": true,
	})
	tgtResp := dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "pattern",
		"title": "distinct beta", "content": "second", "skip_scan": true,
	})

	resp := dispatchOn(t, srv, "vault_compare", map[string]any{
		"source_id":       srcResp["id"],
		"target_id":       tgtResp["id"],
		"relation":        "related",
		"confidence":      0.8,
		"marked_by_model": "claude-opus-4-7",
	})
	if resp["status"] != "stored" {
		t.Errorf("expected status=stored, got %v", resp["status"])
	}
	id, _ := resp["judgment_id"].(string)
	if id == "" {
		t.Fatal("expected judgment_id in response")
	}
	got, _ := srv.store.GetJudgment(id)
	if got == nil || string(got.MarkedByKind) != "llm" {
		t.Errorf("expected stored row marked_by_kind=llm, got %+v", got)
	}
}

// TestToolCall_VaultSave_SkipScan respects the new opt-out flag so callers
// can persist without auto-scanning (used by bulk imports).
func TestToolCall_VaultSave_SkipScan(t *testing.T) {
	srv := newSharedServer(t)

	dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "decision",
		"title": "ULID primary keys", "content": "x",
	})
	resp := dispatchOn(t, srv, "vault_save", map[string]any{
		"project": "smoke", "type": "pattern",
		"title": "ULID encoding", "content": "x", "skip_scan": true,
	})
	if _, present := resp["judgment_required"]; present {
		t.Errorf("skip_scan should suppress judgment_required, got %+v", resp)
	}
}
