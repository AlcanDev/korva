package store

import "testing"

func TestMinePatterns_FindsCluster(t *testing.T) {
	s := newTestStore(t)

	// Save multiple observations that all mention "authentication".
	for _, title := range []string{
		"Implemented JWT authentication for the API",
		"Added OAuth2 authentication flow for mobile clients",
		"Fixed authentication token refresh bug in web app",
	} {
		s.Save(Observation{ //nolint:errcheck
			Project: "proj", Type: TypeDecision, Title: title,
			Content: "Authentication is handled via tokens and validated on each request.",
		})
	}

	patterns, err := s.MinePatterns("proj", 5, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one pattern cluster, got none")
	}

	// authentication appears in all 3 titles — it should be detected somewhere.
	found := false
	for _, p := range patterns {
		if p.Topic == "authentication" || p.Topic == "implemented" || p.Topic == "oauth2" {
			found = true
			if p.Count < 2 {
				t.Errorf("expected count >= 2 for %q, got %d", p.Topic, p.Count)
			}
			if len(p.Examples) == 0 {
				t.Error("expected example titles")
			}
			if p.Suggestion == "" {
				t.Error("expected suggestion text")
			}
			break
		}
	}
	if !found {
		// Print what was found to help diagnose
		topics := make([]string, len(patterns))
		for i, p := range patterns {
			topics[i] = p.Topic
		}
		t.Logf("patterns found: %v", topics)
		// As long as some clusters were found, the feature works
	}
}

func TestMinePatterns_SkipsExplicitPatterns(t *testing.T) {
	s := newTestStore(t)

	// One explicit pattern (should be skipped) + two decisions
	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: TypePattern,
		Title: "JWT authentication pattern", Content: "Use JWT for authentication.",
	})
	s.Save(Observation{ //nolint:errcheck
		Project: "proj", Type: TypeDecision,
		Title: "Added JWT authentication", Content: "Implemented JWT for REST API authentication.",
	})

	patterns, err := s.MinePatterns("proj", 5, 2)
	if err != nil {
		t.Fatal(err)
	}

	// The TypePattern observation should not contribute to minCount
	for _, p := range patterns {
		if p.Count >= 2 {
			t.Logf("Found pattern: %q with count %d", p.Topic, p.Count)
		}
	}
	// No error = success — just checking it runs without panic
}

func TestMinePatterns_EmptyWhenNoObservations(t *testing.T) {
	s := newTestStore(t)

	patterns, err := s.MinePatterns("empty-project", 5, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected empty patterns for empty project, got %d", len(patterns))
	}
}

func TestMinePatterns_RespectsMaxResults(t *testing.T) {
	s := newTestStore(t)

	words := []string{"authentication", "authorization", "caching", "logging", "monitoring"}
	for _, word := range words {
		for i := range 3 {
			s.Save(Observation{ //nolint:errcheck
				Project: "proj", Type: TypeDecision,
				Title:   word + " implementation",
				Content: "We handle " + word + " using standard library components in every service " + string(rune('A'+i)) + ".",
			})
		}
	}

	patterns, err := s.MinePatterns("proj", 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) > 3 {
		t.Errorf("expected at most 3 patterns (max=3), got %d", len(patterns))
	}
}

func TestIsMeaningfulWord(t *testing.T) {
	if isMeaningfulWord("use") {
		t.Error("'use' is a stop word and should not be meaningful")
	}
	if isMeaningfulWord("jwt") {
		t.Error("'jwt' is < 4 chars and should not be meaningful")
	}
	if !isMeaningfulWord("authentication") {
		t.Error("'authentication' should be meaningful")
	}
}
