package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminGetIntegrity_HappyPath(t *testing.T) {
	s := newAPITestStore(t)
	// Seed one row so the schema-drift check has something to walk.
	if _, err := s.Save(store.Observation{
		Project: "p", Type: store.TypeDecision,
		Title: "t", Content: "c",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := adminGetIntegrity(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/integrity", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Healthy bool                   `json:"healthy"`
		Checks  []store.IntegrityCheck `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Healthy {
		for _, c := range resp.Checks {
			t.Logf("%s = %s — %s", c.Name, c.Status, c.Detail)
		}
		t.Error("freshly seeded store should report healthy")
	}
	if len(resp.Checks) == 0 {
		t.Error("report should include checks")
	}
}

func TestAdminRepairIntegrity_PlanMode_DoesNotWrite(t *testing.T) {
	s := newAPITestStore(t)
	for i := 0; i < 3; i++ {
		_, _ = s.Save(store.Observation{
			Project: "p", Type: store.TypeDecision,
			Title: "obs-" + string(rune('a'+i)), Content: "content",
		})
	}

	h := adminRepairIntegrity(s)
	body := repairIntegrityRequest{Mode: "plan"}
	bb, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/integrity/repair", bytes.NewReader(bb))
	req.ContentLength = int64(len(bb))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp store.RepairReport
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Mode != store.RepairModePlan {
		t.Errorf("Mode = %q, want plan", resp.Mode)
	}
	if len(resp.Actions) == 0 {
		t.Error("plan should list default operations")
	}
	for _, a := range resp.Actions {
		if a.AppliedRows != 0 {
			t.Errorf("plan must not apply rows; got %d for %s", a.AppliedRows, a.Operation)
		}
	}
}

func TestAdminRepairIntegrity_RejectsUnknownMode(t *testing.T) {
	s := newAPITestStore(t)
	h := adminRepairIntegrity(s)

	body := []byte(`{"mode":"magic"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/integrity/repair", bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminRepairIntegrity_DefaultsToPlanWhenBodyEmpty(t *testing.T) {
	s := newAPITestStore(t)
	h := adminRepairIntegrity(s)

	req := httptest.NewRequest(http.MethodPost, "/admin/integrity/repair", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp store.RepairReport
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Mode != store.RepairModePlan {
		t.Errorf("empty body should default to plan, got %q", resp.Mode)
	}
}
