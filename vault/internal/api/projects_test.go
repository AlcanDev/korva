package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminListProjects(t *testing.T) {
	s := newAPITestStore(t)
	for _, p := range []string{"alpha", "beta", "gamma"} {
		_, _ = s.Save(store.Observation{Project: p, Type: store.TypeDecision, Title: p, Content: "x"})
	}
	h := adminListProjects(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Projects []store.ProjectStats `json:"projects"`
		Count    int                  `json:"count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Count != 3 {
		t.Errorf("count = %d, want 3", resp.Count)
	}
}

func TestAdminSuggestConsolidations(t *testing.T) {
	s := newAPITestStore(t)
	for _, p := range []struct{ project, title string }{
		{"alpha", "a"},
		{"Alpha", "b"},
		{"isolated", "c"},
	} {
		_, _ = s.Save(store.Observation{Project: p.project, Type: store.TypeDecision, Title: p.title, Content: "x"})
	}
	h := adminSuggestConsolidations(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/projects/suggestions", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Proposals []store.ConsolidationProposal `json:"proposals"`
		Count     int                           `json:"count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Count != 1 {
		t.Errorf("expected 1 proposal, got %d", resp.Count)
	}
	if len(resp.Proposals) == 1 && resp.Proposals[0].Canonical == "" {
		t.Error("proposal canonical must be set")
	}
}

func TestAdminConsolidateProjects(t *testing.T) {
	s := newAPITestStore(t)
	_, _ = s.Save(store.Observation{Project: "alpha", Type: store.TypeDecision, Title: "a", Content: "x"})
	_, _ = s.Save(store.Observation{Project: "Alpha", Type: store.TypeDecision, Title: "b", Content: "y"})

	h := adminConsolidateProjects(s)
	body, _ := json.Marshal(consolidateRequest{Canonical: "alpha", Sources: []string{"Alpha"}})
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/consolidate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "merged" {
		t.Errorf("status = %v, want merged", resp["status"])
	}

	// Verify "Alpha" is gone.
	projects, _ := s.ListProjects()
	for _, p := range projects {
		if p.Name == "Alpha" {
			t.Error("Alpha should have been merged into alpha")
		}
	}
}

func TestAdminConsolidateProjects_ValidationErrors(t *testing.T) {
	s := newAPITestStore(t)
	h := adminConsolidateProjects(s)
	tests := []struct {
		name string
		body string
	}{
		{"missing canonical", `{"sources":["a"]}`},
		{"missing sources", `{"canonical":"a","sources":[]}`},
		{"invalid json", `not-json`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/projects/consolidate", bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestAdminPruneProjects_DryRunByDefault(t *testing.T) {
	s := newAPITestStore(t)
	_, _ = s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "a", Content: "x"})
	_, _ = s.SessionStart("abandoned", "", "", "agent", "test")

	h := adminPruneProjects(s)
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/prune", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp store.PruneResult
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.DryRun {
		t.Error("expected DryRun=true with empty body")
	}
	if len(resp.Empty) != 1 || resp.Empty[0].Project != "abandoned" {
		t.Errorf("expected abandoned empty, got %+v", resp.Empty)
	}
}

func TestAdminPruneProjects_ApplyDeletes(t *testing.T) {
	s := newAPITestStore(t)
	_, _ = s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "a", Content: "x"})
	_, _ = s.SessionStart("abandoned", "", "", "agent", "test")

	h := adminPruneProjects(s)
	body, _ := json.Marshal(pruneRequest{Apply: true})
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/prune", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp store.PruneResult
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.DryRun {
		t.Error("expected DryRun=false on apply")
	}
	if resp.SessionsRemoved != 1 {
		t.Errorf("expected 1 session removed, got %d", resp.SessionsRemoved)
	}
}
