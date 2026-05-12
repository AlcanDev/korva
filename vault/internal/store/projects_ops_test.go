package store

import (
	"sort"
	"testing"
)

func TestSuggestConsolidations_GroupsByNormalizedName(t *testing.T) {
	s := newTestStore(t)
	// alpha and Alpha-1 are different normalized forms; Alpha and ALPHA share one.
	for _, p := range []struct{ project, title string }{
		{"alpha", "a1"},
		{"alpha", "a2"},
		{"Alpha", "b1"},
		{"my-project", "c1"},
		{"my_project", "d1"},
		{"my_project", "d2"},
		{"isolated", "e1"},
	} {
		if _, err := s.Save(Observation{Project: p.project, Type: TypeDecision, Title: p.title, Content: "x"}); err != nil {
			t.Fatalf("seed %q: %v", p.project, err)
		}
	}

	got, err := s.SuggestConsolidations()
	if err != nil {
		t.Fatalf("SuggestConsolidations: %v", err)
	}
	// Only "alpha/Alpha" and "my-project/my_project" should surface.
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(got), got)
	}

	// Verify canonicals pick the variant with the most observations.
	for _, g := range got {
		var sources []string
		for _, v := range g.Variants {
			sources = append(sources, v.Name)
		}
		sort.Strings(sources)
		switch g.Canonical {
		case "alpha":
			if sources[0] != "Alpha" || sources[1] != "alpha" {
				t.Errorf("alpha group sources unexpected: %v", sources)
			}
		case "my_project":
			if sources[0] != "my-project" || sources[1] != "my_project" {
				t.Errorf("my_project group sources unexpected: %v", sources)
			}
		default:
			t.Errorf("unexpected canonical: %q", g.Canonical)
		}
	}
}

func TestSuggestConsolidations_EmptyWhenNoVariants(t *testing.T) {
	s := newTestStore(t)
	for _, p := range []string{"alpha", "beta", "gamma"} {
		if _, err := s.Save(Observation{Project: p, Type: TypeDecision, Title: p, Content: "x"}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, _ := s.SuggestConsolidations()
	if len(got) != 0 {
		t.Errorf("expected no suggestions, got %+v", got)
	}
}

func TestPruneEmptyProjects_DryRunListsButDoesNotDelete(t *testing.T) {
	s := newTestStore(t)
	// "korva" has an observation → not empty.
	if _, err := s.Save(Observation{Project: "korva", Type: TypeDecision, Title: "a", Content: "x"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// "abandoned" has only a session → empty.
	if _, err := s.SessionStart("abandoned", "", "", "agent", "test"); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	res, err := s.PruneEmptyProjects(PruneOptions{Apply: false})
	if err != nil {
		t.Fatalf("PruneEmptyProjects: %v", err)
	}
	if !res.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(res.Empty) != 1 || res.Empty[0].Project != "abandoned" {
		t.Errorf("expected 1 empty project (abandoned), got %+v", res.Empty)
	}
	if res.SessionsRemoved != 0 {
		t.Errorf("dry-run must not remove rows, got SessionsRemoved=%d", res.SessionsRemoved)
	}

	// Verify the session is still there.
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project = 'abandoned'`).Scan(&cnt)
	if cnt != 1 {
		t.Errorf("dry-run touched the DB: sessions count = %d", cnt)
	}
}

func TestPruneEmptyProjects_ApplyDeletesOrphanSessions(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Save(Observation{Project: "korva", Type: TypeDecision, Title: "a", Content: "x"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.SessionStart("abandoned", "", "", "agent", "test"); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	res, err := s.PruneEmptyProjects(PruneOptions{Apply: true})
	if err != nil {
		t.Fatalf("PruneEmptyProjects: %v", err)
	}
	if res.DryRun {
		t.Error("expected DryRun=false in apply mode")
	}
	if res.SessionsRemoved != 1 {
		t.Errorf("expected 1 session removed, got %d", res.SessionsRemoved)
	}
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project = 'abandoned'`).Scan(&cnt)
	if cnt != 0 {
		t.Errorf("after apply, sessions count = %d, want 0", cnt)
	}
}
