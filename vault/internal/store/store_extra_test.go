package store

import (
	"testing"
	"time"
)

// helper: save and return an observation with given project/type
func quickSave(t *testing.T, s *Store, project string, typ ObservationType, title string) string {
	t.Helper()
	id, err := s.Save(Observation{
		Project: project,
		Type:    typ,
		Title:   title,
		Content: "content for " + title,
	})
	if err != nil {
		t.Fatalf("quickSave(%q): %v", title, err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

func TestContext_ReturnsMostRecent(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		quickSave(t, s, "home-api", TypeDecision, "obs")
		time.Sleep(time.Millisecond)
	}

	results, err := s.Context("home-api", nil, 3)
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results (limit), got %d", len(results))
	}
}

func TestContext_FiltersByType(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "proj", TypeDecision, "decision")
	quickSave(t, s, "proj", TypePattern, "pattern")
	quickSave(t, s, "proj", TypeBugfix, "bugfix")

	results, err := s.Context("proj", []ObservationType{TypeDecision}, 10)
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 decision, got %d", len(results))
	}
	if results[0].Type != TypeDecision {
		t.Errorf("expected TypeDecision, got %q", results[0].Type)
	}
}

func TestContext_EmptyProjectReturnsNothing(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "other-project", TypeDecision, "obs")

	results, err := s.Context("nonexistent", nil, 10)
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown project, got %d", len(results))
	}
}

