package store

import (
	"strings"
	"testing"
	"time"
)

// TestNormalizeForHash_CollapsesWhitespaceAndCase pins the contract of the
// normaliser feeding the new project-scoped dedup. The hash must be invariant
// under capitalisation and whitespace differences but sensitive to actual
// content changes.
func TestNormalizeForHash_CollapsesWhitespaceAndCase(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		same bool
	}{
		{"case differs", "Hello World", "hello world", true},
		{"extra spaces", "hello   world", "hello world", true},
		{"newlines", "hello\nworld", "hello world", true},
		{"tabs", "hello\tworld", "hello world", true},
		{"trailing whitespace", "hello world   ", "hello world", true},
		{"different content", "hello world", "hello mundo", false},
		{"different order", "hello world", "world hello", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ha := normalizeForHash(tc.a)
			hb := normalizeForHash(tc.b)
			if tc.same && ha != hb {
				t.Errorf("expected %q ≡ %q after normalisation, got %q vs %q", tc.a, tc.b, ha, hb)
			}
			if !tc.same && ha == hb {
				t.Errorf("expected %q ≢ %q, both normalised to %q", tc.a, tc.b, ha)
			}
		})
	}
}

// TestSave_NormalizedDedup_BumpsDuplicateCount confirms that a duplicate save
// increments duplicate_count on the original row, and that the returned ID is
// the original (not a new one).
func TestSave_NormalizedDedup_BumpsDuplicateCount(t *testing.T) {
	s := newTestStore(t)
	obs := Observation{
		Project: "alpha",
		Type:    TypeDecision,
		Title:   "use ULID for IDs",
		Content: "ULIDs are sortable and URL-safe",
	}
	id1, err := s.Save(obs)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Second save with cosmetic differences (case + whitespace) must hit the
	// same row, not create a new one.
	obs.Title = "Use   ULID for IDs"
	obs.Content = "ULIDs are\n  sortable and URL-safe"
	id2, err := s.Save(obs)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if id1 != id2 {
		t.Errorf("normalized dedup should coalesce; got distinct ids %s vs %s", id1, id2)
	}

	// Verify duplicate_count and last_seen_at columns advanced.
	var dup int
	var lastSeen *string
	if err := s.db.QueryRow(
		`SELECT duplicate_count, last_seen_at FROM observations WHERE id = ?`, id1,
	).Scan(&dup, &lastSeen); err != nil {
		t.Fatalf("read columns: %v", err)
	}
	if dup != 1 {
		t.Errorf("duplicate_count = %d, want 1", dup)
	}
	if lastSeen == nil || *lastSeen == "" {
		t.Errorf("last_seen_at should be populated after a dedup hit, got %v", lastSeen)
	}
}

// TestSave_NormalizedDedup_OutsideWindowCreatesNewRow verifies that a duplicate
// save with a created_at older than the dedup window is treated as a fresh
// learning, so timeline replays preserve "we re-discovered this 3 days later".
func TestSave_NormalizedDedup_OutsideWindowCreatesNewRow(t *testing.T) {
	s := newTestStore(t)
	old := Observation{
		Project: "alpha",
		Type:    TypeLearning,
		Title:   "Stale insight",
		Content: "Some lesson",
		// Force created_at older than the dedup window so the second save
		// no longer sees it as a candidate.
		CreatedAt: time.Now().UTC().Add(-dedupNormalizedWindow - time.Minute),
	}
	id1, err := s.Save(old)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	id2, err := s.Save(Observation{
		Project: "alpha",
		Type:    TypeLearning,
		Title:   "Stale insight",
		Content: "Some lesson",
	})
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if id1 == id2 {
		t.Errorf("outside the dedup window, the second save must create a new row; got id %s twice", id1)
	}
}

// TestSave_NormalizedHashPopulatedOnInsert checks the new column is filled in
// on every fresh insert. Without it the dedup query would never match anything.
func TestSave_NormalizedHashPopulatedOnInsert(t *testing.T) {
	s := newTestStore(t)
	id, err := s.Save(Observation{
		Project: "alpha",
		Type:    TypeBugfix,
		Title:   "Some title",
		Content: "Some content",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	var nh string
	if err := s.db.QueryRow(
		`SELECT normalized_hash FROM observations WHERE id = ?`, id,
	).Scan(&nh); err != nil {
		t.Fatalf("read normalized_hash: %v", err)
	}
	if !looksLikeHash(nh) {
		t.Errorf("normalized_hash = %q, want 32-hex string", nh)
	}
}

// looksLikeHash returns true when s is exactly the 32-hex prefix our helpers emit.
func looksLikeHash(s string) bool {
	if len(s) != 32 {
		return false
	}
	const hex = "0123456789abcdef"
	for _, c := range s {
		if !strings.ContainsRune(hex, c) {
			return false
		}
	}
	return true
}
