package store

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewMemory(nil)
	if err != nil {
		t.Fatalf("NewMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveAndGet(t *testing.T) {
	s := newTestStore(t)

	obs := Observation{
		Project: "home-api",
		Team:    "backend-seguros",
		Country: "CL",
		Type:    TypeDecision,
		Title:   "Use Template Method for country adapters",
		Content: "We decided to use Template Method pattern for CL/PE/CO adapters",
		Tags:    []string{"hexagonal", "adapter", "template-method"},
		Author:  "felipe",
	}

	id, err := s.Save(obs)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if id == "" {
		t.Error("Save() should return a non-empty ID")
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Title != obs.Title {
		t.Errorf("Title = %q, want %q", got.Title, obs.Title)
	}
	if len(got.Tags) != 3 {
		t.Errorf("Tags = %v, want 3 tags", got.Tags)
	}
}

func TestSavePrivacyFilter(t *testing.T) {
	s := newTestStore(t)

	obs := Observation{
		Project: "home-api",
		Type:    TypeContext,
		Title:   "Config with password=supersecret",
		Content: "token=abc123 is used for auth",
		Tags:    []string{},
	}

	id, err := s.Save(obs)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, _ := s.Get(id)
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Title == obs.Title {
		t.Error("Title should have been filtered (contains 'password=')")
	}
	if got.Content == obs.Content {
		t.Error("Content should have been filtered (contains 'token=')")
	}
}

func TestSearch(t *testing.T) {
	s := newTestStore(t)

	observations := []Observation{
		{Project: "home-api", Team: "backend", Type: TypeDecision, Title: "Use hexagonal architecture", Content: "Domain ports and adapters", Tags: []string{"hexagonal"}},
		{Project: "home-api", Team: "backend", Type: TypePattern, Title: "Template Method for adapters", Content: "Base class per country", Tags: []string{"template-method"}},
		{Project: "checkout", Team: "payments", Type: TypeBugfix, Title: "Fix null pointer in payment", Content: "Null check on response", Tags: []string{"bugfix"}},
	}

	for _, obs := range observations {
		if _, err := s.Save(obs); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// FTS search
	results, err := s.Search("hexagonal", SearchFilters{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("Search('hexagonal') should return at least 1 result")
	}

	// Filter by project
	results, err = s.Search("", SearchFilters{Project: "checkout", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search by project 'checkout' = %d results, want 1", len(results))
	}

	// Filter by team
	results, err = s.Search("", SearchFilters{Team: "backend", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search by team 'backend' = %d results, want 2", len(results))
	}
}

func TestContext(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		s.Save(Observation{
			Project: "home-api",
			Type:    TypeDecision,
			Title:   "Decision " + string(rune('A'+i)),
			Content: "Content",
			Tags:    []string{},
		})
	}
	s.Save(Observation{Project: "other", Type: TypePattern, Title: "Other project", Content: "x", Tags: []string{}})

	ctx, err := s.Context("home-api", nil, 3)
	if err != nil {
		t.Fatalf("Context() error = %v", err)
	}
	if len(ctx) != 3 {
		t.Errorf("Context() = %d results, want 3", len(ctx))
	}
	for _, o := range ctx {
		if o.Project != "home-api" {
			t.Errorf("Context() returned observation from project %q, want 'home-api'", o.Project)
		}
	}
}

func TestTimeline(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{Project: "home-api", Type: TypeDecision, Title: "Old", Content: "x", Tags: []string{}})

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)

	results, err := s.Timeline("home-api", from, to)
	if err != nil {
		t.Fatalf("Timeline() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("Timeline() should return recent observations")
	}
}

func TestSessionStartEnd(t *testing.T) {
	s := newTestStore(t)

	id, err := s.SessionStart("home-api", "backend", "CL", "copilot", "Implement insurance module")
	if err != nil {
		t.Fatalf("SessionStart() error = %v", err)
	}
	if id == "" {
		t.Error("SessionStart() should return a non-empty ID")
	}

	if err := s.SessionEnd(id, "Implemented InsurancePort and adapter base"); err != nil {
		t.Fatalf("SessionEnd() error = %v", err)
	}
}

func TestSummary(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{Project: "home-api", Type: TypeDecision, Title: "Decision 1", Content: "x", Tags: []string{}})
	s.Save(Observation{Project: "home-api", Type: TypePattern, Title: "Pattern 1", Content: "y", Tags: []string{}})
	s.SessionStart("home-api", "", "", "copilot", "")

	summary, err := s.Summary("home-api")
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary.Observations != 2 {
		t.Errorf("Summary.Observations = %d, want 2", summary.Observations)
	}
	if summary.Sessions != 1 {
		t.Errorf("Summary.Sessions = %d, want 1", summary.Sessions)
	}
}

func TestSavePrompt(t *testing.T) {
	s := newTestStore(t)

	err := s.SavePrompt("arch-review", "Review this code for hexagonal violations:", []string{"arch", "review"})
	if err != nil {
		t.Fatalf("SavePrompt() error = %v", err)
	}

	// Idempotent upsert
	err = s.SavePrompt("arch-review", "Updated content", []string{"arch"})
	if err != nil {
		t.Fatalf("SavePrompt() upsert error = %v", err)
	}
}

func TestStats(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{Project: "home-api", Team: "backend", Type: TypeDecision, Title: "A", Content: "x", Tags: []string{}})
	s.Save(Observation{Project: "home-api", Team: "backend", Type: TypePattern, Title: "B", Content: "y", Tags: []string{}})
	s.Save(Observation{Project: "checkout", Team: "payments", Type: TypeBugfix, Title: "C", Content: "z", Tags: []string{}})

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalObservations != 3 {
		t.Errorf("TotalObservations = %d, want 3", stats.TotalObservations)
	}
	if stats.ByProject["home-api"] != 2 {
		t.Errorf("ByProject[home-api] = %d, want 2", stats.ByProject["home-api"])
	}
}
