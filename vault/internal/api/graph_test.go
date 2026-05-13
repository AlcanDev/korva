package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminGraph_RequiresProject(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/graph", nil)
	rec := httptest.NewRecorder()
	adminGraph(s)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (project required)", rec.Code)
	}
}

func TestAdminGraph_ReturnsNodesAndEdges(t *testing.T) {
	s := newAPITestStore(t)
	// Seed 3 observations with one relation between two of them.
	id1, _ := s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "Use ULID", Content: "x"})
	id2, _ := s.Save(store.Observation{Project: "korva", Type: store.TypePattern, Title: "Outbox", Content: "y"})
	_, _ = s.Save(store.Observation{Project: "korva", Type: store.TypeBugfix, Title: "race fix", Content: "z"})

	if _, err := s.AddRelation(id1, id2, store.RelationRelated, "topical link", "test"); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/graph?project=korva", nil)
	rec := httptest.NewRecorder()
	adminGraph(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var resp GraphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Project != "korva" {
		t.Errorf("Project = %q, want korva", resp.Project)
	}
	if len(resp.Nodes) != 3 {
		t.Errorf("Nodes = %d, want 3", len(resp.Nodes))
	}
	if len(resp.Edges) != 1 {
		t.Errorf("Edges = %d, want 1", len(resp.Edges))
	}
	if resp.Truncated {
		t.Error("Truncated should be false with 3 obs under default limit")
	}
}

func TestAdminGraph_FiltersDanglingEdges(t *testing.T) {
	// When a relation points to an obs outside the limited node set, the
	// edge must be dropped to avoid rendering "ghost" lines.
	s := newAPITestStore(t)
	id1, _ := s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "a", Content: "x"})
	id2, _ := s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "b", Content: "y"})
	_, _ = s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "c", Content: "z"})
	if _, err := s.AddRelation(id1, id2, store.RelationSupersedes, "", "test"); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	// Cap to 2 nodes — only the 2 most-recent will fit; the third drops.
	req := httptest.NewRequest(http.MethodGet, "/admin/graph?project=korva&limit=2", nil)
	rec := httptest.NewRecorder()
	adminGraph(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp GraphResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Nodes) != 2 {
		t.Errorf("Nodes = %d, want 2", len(resp.Nodes))
	}
	if !resp.Truncated {
		t.Error("Truncated should be true when limit < total")
	}
	// The relation (id1, id2) survives only if both endpoints are in the
	// returned set. The test asserts no dangling edges either way.
	for _, e := range resp.Edges {
		hasSource := false
		hasTarget := false
		for _, n := range resp.Nodes {
			if n.ID == e.Source {
				hasSource = true
			}
			if n.ID == e.Target {
				hasTarget = true
			}
		}
		if !hasSource || !hasTarget {
			t.Errorf("dangling edge: %v (source=%v target=%v)", e, hasSource, hasTarget)
		}
	}
}

func TestAdminGraph_LimitParamClamped(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/graph?project=korva&limit=10000", nil)
	rec := httptest.NewRecorder()
	adminGraph(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// Just smoke the response — with no data, no nodes; the test verifies
	// the handler didn't blow up on the absurd limit.
	var resp GraphResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Nodes) != 0 {
		t.Errorf("expected 0 nodes on empty store, got %d", len(resp.Nodes))
	}
}
