package store

import (
	"strings"
	"testing"
	"time"
)

// Phase 20.A — direct coverage for the calls.go store helpers.
// LogCall + ListCalls + GetCallStats are the backbone of every MCP
// observability feature (admin/interactions, /admin/cost/*, the
// new editor adoption widget). Indirect coverage existed via the
// MCP dispatch path; this file pins the contract end-to-end.

func newCallsStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewMemory(nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLogCall_AssignsIDAndPersists(t *testing.T) {
	s := newCallsStore(t)
	in := CallLog{
		Tool:      "vault_save",
		Project:   "korva",
		Author:    "alice@acme.io",
		Editor:    "claude",
		Status:    "ok",
		LatencyMs: 42,
	}
	if err := s.LogCall(in); err != nil {
		t.Fatalf("LogCall: %v", err)
	}

	got, err := s.ListCalls(CallFilters{Limit: 10})
	if err != nil {
		t.Fatalf("ListCalls: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	row := got[0]
	if row.ID == "" {
		t.Error("ID should be auto-assigned")
	}
	if row.Tool != "vault_save" || row.Project != "korva" || row.Editor != "claude" {
		t.Errorf("row = %+v", row)
	}
	if row.LatencyMs != 42 {
		t.Errorf("latency = %d", row.LatencyMs)
	}
	if row.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated by the trigger")
	}
}

func TestLogCall_PreservesErrorRows(t *testing.T) {
	s := newCallsStore(t)
	if err := s.LogCall(CallLog{
		Tool: "vault_search", Status: "error", ErrorMsg: "FTS syntax error",
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListCalls(CallFilters{Status: "error"})
	if len(got) != 1 {
		t.Fatalf("rows = %d", len(got))
	}
	if got[0].ErrorMsg != "FTS syntax error" {
		t.Errorf("error_msg = %q", got[0].ErrorMsg)
	}
}

func TestListCalls_FilterByTool(t *testing.T) {
	s := newCallsStore(t)
	_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})
	_ = s.LogCall(CallLog{Tool: "vault_search", Status: "ok"})
	_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})

	got, _ := s.ListCalls(CallFilters{Tool: "vault_save"})
	if len(got) != 2 {
		t.Errorf("rows = %d, want 2 vault_save", len(got))
	}
	for _, r := range got {
		if r.Tool != "vault_save" {
			t.Errorf("filter leaked %q", r.Tool)
		}
	}
}

func TestListCalls_FilterByProject(t *testing.T) {
	s := newCallsStore(t)
	_ = s.LogCall(CallLog{Tool: "vault_save", Project: "a", Status: "ok"})
	_ = s.LogCall(CallLog{Tool: "vault_save", Project: "b", Status: "ok"})
	got, _ := s.ListCalls(CallFilters{Project: "a"})
	if len(got) != 1 || got[0].Project != "a" {
		t.Errorf("filter wrong: %+v", got)
	}
}

func TestListCalls_FilterByAuthor(t *testing.T) {
	s := newCallsStore(t)
	_ = s.LogCall(CallLog{Tool: "vault_save", Author: "alice", Status: "ok"})
	_ = s.LogCall(CallLog{Tool: "vault_save", Author: "bob", Status: "ok"})
	got, _ := s.ListCalls(CallFilters{Author: "alice"})
	if len(got) != 1 || got[0].Author != "alice" {
		t.Errorf("filter wrong: %+v", got)
	}
}

func TestListCalls_FilterByStatus(t *testing.T) {
	s := newCallsStore(t)
	_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})
	_ = s.LogCall(CallLog{Tool: "vault_save", Status: "error"})
	_ = s.LogCall(CallLog{Tool: "vault_save", Status: "error"})
	got, _ := s.ListCalls(CallFilters{Status: "error"})
	if len(got) != 2 {
		t.Errorf("rows = %d, want 2", len(got))
	}
}

