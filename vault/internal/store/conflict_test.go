package store

import "testing"

func TestFindDecisionConflicts_DetectsConflict(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: TypeDecision,
		Title:   "Use PostgreSQL for primary storage",
		Content: "We use PostgreSQL as the primary relational database. All services query it directly.",
	})

	candidate := Observation{
		Project: "proj", Type: TypeDecision,
		Title:   "Switch to MongoDB document store",
		Content: "Use MongoDB as the main storage engine. Its document model fits our schema better.",
	}

	warnings := s.FindDecisionConflicts(candidate)
	if len(warnings) == 0 {
		t.Fatal("expected conflict warning for sql vs document store, got none")
	}
	if warnings[0].ExistingTitle == "" {
		t.Error("conflict warning missing existing title")
	}
}

func TestFindDecisionConflicts_NoConflictForSameTechnology(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: TypeDecision,
		Title:   "Use JWT for auth",
		Content: "All API calls authenticated via JWT bearer tokens.",
	})

	candidate := Observation{
		Project: "proj", Type: TypeDecision,
		Title:   "Extend JWT with refresh tokens",
		Content: "JWT access tokens expire in 15 min; refresh tokens last 7 days.",
	}

	warnings := s.FindDecisionConflicts(candidate)
	if len(warnings) > 0 {
		t.Errorf("no conflict expected for same-technology decisions, got: %+v", warnings)
	}
}

func TestFindDecisionConflicts_SkipsNonDecision(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: TypeDecision,
		Title:   "Use PostgreSQL",
		Content: "Use PostgreSQL as primary relational DB.",
	})

	candidate := Observation{
		Project: "proj", Type: TypePattern, // pattern, not decision
		Title:   "MongoDB document model",
		Content: "Mongo documents for nested schemas.",
	}

	warnings := s.FindDecisionConflicts(candidate)
	if len(warnings) != 0 {
		t.Error("conflict detector should skip non-decision types")
	}
}

func TestFindDecisionConflicts_LimitsToThree(t *testing.T) {
	s := newTestStore(t)

	// Save four conflicting decisions
	for _, title := range []string{
		"Use REST for all endpoints",
		"Use REST for service A",
		"Use REST for service B",
		"Use REST for service C",
	} {
		s.Save(Observation{ //nolint:errcheck
			Project: "proj", Type: TypeDecision, Title: title, Content: "REST is the API style.",
		})
	}

	candidate := Observation{
		Project: "proj", Type: TypeDecision,
		Title:   "Migrate all endpoints to gRPC",
		Content: "gRPC provides better performance than REST for internal services.",
	}

	warnings := s.FindDecisionConflicts(candidate)
	if len(warnings) > 3 {
		t.Errorf("conflict warnings capped at 3, got %d", len(warnings))
	}
}
