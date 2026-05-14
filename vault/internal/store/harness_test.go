package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

const testTeam = "team-1"

func TestSaveHarnessSnapshot_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	payload := `{"project":"x","features":[]}`
	if err := s.SaveHarnessSnapshot(testTeam, "x", "/tmp/x", payload); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetHarnessSnapshot(testTeam, "x", "/tmp/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Payload != payload {
		t.Errorf("payload roundtrip = %q, want %q", got.Payload, payload)
	}
	if got.TeamID != testTeam {
		t.Errorf("team = %q, want %q", got.TeamID, testTeam)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not parsed")
	}
}

func TestSaveHarnessSnapshot_UpsertReplacesPayload(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot(testTeam, "x", "/r", `{"v":1}`)
	if err := s.SaveHarnessSnapshot(testTeam, "x", "/r", `{"v":2}`); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, _ := s.GetHarnessSnapshot(testTeam, "x", "/r")
	if got.Payload != `{"v":2}` {
		t.Errorf("upsert did not replace payload, got %q", got.Payload)
	}
}

func TestSaveHarnessSnapshot_RequiresProjectAndRoot(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveHarnessSnapshot(testTeam, "", "/r", "x"); err == nil {
		t.Error("expected error for empty project")
	}
	if err := s.SaveHarnessSnapshot(testTeam, "p", "", "x"); err == nil {
		t.Error("expected error for empty root")
	}
}

func TestGetHarnessSnapshot_MissingReturnsErrNoRows(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetHarnessSnapshot(testTeam, "never", "/r")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing snapshot should return sql.ErrNoRows, got %v", err)
	}
}

// Phase 14.2 — multi-tenant isolation: cross-team reads must look
// indistinguishable from "doesn't exist" so an attacker can't enumerate
// other teams' projects.
func TestGetHarnessSnapshot_CrossTeamLooksLikeMissing(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("team-A", "secret-proj", "/r", "x")
	_, err := s.GetHarnessSnapshot("team-B", "secret-proj", "/r")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("cross-team read should return sql.ErrNoRows, got %v", err)
	}
}

func TestListHarnessSnapshotsForTeam_OrdersByUpdatedDesc(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot(testTeam, "a", "/r1", "1")
	if _, err := s.db.Exec(
		`INSERT INTO harness_snapshots(team_id, project, root, payload, updated_at)
		 VALUES(?, 'b','/r2','2',?)`,
		testTeam, time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListHarnessSnapshotsForTeam(testTeam)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Project != "b" {
		t.Errorf("most-recent should be first, got %q", rows[0].Project)
	}
}

func TestListHarnessSnapshotsForTeam_OnlyOwnTeam(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("team-A", "p1", "/r", "x")
	_ = s.SaveHarnessSnapshot("team-B", "p2", "/r", "x")
	got, err := s.ListHarnessSnapshotsForTeam("team-A")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Project != "p1" {
		t.Errorf("got %v, want only team-A's row", got)
	}
}

func TestListHarnessSnapshotsForTeam_EmptyTeamReturnsNothing(t *testing.T) {
	// Anonymous queries see nothing — by design.
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("team-A", "p", "/r", "x")
	got, _ := s.ListHarnessSnapshotsForTeam("")
	if len(got) != 0 {
		t.Errorf("empty team should return nothing, got %v", got)
	}
}

func TestRecordHarnessTransition_PersistsAllFields(t *testing.T) {
	s := newTestStore(t)
	tr := HarnessTransition{
		TeamID:     testTeam,
		Project:    "x",
		Root:       "/r",
		FeatureID:  7,
		FromStatus: "pending",
		ToStatus:   "in_progress",
		Owner:      "alice",
	}
	if err := s.RecordHarnessTransition(tr); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, err := s.ListHarnessTransitionsForTeam(testTeam, "x", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("rows = %d", len(got))
	}
	row := got[0]
	if row.FeatureID != 7 || row.FromStatus != "pending" ||
		row.ToStatus != "in_progress" || row.Owner != "alice" {
		t.Errorf("row mismatch: %+v", row)
	}
	if row.TeamID != testTeam {
		t.Errorf("team_id = %q, want %q", row.TeamID, testTeam)
	}
	if row.ID == "" {
		t.Error("ID not auto-generated")
	}
	if row.OccurredAt.IsZero() {
		t.Error("OccurredAt not parsed")
	}
}

func TestRecordHarnessTransition_RejectsBadInput(t *testing.T) {
	s := newTestStore(t)
	cases := []struct {
		name string
		tr   HarnessTransition
	}{
		{"missing project", HarnessTransition{TeamID: testTeam, Root: "/r", FeatureID: 1, ToStatus: "done"}},
		{"missing root", HarnessTransition{TeamID: testTeam, Project: "p", FeatureID: 1, ToStatus: "done"}},
		{"zero feature_id", HarnessTransition{TeamID: testTeam, Project: "p", Root: "/r", ToStatus: "done"}},
		{"missing to_status", HarnessTransition{TeamID: testTeam, Project: "p", Root: "/r", FeatureID: 1}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := s.RecordHarnessTransition(tc.tr); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestListHarnessTransitionsForTeam_FilterByProject(t *testing.T) {
	s := newTestStore(t)
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: testTeam, Project: "a", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: testTeam, Project: "b", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})

	all, _ := s.ListHarnessTransitionsForTeam(testTeam, "", 10)
	if len(all) != 2 {
		t.Errorf("team-wide list got %d, want 2", len(all))
	}
	onlyA, _ := s.ListHarnessTransitionsForTeam(testTeam, "a", 10)
	if len(onlyA) != 1 || onlyA[0].Project != "a" {
		t.Errorf("filtered list = %+v", onlyA)
	}
}

func TestListHarnessTransitionsForTeam_OnlyOwnTeam(t *testing.T) {
	s := newTestStore(t)
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: "team-A", Project: "p", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: "team-B", Project: "p", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})
	got, _ := s.ListHarnessTransitionsForTeam("team-A", "", 10)
	if len(got) != 1 || got[0].TeamID != "team-A" {
		t.Errorf("cross-team isolation broken: %+v", got)
	}
}