func TestListCalls_OrdersNewestFirst(t *testing.T) {
	s := newCallsStore(t)
	for i := 0; i < 5; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_save", Project: "p", Status: "ok"})
		// SQLite's `datetime('now')` truncates to seconds so two rows
		// inserted in the same second can tie. A small sleep ensures a
		// stable ORDER BY for the assertion.
		time.Sleep(1100 * time.Millisecond)
	}
	got, _ := s.ListCalls(CallFilters{Limit: 5})
	if len(got) != 5 {
		t.Fatalf("rows = %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].CreatedAt.Before(got[i].CreatedAt) {
			t.Errorf("rows not ordered DESC: %v vs %v", got[i-1].CreatedAt, got[i].CreatedAt)
		}
	}
}

func TestListCalls_LimitAndOffset(t *testing.T) {
	s := newCallsStore(t)
	for i := 0; i < 10; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})
	}
	page1, _ := s.ListCalls(CallFilters{Limit: 3})
	if len(page1) != 3 {
		t.Errorf("page1 = %d, want 3", len(page1))
	}
	page2, _ := s.ListCalls(CallFilters{Limit: 3, Offset: 3})
	if len(page2) != 3 {
		t.Errorf("page2 = %d, want 3", len(page2))
	}
	// The two pages must not overlap.
	for _, a := range page1 {
		for _, b := range page2 {
			if a.ID == b.ID {
				t.Errorf("offset paging returned the same row twice: %s", a.ID)
			}
		}
	}
}

func TestListCalls_DefaultsLimitTo100(t *testing.T) {
	s := newCallsStore(t)
	// 150 rows; the default limit caps to 100.
	for i := 0; i < 150; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})
	}
	got, _ := s.ListCalls(CallFilters{})
	if len(got) != 100 {
		t.Errorf("rows = %d, want 100 (default cap)", len(got))
	}
}

func TestGetCallStats_AggregatesCounts(t *testing.T) {
	s := newCallsStore(t)
	for i := 0; i < 7; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok", LatencyMs: 10})
	}
	for i := 0; i < 3; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_search", Status: "error", LatencyMs: 100})
	}
	stats, err := s.GetCallStats()
	if err != nil {
		t.Fatalf("GetCallStats: %v", err)
	}
	if stats.Total != 10 {
		t.Errorf("Total = %d, want 10", stats.Total)
	}
	if stats.ErrorCount != 3 {
		t.Errorf("ErrorCount = %d, want 3", stats.ErrorCount)
	}
	// AvgLatency = (7*10 + 3*100) / 10 = 37.
	if stats.AvgLatency < 36 || stats.AvgLatency > 38 {
		t.Errorf("AvgLatency = %f, want ~37", stats.AvgLatency)
	}
	if stats.ByTool["vault_save"] != 7 || stats.ByTool["vault_search"] != 3 {
		t.Errorf("ByTool = %v", stats.ByTool)
	}
	if stats.ByStatus["ok"] != 7 || stats.ByStatus["error"] != 3 {
		t.Errorf("ByStatus = %v", stats.ByStatus)
	}
}

func TestGetCallStats_EmptyStoreReturnsZeros(t *testing.T) {
	s := newCallsStore(t)
	stats, err := s.GetCallStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.AvgLatency != 0 {
		t.Errorf("AvgLatency = %f, want 0", stats.AvgLatency)
	}
}

func TestLogCall_AssignsUniqueIDs(t *testing.T) {
	s := newCallsStore(t)
	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		_ = s.LogCall(CallLog{Tool: "vault_save", Status: "ok"})
	}
	rows, _ := s.ListCalls(CallFilters{Limit: 50})
	for _, r := range rows {
		if seen[r.ID] {
			t.Fatalf("ID collision: %s", r.ID)
		}
		seen[r.ID] = true
		if !strings.Contains(r.ID, "") || len(r.ID) < 16 {
			t.Errorf("ID looks malformed: %q", r.ID)
		}
	}
}
