package store

import (
	"testing"
)

// TestEnsurePendingForSource_NewCandidates inserts a row per candidate when
// none exists yet, and the JudgmentID is populated for each entry.
func TestEnsurePendingForSource_NewCandidates(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)

	cands := make([]Observation, 0, len(otherIDs))
	for _, id := range otherIDs {
		o, _ := s.Get(id)
		if o != nil {
			cands = append(cands, *o)
		}
	}

	got, err := s.EnsurePendingForSource(sourceID, cands)
	if err != nil {
		t.Fatalf("EnsurePendingForSource: %v", err)
	}
	if len(got) != len(cands) {
		t.Fatalf("expected one entry per candidate, got %d for %d", len(got), len(cands))
	}
	for _, e := range got {
		if e.JudgmentID == "" {
			t.Errorf("missing judgment_id for candidate %s", e.CandidateID)
		}
		if e.AlreadyExisted {
			t.Errorf("expected new row, got AlreadyExisted=true for %s", e.CandidateID)
		}
		if e.CandidateTitle == "" {
			t.Errorf("missing candidate_title for %s", e.CandidateID)
		}
	}
}

// TestEnsurePendingForSource_Idempotent confirms a second call returns the
// same judgment_ids and marks each entry AlreadyExisted=true.
func TestEnsurePendingForSource_Idempotent(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)
	target, _ := s.Get(otherIDs[0])

	first, err := s.EnsurePendingForSource(sourceID, []Observation{*target})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first) != 1 || first[0].AlreadyExisted {
		t.Fatalf("first call should create one new row, got %+v", first)
	}

	second, err := s.EnsurePendingForSource(sourceID, []Observation{*target})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("second call should return one entry, got %d", len(second))
	}
	if !second[0].AlreadyExisted {
		t.Error("second call must flag AlreadyExisted=true")
	}
	if second[0].JudgmentID != first[0].JudgmentID {
		t.Errorf("judgment_id changed across calls: %s vs %s",
			first[0].JudgmentID, second[0].JudgmentID)
	}
}

// TestEnsurePendingForSource_SkipsCrossProject verifies project isolation.
func TestEnsurePendingForSource_SkipsCrossProject(t *testing.T) {
	s := newTestStore(t)
	src := quickSave(t, s, "alpha", TypeDecision, "alpha src")
	betaID := quickSave(t, s, "beta", TypeDecision, "beta candidate")
	beta, _ := s.Get(betaID)

	got, err := s.EnsurePendingForSource(src, []Observation{*beta})
	if err != nil {
		t.Fatalf("EnsurePendingForSource: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-project candidate must be skipped, got %d entries", len(got))
	}
}

// TestEnsurePendingForSource_NoSourceReturnsError ensures the helper fails
// loudly when called on a missing source observation.
func TestEnsurePendingForSource_NoSourceReturnsError(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.EnsurePendingForSource("01DOESNOTEXIST", []Observation{
		{ID: "01OTHER", Project: "p", Title: "x"},
	}); err == nil {
		t.Error("expected error for missing source")
	}
}