func TestListHarnessTransitionsForTeam_EmptyTeamReturnsNothing(t *testing.T) {
	s := newTestStore(t)
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: "team-A", Project: "p", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})
	got, _ := s.ListHarnessTransitionsForTeam("", "", 10)
	if len(got) != 0 {
		t.Errorf("empty team should return nothing, got %v", got)
	}
}

func TestListHarnessTransitionsForTeam_LimitClampsAndDefaults(t *testing.T) {
	s := newTestStore(t)
	for i := 1; i <= 5; i++ {
		_ = s.RecordHarnessTransition(HarnessTransition{
			TeamID: testTeam, Project: "p", Root: "/r", FeatureID: i, ToStatus: "in_progress",
		})
	}
	got, _ := s.ListHarnessTransitionsForTeam(testTeam, "p", 3)
	if len(got) != 3 {
		t.Errorf("limit not respected: %d", len(got))
	}
	got, _ = s.ListHarnessTransitionsForTeam(testTeam, "p", 0)
	if len(got) != 5 {
		t.Errorf("default limit should not truncate 5 rows, got %d", len(got))
	}
	// limit > 1000 must clamp; we don't insert 1001 rows but we can
	// verify the SQL doesn't fail when asked for a huge limit.
	got, err := s.ListHarnessTransitionsForTeam(testTeam, "p", 99999)
	if err != nil {
		t.Errorf("limit clamp should not error: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("clamp result = %d, want 5", len(got))
	}
}

func TestListHarnessTransitionsForTeam_NewestFirst(t *testing.T) {
	s := newTestStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	recent := time.Now().UTC()
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: testTeam, Project: "p", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
		OccurredAt: old,
	})
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: testTeam, Project: "p", Root: "/r", FeatureID: 2, ToStatus: "in_progress",
		OccurredAt: recent,
	})
	got, _ := s.ListHarnessTransitionsForTeam(testTeam, "p", 10)
	if got[0].FeatureID != 2 {
		t.Errorf("expected newest first, got %+v", got)
	}
}

func TestListHarnessProjectSummariesForTeam_JoinsLastTransition(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot(testTeam, "p", "/r", `{}`)
	_ = s.RecordHarnessTransition(HarnessTransition{
		TeamID: testTeam, Project: "p", Root: "/r", FeatureID: 1, ToStatus: "spec_ready",
	})
	rows, err := s.ListHarnessProjectSummariesForTeam(testTeam)
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].LastTransitionTo != "spec_ready" {
		t.Errorf("last transition not joined: %+v", rows[0])
	}
	if rows[0].TeamID != testTeam {
		t.Errorf("team = %q", rows[0].TeamID)
	}
}

func TestListHarnessProjectSummariesForTeam_OnlyOwnTeam(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("team-A", "pa", "/r", "x")
	_ = s.SaveHarnessSnapshot("team-B", "pb", "/r", "x")
	rows, _ := s.ListHarnessProjectSummariesForTeam("team-A")
	if len(rows) != 1 || rows[0].Project != "pa" {
		t.Errorf("cross-team isolation broken: %+v", rows)
	}
}

func TestListHarnessProjectSummariesForTeam_ProjectsWithoutTransitionsStillAppear(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot(testTeam, "p", "/r", `{}`)
	rows, _ := s.ListHarnessProjectSummariesForTeam(testTeam)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].LastTransitionTo != "" {
		t.Errorf("expected empty last transition, got %q", rows[0].LastTransitionTo)
	}
}

func TestListHarnessProjectSummariesForTeam_EmptyTeamReturnsNothing(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("team-A", "p", "/r", "x")
	rows, _ := s.ListHarnessProjectSummariesForTeam("")
	if len(rows) != 0 {
		t.Errorf("empty team should return nothing, got %v", rows)
	}
}

func TestNewTransitionID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newTransitionID()
		if seen[id] {
			t.Errorf("duplicate id at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}
