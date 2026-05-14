package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestSaveHarnessSnapshot_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	payload := `{"project":"x","features":[]}`
	if err := s.SaveHarnessSnapshot("x", "/tmp/x", payload); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetHarnessSnapshot("x", "/tmp/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Payload != payload {
		t.Errorf("payload roundtrip = %q, want %q", got.Payload, payload)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not parsed")
	}
}

func TestSaveHarnessSnapshot_UpsertReplacesPayload(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("x", "/r", `{"v":1}`)
	if err := s.SaveHarnessSnapshot("x", "/r", `{"v":2}`); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, _ := s.GetHarnessSnapshot("x", "/r")
	if got.Payload != `{"v":2}` {
		t.Errorf("upsert did not replace payload, got %q", got.Payload)
	}
}

func TestSaveHarnessSnapshot_RequiresProjectAndRoot(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveHarnessSnapshot("", "/r", "x"); err == nil {
		t.Error("expected error for empty project")
	}
	if err := s.SaveHarnessSnapshot("p", "", "x"); err == nil {
		t.Error("expected error for empty root")
	}
}

func TestGetHarnessSnapshot_MissingReturnsErrNoRows(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetHarnessSnapshot("never", "/r")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing snapshot should return sql.ErrNoRows, got %v", err)
	}
}

func TestListHarnessSnapshots_OrdersByUpdatedDesc(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("a", "/r1", "1")
	// Force the second snapshot to land later by stamping a known time.
	if _, err := s.db.Exec(
		`INSERT INTO harness_snapshots(project, root, payload, updated_at)
		 VALUES('b','/r2','2',?)`,
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListHarnessSnapshots()
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

func TestRecordHarnessTransition_PersistsAllFields(t *testing.T) {
	s := newTestStore(t)
	tr := HarnessTransition{
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
	got, err := s.ListHarnessTransitions("x", 10)
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
		{"missing project", HarnessTransition{Root: "/r", FeatureID: 1, ToStatus: "done"}},
		{"missing root", HarnessTransition{Project: "p", FeatureID: 1, ToStatus: "done"}},
		{"zero feature_id", HarnessTransition{Project: "p", Root: "/r", ToStatus: "done"}},
		{"missing to_status", HarnessTransition{Project: "p", Root: "/r", FeatureID: 1}},
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

func TestListHarnessTransitions_FilterByProject(t *testing.T) {
	s := newTestStore(t)
	_ = s.RecordHarnessTransition(HarnessTransition{
		Project: "a", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})
	_ = s.RecordHarnessTransition(HarnessTransition{
		Project: "b", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
	})

	all, _ := s.ListHarnessTransitions("", 10)
	if len(all) != 2 {
		t.Errorf("global list got %d, want 2", len(all))
	}
	onlyA, _ := s.ListHarnessTransitions("a", 10)
	if len(onlyA) != 1 || onlyA[0].Project != "a" {
		t.Errorf("filtered list = %+v", onlyA)
	}
}

func TestListHarnessTransitions_LimitClamps(t *testing.T) {
	s := newTestStore(t)
	for i := 1; i <= 5; i++ {
		_ = s.RecordHarnessTransition(HarnessTransition{
			Project: "p", Root: "/r", FeatureID: i, ToStatus: "in_progress",
		})
	}
	got, _ := s.ListHarnessTransitions("p", 3)
	if len(got) != 3 {
		t.Errorf("limit not respected: %d", len(got))
	}
	// limit ≤ 0 falls back to default 100; 5 rows fit under that.
	got, _ = s.ListHarnessTransitions("p", 0)
	if len(got) != 5 {
		t.Errorf("default limit should not truncate 5 rows, got %d", len(got))
	}
}

func TestListHarnessTransitions_NewestFirst(t *testing.T) {
	s := newTestStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	recent := time.Now().UTC()
	_ = s.RecordHarnessTransition(HarnessTransition{
		Project: "p", Root: "/r", FeatureID: 1, ToStatus: "in_progress",
		OccurredAt: old,
	})
	_ = s.RecordHarnessTransition(HarnessTransition{
		Project: "p", Root: "/r", FeatureID: 2, ToStatus: "in_progress",
		OccurredAt: recent,
	})
	got, _ := s.ListHarnessTransitions("p", 10)
	if got[0].FeatureID != 2 {
		t.Errorf("expected newest first, got %+v", got)
	}
}

func TestListHarnessProjectSummaries_JoinsLastTransition(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("p", "/r", `{}`)
	_ = s.RecordHarnessTransition(HarnessTransition{
		Project: "p", Root: "/r", FeatureID: 1, ToStatus: "spec_ready",
	})
	rows, err := s.ListHarnessProjectSummaries()
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].LastTransitionTo != "spec_ready" {
		t.Errorf("last transition not joined: %+v", rows[0])
	}
}

func TestListHarnessProjectSummaries_ProjectsWithoutTransitionsStillAppear(t *testing.T) {
	// A snapshot with zero transitions (e.g. operator just ran init)
	// must still show up in the dashboard.
	s := newTestStore(t)
	_ = s.SaveHarnessSnapshot("p", "/r", `{}`)
	rows, _ := s.ListHarnessProjectSummaries()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].LastTransitionTo != "" {
		t.Errorf("expected empty last transition, got %q", rows[0].LastTransitionTo)
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
