package store

import (
	"testing"
	"time"
)

// --- Purge ---

func TestPurge_RequiresFilter(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	_, err := s.Purge(PurgeOptions{})
	if err == nil {
		t.Fatal("want error when no filter provided, got nil")
	}
}

func TestPurge_ByProject(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	save := func(project, typ string) {
		s.Save(Observation{Project: project, Type: ObservationType(typ), Title: "t", Content: "c"}) //nolint:errcheck
	}
	save("alpha", "pattern")
	save("alpha", "pattern")
	save("beta", "pattern")

	n, err := s.Purge(PurgeOptions{Project: "alpha"})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 deleted, got %d", n)
	}

	// beta untouched.
	results, _ := s.Search("", SearchFilters{Project: "beta", Limit: 10})
	if len(results) != 1 {
		t.Fatalf("beta should have 1 observation, got %d", len(results))
	}
}

func TestPurge_DryRun(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	s.Save(Observation{Project: "demo", Type: "pattern", Title: "t", Content: "c"}) //nolint:errcheck
	s.Save(Observation{Project: "demo", Type: "pattern", Title: "t2", Content: "c2"}) //nolint:errcheck

	n, err := s.Purge(PurgeOptions{Project: "demo", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run purge: %v", err)
	}
	if n != 2 {
		t.Fatalf("dry-run: want count=2, got %d", n)
	}

	// Database must be untouched.
	results, _ := s.Search("", SearchFilters{Project: "demo", Limit: 10})
	if len(results) != 2 {
		t.Fatalf("dry-run must not delete: want 2, got %d", len(results))
	}
}

func TestPurge_ByBefore(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	old := Observation{Project: "proj", Type: "pattern", Title: "old", Content: "old content"}
	old.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)
	s.Save(old) //nolint:errcheck

	s.Save(Observation{Project: "proj", Type: "pattern", Title: "new", Content: "new content"}) //nolint:errcheck

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	n, err := s.Purge(PurgeOptions{Project: "proj", Before: cutoff})
	if err != nil {
		t.Fatalf("purge by before: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 deleted (old), got %d", n)
	}
}

// --- Export ---

func TestExport_All(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	for i := 0; i < 3; i++ {
		s.Save(Observation{Project: "p", Type: "pattern", Title: "t", Content: "c"}) //nolint:errcheck
	}

	obs, err := s.Export(ExportOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("want 3, got %d", len(obs))
	}
}

func TestExport_Filtered(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	s.Save(Observation{Project: "alpha", Type: "pattern", Title: "a", Content: "a"}) //nolint:errcheck
	s.Save(Observation{Project: "beta", Type: "decision", Title: "b", Content: "b"}) //nolint:errcheck
	s.Save(Observation{Project: "alpha", Type: "decision", Title: "c", Content: "c"}) //nolint:errcheck

	obs, err := s.Export(ExportOptions{Project: "alpha"})
	if err != nil {
		t.Fatalf("export filtered: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("want 2 (alpha only), got %d", len(obs))
	}
}

// --- Dedup ---

func TestDedup_NoDuplicates(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	s.Save(Observation{Project: "p", Type: "pattern", Title: "a", Content: "unique-a"}) //nolint:errcheck
	s.Save(Observation{Project: "p", Type: "pattern", Title: "b", Content: "unique-b"}) //nolint:errcheck

	res, err := s.Dedup("", false)
	if err != nil {
		t.Fatalf("dedup: %v", err)
	}
	if res.Duplicates != 0 || res.Removed != 0 {
		t.Fatalf("expected no dups, got %+v", res)
	}
}

func TestDedup_RemovesDuplicates(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	// Save the same (project, type, content) three times.
	for i := 0; i < 3; i++ {
		s.Save(Observation{Project: "proj", Type: "pattern", Title: "title", Content: "same content"}) //nolint:errcheck
	}

	res, err := s.Dedup("proj", false)
	if err != nil {
		t.Fatalf("dedup: %v", err)
	}
	if res.Duplicates != 2 {
		t.Fatalf("want 2 duplicates, got %d", res.Duplicates)
	}
	if res.Removed != 2 {
		t.Fatalf("want 2 removed, got %d", res.Removed)
	}

	// Only one should remain.
	remaining, _ := s.Search("", SearchFilters{Project: "proj", Limit: 10})
	if len(remaining) != 1 {
		t.Fatalf("want 1 remaining, got %d", len(remaining))
	}
}

func TestDedup_DryRun(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	for i := 0; i < 2; i++ {
		s.Save(Observation{Project: "proj", Type: "pattern", Title: "t", Content: "dup"}) //nolint:errcheck
	}

	res, err := s.Dedup("proj", true)
	if err != nil {
		t.Fatalf("dedup dry-run: %v", err)
	}
	if res.Duplicates != 1 {
		t.Fatalf("want 1 dup found, got %d", res.Duplicates)
	}
	if res.Removed != 0 {
		t.Fatal("dry-run must not remove anything")
	}

	// Verify database is untouched.
	all, _ := s.Search("", SearchFilters{Project: "proj", Limit: 10})
	if len(all) != 2 {
		t.Fatalf("dry-run: want 2 rows, got %d", len(all))
	}
}

func TestDedup_DifferentTypesNotDuplicates(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	// Same content but different types — should NOT be deduped.
	s.Save(Observation{Project: "p", Type: "pattern", Title: "t", Content: "same"}) //nolint:errcheck
	s.Save(Observation{Project: "p", Type: "decision", Title: "t", Content: "same"}) //nolint:errcheck

	res, err := s.Dedup("p", false)
	if err != nil {
		t.Fatalf("dedup: %v", err)
	}
	if res.Duplicates != 0 {
		t.Fatalf("different types: want 0 dups, got %d", res.Duplicates)
	}
}

// --- SearchFilters Since/Until ---

func TestSearch_SinceUntil(t *testing.T) {
	s, _ := NewMemory(nil)
	defer s.Close()

	old := Observation{Project: "p", Type: "pattern", Title: "old", Content: "old obs"}
	old.CreatedAt = time.Now().Add(-48 * time.Hour)
	s.Save(old) //nolint:errcheck

	s.Save(Observation{Project: "p", Type: "pattern", Title: "new", Content: "new obs"}) //nolint:errcheck

	cutoff := time.Now().Add(-24 * time.Hour)

	recent, err := s.Search("", SearchFilters{
		Project: "p",
		Since:   cutoff,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search since: %v", err)
	}
	if len(recent) != 1 || recent[0].Title != "new" {
		t.Fatalf("since filter: want only 'new', got %+v", recent)
	}

	before, err := s.Search("", SearchFilters{
		Project: "p",
		Until:   cutoff,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search until: %v", err)
	}
	if len(before) != 1 || before[0].Title != "old" {
		t.Fatalf("until filter: want only 'old', got %+v", before)
	}
}
