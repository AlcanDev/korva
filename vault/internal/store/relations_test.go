package store

import (
	"strings"
	"testing"
)

// Phase 20.A — direct coverage for relations.go: the cross-
// observation linking layer (supersedes / conflicts_with /
// related / compatible / scoped). The conflict-judgment workflow
// builds on top of these; missing tests here would let
// invariants (cross-project rejection, upsert idempotency) drift
// silently.

func saveObsForRelation(t *testing.T, s *Store, project, title string) string {
	t.Helper()
	id, err := s.Save(Observation{
		Project: project, Title: title, Content: "x",
		Type: TypeLearning, Author: "test",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return id
}

func TestAddRelation_HappyPath(t *testing.T) {
	s := newCallsStore(t)
	src := saveObsForRelation(t, s, "p", "older")
	tgt := saveObsForRelation(t, s, "p", "newer")

	rel, err := s.AddRelation(src, tgt, RelationSupersedes, "newer doc", "alice")
	if err != nil {
		t.Fatalf("AddRelation: %v", err)
	}
	if rel.ID == "" {
		t.Error("ID auto-assigned expected")
	}
	if rel.SourceID != src || rel.TargetID != tgt || rel.Relation != RelationSupersedes {
		t.Errorf("rel = %+v", rel)
	}
	if rel.Project != "p" {
		t.Errorf("project = %q, want p", rel.Project)
	}
	if rel.Status != "confirmed" {
		t.Errorf("status = %q, want confirmed (legacy default)", rel.Status)
	}
	if rel.CreatedAt.IsZero() {
		t.Error("CreatedAt not parsed")
	}
}

func TestAddRelation_RejectsInvalidType(t *testing.T) {
	s := newCallsStore(t)
	src := saveObsForRelation(t, s, "p", "a")
	tgt := saveObsForRelation(t, s, "p", "b")
	_, err := s.AddRelation(src, tgt, RelationType("unknown"), "", "")
	if err == nil {
		t.Fatal("expected invalid-type error")
	}
	if !strings.Contains(err.Error(), "invalid relation type") {
		t.Errorf("error message = %v", err)
	}
}

func TestAddRelation_RejectsCrossProject(t *testing.T) {
	s := newCallsStore(t)
	src := saveObsForRelation(t, s, "alpha", "a")
	tgt := saveObsForRelation(t, s, "beta", "b")
	_, err := s.AddRelation(src, tgt, RelationRelated, "", "")
	if err == nil {
		t.Fatal("expected cross-project error")
	}
	if !strings.Contains(err.Error(), "cross-project") {
		t.Errorf("error message = %v", err)
	}
}

func TestAddRelation_RejectsMissingObservations(t *testing.T) {
	s := newCallsStore(t)
	if _, err := s.AddRelation("missing-src", "missing-tgt", RelationRelated, "", ""); err == nil {
		t.Error("expected missing-source error")
	}
	src := saveObsForRelation(t, s, "p", "a")
	if _, err := s.AddRelation(src, "missing-tgt", RelationRelated, "", ""); err == nil {
		t.Error("expected missing-target error")
	}
}

func TestAddRelation_UpsertOnConflict(t *testing.T) {
	s := newCallsStore(t)
	src := saveObsForRelation(t, s, "p", "a")
	tgt := saveObsForRelation(t, s, "p", "b")

	// First write.
	r1, err := s.AddRelation(src, tgt, RelationRelated, "first reason", "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Second write with same (source, target) + different type/reason
	// should UPSERT — only one row should exist after.
	r2, err := s.AddRelation(src, tgt, RelationConflicts, "second reason", "bob")
	if err != nil {
		t.Fatal(err)
	}

	rels, err := s.GetRelations(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels.AsSource) != 1 {
		t.Fatalf("upsert produced %d rows, want 1", len(rels.AsSource))
	}
	got := rels.AsSource[0]
	if got.Relation != RelationConflicts {
		t.Errorf("relation type after upsert = %q, want %q", got.Relation, RelationConflicts)
	}
	if got.Reason != "second reason" || got.Author != "bob" {
		t.Errorf("upsert did not refresh fields: %+v", got)
	}
	// Both calls return rows (existence pin).
	if r1 == nil || r2 == nil {
		t.Error("AddRelation returned nil after upsert")
	}
}

func TestGetRelations_SplitsAsSourceVsAsTarget(t *testing.T) {
	s := newCallsStore(t)
	hub := saveObsForRelation(t, s, "p", "hub")
	other := saveObsForRelation(t, s, "p", "other")

	// hub → other (hub is source).
	if _, err := s.AddRelation(hub, other, RelationSupersedes, "", ""); err != nil {
		t.Fatal(err)
	}
	// other → hub (hub is target).
	if _, err := s.AddRelation(other, hub, RelationRelated, "", ""); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRelations(hub)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AsSource) != 1 || got.AsSource[0].TargetID != other {
		t.Errorf("AsSource wrong: %+v", got.AsSource)
	}
	if len(got.AsTarget) != 1 || got.AsTarget[0].SourceID != other {
		t.Errorf("AsTarget wrong: %+v", got.AsTarget)
	}
}

func TestGetRelations_EmptyForUnrelatedID(t *testing.T) {
	s := newCallsStore(t)
	got, err := s.GetRelations("does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AsSource) != 0 || len(got.AsTarget) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestListRelationsByProject_FilterAndAll(t *testing.T) {
	s := newCallsStore(t)
	a := saveObsForRelation(t, s, "alpha", "a")
	b := saveObsForRelation(t, s, "alpha", "b")
	c := saveObsForRelation(t, s, "alpha", "c")

	_, _ = s.AddRelation(a, b, RelationSupersedes, "", "")
	_, _ = s.AddRelation(a, c, RelationRelated, "", "")
	_, _ = s.AddRelation(b, c, RelationConflicts, "", "")

	all, err := s.ListRelationsByProject("alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}

	supersedes, err := s.ListRelationsByProject("alpha", RelationSupersedes)
	if err != nil {
		t.Fatal(err)
	}
	if len(supersedes) != 1 {
		t.Errorf("supersedes = %d, want 1", len(supersedes))
	}
	if supersedes[0].SourceID != a || supersedes[0].TargetID != b {
		t.Errorf("supersedes row wrong: %+v", supersedes[0])
	}
}

func TestListRelationsByProject_OtherProjectIsolated(t *testing.T) {
	s := newCallsStore(t)
	a := saveObsForRelation(t, s, "alpha", "a")
	b := saveObsForRelation(t, s, "alpha", "b")
	_, _ = s.AddRelation(a, b, RelationRelated, "", "")

	got, err := s.ListRelationsByProject("beta", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("beta should see no rows, got %d", len(got))
	}
}

func TestDeleteRelation_RemovesAndReportsHit(t *testing.T) {
	s := newCallsStore(t)
	src := saveObsForRelation(t, s, "p", "a")
	tgt := saveObsForRelation(t, s, "p", "b")
	r, err := s.AddRelation(src, tgt, RelationRelated, "", "")
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := s.DeleteRelation(r.ID)
	if err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}
	if !deleted {
		t.Error("expected deleted=true on existing row")
	}
	got, _ := s.GetRelations(src)
	if len(got.AsSource) != 0 {
		t.Errorf("row still present after delete: %+v", got)
	}
}

func TestDeleteRelation_ReturnsFalseOnMiss(t *testing.T) {
	s := newCallsStore(t)
	deleted, err := s.DeleteRelation("does-not-exist")
	if err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}
	if deleted {
		t.Error("expected deleted=false on missing id")
	}
}