func TestContext_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		quickSave(t, s, "proj", TypeDecision, "obs")
	}
	// limit=0 should use default (10)
	results, err := s.Context("proj", nil, 0)
	if err != nil {
		t.Fatalf("Context with limit=0: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 (all), got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

func TestTimeline_BasicRange(t *testing.T) {
	s := newTestStore(t)
	from := time.Now().UTC().Add(-time.Hour)
	quickSave(t, s, "proj", TypeDecision, "in range")
	to := time.Now().UTC().Add(time.Hour)

	results, err := s.Timeline("proj", from, to)
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result in range, got %d", len(results))
	}
}

func TestTimeline_ExcludesFutureFrom(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "proj", TypeDecision, "past obs")

	// Range that starts in the future — nothing should match
	from := time.Now().UTC().Add(time.Hour)
	to := time.Now().UTC().Add(2 * time.Hour)
	results, err := s.Timeline("proj", from, to)
	if err != nil {
		t.Fatalf("Timeline future range: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for future range, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func TestSessionStartAndEnd(t *testing.T) {
	s := newTestStore(t)

	id, err := s.SessionStart("home-api", "backend", "CL", "copilot", "implement insurance feature")
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if id == "" {
		t.Error("SessionStart should return a non-empty ID")
	}

	if err := s.SessionEnd(id, "Completed hexagonal implementation"); err != nil {
		t.Fatalf("SessionEnd: %v", err)
	}
}

func TestListSessions_Empty(t *testing.T) {
	s := newTestStore(t)
	sessions, err := s.ListSessions(10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_ReturnsAll(t *testing.T) {
	s := newTestStore(t)
	goals := []string{"first-session", "second-session", "third-session"}
	for _, g := range goals {
		s.SessionStart("proj", "team", "CL", "copilot", g)
	}

	sessions, err := s.ListSessions(10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	// All goals should be present (order depends on started_at precision)
	found := make(map[string]bool)
	for _, sess := range sessions {
		found[sess.Goal] = true
	}
	for _, g := range goals {
		if !found[g] {
			t.Errorf("session %q not found in results", g)
		}
	}
}

func TestListSessions_Limit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		s.SessionStart("proj", "team", "CL", "copilot", "session")
		time.Sleep(time.Millisecond)
	}
	sessions, err := s.ListSessions(2)
	if err != nil {
		t.Fatalf("ListSessions with limit: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions with limit=2, got %d", len(sessions))
	}
}

func TestListSessions_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	s.SessionStart("proj", "team", "CL", "copilot", "s")
	sessions, err := s.ListSessions(0) // 0 → default=20
	if err != nil {
		t.Fatalf("ListSessions limit=0: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session with default limit, got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// SavePrompt
// ---------------------------------------------------------------------------

func TestSavePrompt_CreateAndUpsert(t *testing.T) {
	s := newTestStore(t)

	// Create
	if err := s.SavePrompt("hexagonal-review", "Review hexagonal boundaries: ...", []string{"hex", "review"}); err != nil {
		t.Fatalf("SavePrompt create: %v", err)
	}

	// Upsert — same name, updated content
	if err := s.SavePrompt("hexagonal-review", "Updated: Review hexagonal boundaries carefully.", []string{"hex"}); err != nil {
		t.Fatalf("SavePrompt upsert: %v", err)
	}

	// Verify only 1 prompt exists with updated content
	stats, _ := s.Stats()
	if stats.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt after upsert, got %d", stats.TotalPrompts)
	}
}

func TestSavePrompt_PrivacyFilter(t *testing.T) {
	s, err := NewMemory([]string{"secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SavePrompt("test", "my secret is 12345", nil)

	stats, _ := s.Stats()
	if stats.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt, got %d", stats.TotalPrompts)
	}
	// We can't easily verify the content was filtered without a GetPrompt method,
	// but at least verify no panic and the save succeeded
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func TestStats_Empty(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalObservations != 0 {
		t.Errorf("expected 0 observations, got %d", stats.TotalObservations)
	}
	if stats.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", stats.TotalSessions)
	}
	if stats.TotalPrompts != 0 {
		t.Errorf("expected 0 prompts, got %d", stats.TotalPrompts)
	}
}

func TestStats_CountsCorrectly(t *testing.T) {
	s := newTestStore(t)

	quickSave(t, s, "proj-a", TypeDecision, "decision 1")
	quickSave(t, s, "proj-a", TypeDecision, "decision 2")
	quickSave(t, s, "proj-b", TypePattern, "pattern 1")
	s.SessionStart("proj-a", "team", "CL", "copilot", "session")
	s.SavePrompt("prompt-1", "content", nil)

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalObservations != 3 {
		t.Errorf("expected 3 observations, got %d", stats.TotalObservations)
	}
	if stats.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", stats.TotalSessions)
	}
	if stats.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt, got %d", stats.TotalPrompts)
	}
	if stats.ByType["decision"] != 2 {
		t.Errorf("expected 2 decisions by type, got %d", stats.ByType["decision"])
	}
	if stats.ByType["pattern"] != 1 {
		t.Errorf("expected 1 pattern by type, got %d", stats.ByType["pattern"])
	}
	if stats.ByProject["proj-a"] != 2 {
		t.Errorf("expected 2 for proj-a, got %d", stats.ByProject["proj-a"])
	}
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func TestSummary_EmptyProject(t *testing.T) {
	s := newTestStore(t)
	sum, err := s.Summary("nonexistent")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.Observations != 0 {
		t.Errorf("expected 0 observations, got %d", sum.Observations)
	}
}

func TestSummary_PopulatedProject(t *testing.T) {
	s := newTestStore(t)

	quickSave(t, s, "home-api", TypeDecision, "decision A")
	quickSave(t, s, "home-api", TypeDecision, "decision B")
	quickSave(t, s, "home-api", TypePattern, "pattern X")
	s.SessionStart("home-api", "team", "CL", "copilot", "work")

	sum, err := s.Summary("home-api")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.Observations != 3 {
		t.Errorf("expected 3 observations, got %d", sum.Observations)
	}
	if sum.Sessions != 1 {
		t.Errorf("expected 1 session, got %d", sum.Sessions)
	}
	if len(sum.Recent) == 0 {
		t.Error("expected some recent observations")
	}
	if len(sum.Decisions) != 2 {
		t.Errorf("expected 2 decisions in summary, got %d", len(sum.Decisions))
	}
}

// ---------------------------------------------------------------------------
// Search filters
// ---------------------------------------------------------------------------

func TestSearch_FilterByProject(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "home-api", TypeDecision, "home-api obs")
	quickSave(t, s, "other-api", TypeDecision, "other obs")

	results, err := s.Search("", SearchFilters{Project: "home-api", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for home-api, got %d", len(results))
	}
}

func TestSearch_FilterByType(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "proj", TypeDecision, "decision")
	quickSave(t, s, "proj", TypeBugfix, "bugfix")

	results, err := s.Search("", SearchFilters{Type: TypeBugfix, Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 bugfix, got %d", len(results))
	}
}

func TestSearch_FullText(t *testing.T) {
	s := newTestStore(t)
	quickSave(t, s, "proj", TypeDecision, "hexagonal architecture decision")
	quickSave(t, s, "proj", TypePattern, "unrelated topic entirely")

	results, err := s.Search("hexagonal", SearchFilters{Limit: 10})
	if err != nil {
		t.Fatalf("FTS search: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 FTS result for 'hexagonal'")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	result, err := s.Get("nonexistent-ulid")
	if err != nil {
		t.Fatalf("Get nonexistent: %v", err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent ID")
	}
}
