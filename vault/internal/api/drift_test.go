package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestJaccard_KnownOverlaps(t *testing.T) {
	a := map[string]struct{}{"ulid": {}, "primary": {}, "keys": {}}
	b := map[string]struct{}{"ulid": {}, "primary": {}, "uuid": {}}
	// A∩B = {ulid, primary} = 2
	// A∪B = {ulid, primary, keys, uuid} = 4
	got := jaccard(a, b)
	if got < 0.49 || got > 0.51 {
		t.Errorf("jaccard = %v, want ≈ 0.5", got)
	}
}

func TestSeverityForOverlap_Thresholds(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.6, "danger"},
		{0.5, "danger"},
		{0.4, "warning"},
		{0.35, "warning"},
		{0.3, "info"},
		{0.1, "info"},
	}
	for _, tc := range tests {
		if got := severityForOverlap(tc.score); got != tc.want {
			t.Errorf("severityForOverlap(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestComputeDriftAlerts_FlagsLaterViolator(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -10)
	recent := time.Now().UTC().AddDate(0, 0, -1)
	obs := []store.Observation{
		{
			ID:        "decision-1",
			Project:   "korva",
			Type:      store.TypeDecision,
			Title:     "Use ULID for primary keys",
			Content:   "ULID gives sortable, URL-safe identifiers — never UUID.",
			CreatedAt: old,
		},
		{
			ID:        "bugfix-1",
			Project:   "korva",
			Type:      store.TypeBugfix,
			Title:     "Fix UUID primary keys",
			Content:   "Race condition in our UUID generator — primary keys collide.",
			CreatedAt: recent,
		},
	}
	alerts := computeDriftAlerts(obs, 30)
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 drift alert")
	}
	if alerts[0].DecisionID != "decision-1" {
		t.Errorf("decision_id = %q", alerts[0].DecisionID)
	}
	if alerts[0].ViolatorID != "bugfix-1" {
		t.Errorf("violator_id = %q", alerts[0].ViolatorID)
	}
	if alerts[0].OverlapScore < minOverlapScore {
		t.Errorf("overlap score = %v < %v", alerts[0].OverlapScore, minOverlapScore)
	}
}

func TestComputeDriftAlerts_IgnoresOlderViolator(t *testing.T) {
	// A violator that lands BEFORE the decision can't be drift.
	dec := time.Now().UTC().AddDate(0, 0, -5)
	violatorOlder := time.Now().UTC().AddDate(0, 0, -10)
	obs := []store.Observation{
		{ID: "d", Project: "korva", Type: store.TypeDecision, Title: "Use ULID", Content: "x", CreatedAt: dec},
		{ID: "v", Project: "korva", Type: store.TypeBugfix, Title: "Use ULID", Content: "x", CreatedAt: violatorOlder},
	}
	alerts := computeDriftAlerts(obs, 30)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts when violator is older, got %d", len(alerts))
	}
}

func TestAdminDecisionDrift_NoDataReturnsEmpty(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/drift/decisions?project=korva", nil)
	rec := httptest.NewRecorder()
	adminDecisionDrift(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp DriftResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Alerts) != 0 {
		t.Errorf("expected 0 alerts on empty store, got %d", len(resp.Alerts))
	}
}
