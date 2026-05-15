package store

import (
	"testing"
)

// Phase 20.A — direct coverage for replay.go's three queries.
// They power /admin/sessions/{id}/replay; without these tests the
// replay endpoint relied entirely on integration coverage.

func TestGetSession_RoundTripsActiveSession(t *testing.T) {
	s := newCallsStore(t)
	id, err := s.SessionStart("p", "t", "CL", "claude", "implement X")
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	got, err := s.GetSession(id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("session not found")
	}
	if got.ID != id || got.Project != "p" || got.Agent != "claude" || got.Goal != "implement X" {
		t.Errorf("session = %+v", got)
	}
	if got.EndedAt != nil {
		t.Error("active session should have nil EndedAt")
	}
}

func TestGetSession_PopulatesEndedAtAfterEnd(t *testing.T) {
	s := newCallsStore(t)
	id, _ := s.SessionStart("p", "t", "CL", "claude", "g")
	if err := s.SessionEnd(id, "wrap up"); err != nil {
		t.Fatalf("SessionEnd: %v", err)
	}
	got, err := s.GetSession(id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("session missing after end")
	}
	if got.EndedAt == nil {
		t.Error("EndedAt should be populated after SessionEnd")
	}
	if got.Summary != "wrap up" {
		t.Errorf("summary = %q", got.Summary)
	}
}

func TestGetSession_ReturnsNilForUnknownID(t *testing.T) {
	s := newCallsStore(t)
	got, err := s.GetSession("does-not-exist")
	if err != nil {
		t.Errorf("GetSession on missing id should not error, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestListObservationsBySession_FiltersAndOrdersAscending(t *testing.T) {
	s := newCallsStore(t)
	id, _ := s.SessionStart("p", "", "", "claude", "g")

	// Three observations under this session + one under a different
	// session to prove the filter excludes outsiders. Each obs gets
	// distinct content so the dedup layer doesn't collapse them.
	for i := 0; i < 3; i++ {
		_, _ = s.Save(Observation{
			Project: "p", Title: "obs",
			Content: "unique-content-" + string(rune('a'+i)),
			Type:    TypeLearning, SessionID: id,
		})
	}
	otherID, _ := s.SessionStart("p", "", "", "claude", "other")
	_, _ = s.Save(Observation{
		Project: "p", Title: "outsider", Content: "x",
		Type: TypeLearning, SessionID: otherID,
	})

	got, err := s.ListObservationsBySession(id)
	if err != nil {
		t.Fatalf("ListObservationsBySession: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("rows = %d, want 3", len(got))
	}
	for _, o := range got {
		if o.SessionID != id {
			t.Errorf("filter leaked: obs.SessionID = %q", o.SessionID)
		}
		if o.Title != "obs" {
			t.Errorf("title = %q", o.Title)
		}
	}
	// Ordered ASC: each created_at >= the previous.
	for i := 1; i < len(got); i++ {
		if got[i].CreatedAt.Before(got[i-1].CreatedAt) {
			t.Errorf("rows not ASC: %v before %v", got[i].CreatedAt, got[i-1].CreatedAt)
		}
	}
}

func TestListObservationsBySession_EmptyForUnknownSession(t *testing.T) {
	s := newCallsStore(t)
	got, err := s.ListObservationsBySession("does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d rows", len(got))
	}
}

func TestListInteractionsBySession_ScansEditorAndCreatedAt(t *testing.T) {
	s := newCallsStore(t)
	id, _ := s.SessionStart("p", "", "", "claude", "g")

	if _, err := s.SaveInteraction(Interaction{
		SessionID: id, Project: "p", Agent: "claude", Editor: "cursor",
		Model: "m", PromptExcerpt: "p", ResponseExcerpt: "r",
		DurationMs: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInteraction(Interaction{
		SessionID: id, Project: "p", Agent: "claude",
		Model: "m", PromptExcerpt: "p2", DurationMs: 50,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListInteractionsBySession(id)
	if err != nil {
		t.Fatalf("ListInteractionsBySession: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("rows = %d, want 2", len(got))
	}
	// Each row carries the session id back, has CreatedAt populated,
	// and the new (Phase 19.A) editor field is round-tripped.
	for _, in := range got {
		if in.SessionID != id {
			t.Errorf("session id leak: %q", in.SessionID)
		}
		if in.CreatedAt.IsZero() {
			t.Errorf("created_at not parsed for row %s", in.ID)
		}
	}
	// First row has editor=cursor.
	var foundCursor bool
	for _, in := range got {
		if in.Editor == "cursor" {
			foundCursor = true
		}
	}
	if !foundCursor {
		t.Error("editor field not round-tripped on first interaction")
	}
}

func TestListInteractionsBySession_EmptyForUnknownSession(t *testing.T) {
	s := newCallsStore(t)
	got, err := s.ListInteractionsBySession("does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}
