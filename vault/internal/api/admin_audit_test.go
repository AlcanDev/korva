package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAdminListAudit_Empty(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminListAudit(s), http.MethodGet, "/admin/audit", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	logs, ok := resp["logs"]
	if !ok {
		t.Fatal("missing 'logs' key")
	}
	if len(logs.([]interface{})) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs.([]interface{})))
	}
}

func TestAdminListAudit_WithEntries(t *testing.T) {
	s := newTestStore(t)
	writeAudit(s, "admin", "create_team", "team-001", "", hashStr("Alpha"))
	writeAudit(s, "admin", "add_member", "mem-001", "", hashStr("alice@example.com"))

	rec := adminDo(t, adminListAudit(s), http.MethodGet, "/admin/audit", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	logs := resp["logs"].([]interface{})
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestAdminListAudit_LimitDefault(t *testing.T) {
	s := newTestStore(t)
	// Insert 60 entries — default limit should cap at 50
	for i := 0; i < 60; i++ {
		writeAudit(s, "admin", "action", "target", "", "")
	}

	rec := adminDo(t, adminListAudit(s), http.MethodGet, "/admin/audit", nil)
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	logs := resp["logs"].([]interface{})
	if len(logs) > 50 {
		t.Errorf("expected at most 50 logs, got %d", len(logs))
	}
}

func TestAdminListAudit_ResponseHasCount(t *testing.T) {
	s := newTestStore(t)
	writeAudit(s, "admin", "create", "x", "", "")
	writeAudit(s, "admin", "delete", "y", "h1", "")

	rec := adminDo(t, adminListAudit(s), http.MethodGet, "/admin/audit", nil)
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	count, ok := resp["count"]
	if !ok {
		t.Fatal("missing 'count' key")
	}
	if int(count.(float64)) != 2 {
		t.Errorf("count = %v, want 2", count)
	}
}

func TestWriteAudit_Persists(t *testing.T) {
	s := newTestStore(t)
	writeAudit(s, "testactor", "test_action", "tgt-001", "before", "after")

	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE actor=? AND action=?`, "testactor", "test_action").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 audit entry, got %d", count)
	}
}

func TestHashStr(t *testing.T) {
	h1 := hashStr("hello")
	h2 := hashStr("hello")
	h3 := hashStr("world")

	if h1 != h2 {
		t.Error("hashStr not deterministic")
	}
	if h1 == h3 {
		t.Error("hashStr collision for different inputs")
	}
	if len(h1) == 0 {
		t.Error("hashStr returned empty string")
	}
}

func TestHashStr_Empty(t *testing.T) {
	h := hashStr("")
	if h == "" {
		t.Error("hashStr('') should still return a hash")
	}
}
