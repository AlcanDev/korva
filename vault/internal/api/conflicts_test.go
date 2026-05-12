package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// seedPending stages a project with two observations and one pending judgment.
// Returns (sourceID, targetID, judgmentID).
func seedPending(t *testing.T, s *store.Store, project string) (string, string, string) {
	t.Helper()
	src, err := s.Save(store.Observation{
		Project: project, Type: store.TypeDecision,
		Title: "use ULID for keys", Content: "ULIDs sort lexicographically",
	})
	if err != nil {
		t.Fatalf("save source: %v", err)
	}
	tgt, err := s.Save(store.Observation{
		Project: project, Type: store.TypePattern,
		Title: "ULID encoding", Content: "Crockford base32 keeps URLs safe",
	})
	if err != nil {
		t.Fatalf("save target: %v", err)
	}
	tgtObs, _ := s.Get(tgt)
	created, err := s.CreatePendingJudgments(src, []store.Observation{*tgtObs})
	if err != nil || len(created) == 0 {
		t.Fatalf("create pending: err=%v created=%v", err, created)
	}
	return src, tgt, created[0]
}

func TestAdminListConflicts_DefaultsToPending(t *testing.T) {
	s := newAPITestStore(t)
	seedPending(t, s, "korva")

	h := adminListConflicts(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/conflicts", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Conflicts []store.Relation `json:"conflicts"`
		Status    string           `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != string(store.JudgmentPending) {
		t.Errorf("status = %q, want pending", resp.Status)
	}
	if len(resp.Conflicts) != 1 {
		t.Errorf("expected 1 pending, got %d", len(resp.Conflicts))
	}
}

func TestAdminListConflicts_RejectsUnknownStatus(t *testing.T) {
	s := newAPITestStore(t)
	h := adminListConflicts(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/conflicts?status=magic", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminListConflicts_ScopedToProject(t *testing.T) {
	s := newAPITestStore(t)
	seedPending(t, s, "alpha")
	seedPending(t, s, "beta")

	h := adminListConflicts(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/conflicts?project=alpha", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct {
		Conflicts []store.Relation `json:"conflicts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Conflicts) != 1 || resp.Conflicts[0].Project != "alpha" {
		t.Errorf("project filter leaked: %+v", resp.Conflicts)
	}
}

func TestAdminGetConflict_EmbedsSourceAndTarget(t *testing.T) {
	s := newAPITestStore(t)
	src, tgt, jid := seedPending(t, s, "korva")

	h := adminGetConflict(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/conflicts/"+jid, nil)
	req.SetPathValue("id", jid)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Conflict store.Relation     `json:"conflict"`
		Source   *store.Observation `json:"source"`
		Target   *store.Observation `json:"target"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Source == nil || resp.Source.ID != src {
		t.Errorf("expected source %s embedded, got %+v", src, resp.Source)
	}
	if resp.Target == nil || resp.Target.ID != tgt {
		t.Errorf("expected target %s embedded, got %+v", tgt, resp.Target)
	}
}

func TestAdminGetConflict_NotFound(t *testing.T) {
	s := newAPITestStore(t)
	h := adminGetConflict(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/conflicts/nope", nil)
	req.SetPathValue("id", "01DOESNOTEXIST")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminJudgeConflict_HappyPath(t *testing.T) {
	s := newAPITestStore(t)
	_, _, jid := seedPending(t, s, "korva")

	h := adminJudgeConflict(s)
	body := judgeRequest{
		Relation: string(store.RelationSupersedes), Reason: "newer insight",
		Confidence: 0.9, MarkedByActor: string(store.ActorAgent), MarkedByKind: string(store.VerdictHeuristic),
	}
	bb, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/conflicts/"+jid+"/judge", bytes.NewReader(bb))
	req.SetPathValue("id", jid)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got store.Relation
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.JudgmentStatus != store.JudgmentJudged {
		t.Errorf("status = %q, want judged", got.JudgmentStatus)
	}
}

func TestAdminJudgeConflict_RejectsBadInput(t *testing.T) {
	s := newAPITestStore(t)
	_, _, jid := seedPending(t, s, "korva")

	h := adminJudgeConflict(s)
	body := []byte(`{"relation":"yelling"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/conflicts/"+jid+"/judge", bytes.NewReader(body))
	req.SetPathValue("id", jid)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminIgnoreConflict_ClosesPending(t *testing.T) {
	s := newAPITestStore(t)
	_, _, jid := seedPending(t, s, "korva")

	h := adminIgnoreConflict(s)
	body := []byte(`{"reason":"noise"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/conflicts/"+jid+"/ignore", bytes.NewReader(body))
	req.SetPathValue("id", jid)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, _ := s.GetJudgment(jid)
	if got.JudgmentStatus != store.JudgmentIgnored {
		t.Errorf("status = %q, want ignored", got.JudgmentStatus)
	}
}

func TestAdminCompareConflict_UpsertsRow(t *testing.T) {
	s := newAPITestStore(t)
	src, err := s.Save(store.Observation{Project: "korva", Type: store.TypeDecision, Title: "a", Content: "a"})
	if err != nil {
		t.Fatal(err)
	}
	tgt, err := s.Save(store.Observation{Project: "korva", Type: store.TypePattern, Title: "b", Content: "b"})
	if err != nil {
		t.Fatal(err)
	}

	h := adminCompareConflict(s)
	body, _ := json.Marshal(compareRequest{
		SourceID: src, TargetID: tgt,
		Relation: string(store.RelationRelated), Confidence: 0.7,
		MarkedByActor: string(store.ActorAgent), MarkedByModel: "claude-opus-4-7",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/conflicts/compare", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got store.Relation
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.MarkedByKind != store.VerdictLLM {
		t.Errorf("kind = %q, want llm", got.MarkedByKind)
	}
}
