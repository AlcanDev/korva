package store

import (
	"testing"
)

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		a, b      string
		wantAbove float64
		wantBelow float64
	}{
		{"identical", "use jwt for auth", "use jwt for auth", 1.0, 0},
		{"high overlap", "use jwt for authentication in API", "use jwt for authentication token", 0.6, 1.0},
		{"low overlap", "use jwt for auth", "use redis for caching", 0, 0.5},
		{"empty b", "use jwt for auth", "", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tokenize(tt.a), tokenize(tt.b))
			if got < tt.wantAbove {
				t.Errorf("similarity(%q, %q) = %.2f, want >= %.2f", tt.a, tt.b, got, tt.wantAbove)
			}
			if tt.wantBelow > 0 && got >= tt.wantBelow {
				t.Errorf("similarity(%q, %q) = %.2f, want < %.2f", tt.a, tt.b, got, tt.wantBelow)
			}
		})
	}
}

func TestFindSimilar_DetectsDuplicate(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: "decision",
		Title:   "Use JWT for API authentication",
		Content: "All API endpoints should use JWT tokens with RS256 signing for authentication. Tokens expire in 24 hours.",
	})

	candidate := Observation{
		Project: "proj", Type: "decision",
		Title:   "JWT authentication for API",
		Content: "All API endpoints must use JWT tokens with RS256 signing. Token expiry is 24 hours.",
	}

	similar, id := s.FindSimilar(candidate, 0.70)
	if similar == nil {
		t.Fatal("expected duplicate to be detected, got nil")
	}
	if id == "" {
		t.Error("expected non-empty similar ID")
	}
}

func TestFindSimilar_AllowsDistinctObservation(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: "decision",
		Title:   "Use Redis for session caching",
		Content: "Store session data in Redis with 30-minute TTL. Use cluster mode for high availability.",
	})

	candidate := Observation{
		Project: "proj", Type: "decision",
		Title:   "Use PostgreSQL for audit logs",
		Content: "All audit events must be written to PostgreSQL. Use write-ahead logging for durability.",
	}

	similar, _ := s.FindSimilar(candidate, 0.70)
	if similar != nil {
		t.Errorf("distinct observation was incorrectly flagged as duplicate: %q", similar.Title)
	}
}

func TestFindSimilar_IgnoresDifferentType(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: "pattern",
		Title:   "Use JWT for authentication across services",
		Content: "JWT with RS256 is the standard for inter-service auth in this project.",
	})

	// Same content but different type — should NOT trigger dedup
	candidate := Observation{
		Project: "proj", Type: "decision",
		Title:   "Use JWT for authentication across services",
		Content: "JWT with RS256 is the standard for inter-service auth in this project.",
	}

	similar, _ := s.FindSimilar(candidate, 0.70)
	if similar != nil {
		t.Errorf("different-type observation should not trigger dedup, got: %q", similar.Title)
	}
}

func TestFindSimilar_SkipsShortContent(t *testing.T) {
	s := newTestStore(t)

	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: "decision", Title: "Use JWT", Content: "JWT auth",
	})

	candidate := Observation{
		Project: "proj", Type: "decision", Title: "Use JWT", Content: "JWT",
	}

	// Short content (< 5 words) should bypass dedup entirely
	similar, _ := s.FindSimilar(candidate, 0.70)
	if similar != nil {
		t.Error("short content should bypass dedup check")
	}
}
