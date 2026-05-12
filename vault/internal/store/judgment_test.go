package store

import (
	"errors"
	"strings"
	"testing"
)

// helper: stages three observations in the same project with overlapping
// titles so FindRelationCandidates can find something to score.
func seedJudgmentCorpus(t *testing.T, s *Store) (sourceID string, otherIDs []string) {
	t.Helper()
	sourceID = quickSave(t, s, "korva", TypeDecision, "use ULID for primary keys")
	otherIDs = []string{
		quickSave(t, s, "korva", TypePattern, "ULID format guarantees lexicographic sort order"),
		quickSave(t, s, "korva", TypeLearning, "ULID monotonic component handles clock drift"),
		quickSave(t, s, "korva", TypePattern, "unrelated topic about API rate limits"),
	}
	return
}

func TestFindRelationCandidates_ReturnsSimilarObservations(t *testing.T) {
	s := newTestStore(t)
	sourceID, _ := seedJudgmentCorpus(t, s)

	got, err := s.FindRelationCandidates(sourceID, FindRelationCandidatesOpts{Limit: 5})
	if err != nil {
		t.Fatalf("FindRelationCandidates: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one ULID-related candidate")
	}
	for _, c := range got {
		if c.ID == sourceID {
			t.Error("source must never appear in its own candidates")
		}
		if !strings.Contains(strings.ToLower(c.Title), "ulid") &&
			!strings.Contains(strings.ToLower(c.Content), "ulid") {
			t.Logf("candidate %q does not mention ULID — BM25 picked it on overlap, OK", c.Title)
		}
	}
}

func TestFindRelationCandidates_ProjectScoped(t *testing.T) {
	s := newTestStore(t)
	sourceID := quickSave(t, s, "alpha", TypeDecision, "shared topic")
	_ = quickSave(t, s, "beta", TypeDecision, "shared topic")

	got, _ := s.FindRelationCandidates(sourceID, FindRelationCandidatesOpts{})
	for _, c := range got {
		if c.Project != "alpha" {
			t.Errorf("candidate from project %q leaked through — must be scoped to source project", c.Project)
		}
	}
}

func TestFindRelationCandidates_EmptyTitleNoResults(t *testing.T) {
	s := newTestStore(t)
	// Save with content only; the buildFTSQueryFromObservation uses title.
	id, err := s.Save(Observation{
		Project: "korva",
		Type:    TypeDecision,
		Content: "lorem ipsum",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.FindRelationCandidates(id, FindRelationCandidatesOpts{})
	if err != nil {
		t.Fatalf("FindRelationCandidates: %v", err)
	}
	if got != nil {
		t.Errorf("empty-title source should yield nil, got %d", len(got))
	}
}

func TestCreatePendingJudgments_Idempotent(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)

	cands := make([]Observation, 0, len(otherIDs))
	for _, id := range otherIDs {
		obs, _ := s.Get(id)
		if obs != nil {
			cands = append(cands, *obs)
		}
	}

	first, err := s.CreatePendingJudgments(sourceID, cands)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first) != len(otherIDs) {
		t.Errorf("expected %d new judgments, got %d", len(otherIDs), len(first))
	}

	second, err := s.CreatePendingJudgments(sourceID, cands)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second call should be a no-op, got %d new", len(second))
	}
}

func TestJudge_FlipsPendingToJudged(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)
	target, _ := s.Get(otherIDs[0])
	created, err := s.CreatePendingJudgments(sourceID, []Observation{*target})
	if err != nil || len(created) != 1 {
		t.Fatalf("seed pending: err=%v created=%v", err, created)
	}
	judgmentID := created[0]

	got, err := s.Judge(judgmentID, JudgeInput{
		Relation:      RelationSupersedes,
		Reason:        "newer insight",
		Evidence:      "decision postdates the pattern note",
		Confidence:    0.92,
		MarkedByActor: ActorAgent,
		MarkedByKind:  VerdictHeuristic,
		MarkedByModel: "claude-opus-4-7",
		SessionID:     "01TESTSESSION",
	})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if got.JudgmentStatus != JudgmentJudged {
		t.Errorf("status = %q, want judged", got.JudgmentStatus)
	}
	if got.Relation != RelationSupersedes {
		t.Errorf("relation = %q, want supersedes", got.Relation)
	}
	if got.Confidence < 0.91 || got.Confidence > 0.93 {
		t.Errorf("confidence = %g, want 0.92", got.Confidence)
	}
	if got.MarkedByModel != "claude-opus-4-7" {
		t.Errorf("model = %q, want claude-opus-4-7", got.MarkedByModel)
	}
	if got.JudgedAt == nil {
		t.Error("judged_at must be populated after Judge")
	}
}

func TestJudge_AlreadyJudgedReturnsNotFound(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)
	target, _ := s.Get(otherIDs[0])
	created, _ := s.CreatePendingJudgments(sourceID, []Observation{*target})

	if _, err := s.Judge(created[0], JudgeInput{Relation: RelationRelated}); err != nil {
		t.Fatalf("first judge: %v", err)
	}
	_, err := s.Judge(created[0], JudgeInput{Relation: RelationConflicts})
	if !errors.Is(err, ErrJudgmentNotFound) {
		t.Errorf("second judge should return ErrJudgmentNotFound, got %v", err)
	}
}

func TestJudge_ValidationErrors(t *testing.T) {
	s := newTestStore(t)
	tests := []struct {
		name string
		in   JudgeInput
	}{
		{"empty relation", JudgeInput{}},
		{"unknown relation", JudgeInput{Relation: "yelling"}},
		{"confidence too high", JudgeInput{Relation: RelationRelated, Confidence: 1.5}},
		{"confidence negative", JudgeInput{Relation: RelationRelated, Confidence: -0.1}},
		{"unknown actor", JudgeInput{Relation: RelationRelated, MarkedByActor: "robot"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Judge("fake-id", tc.in); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestIgnoreJudgment_HidesFromPendingListing(t *testing.T) {
	s := newTestStore(t)
	sourceID, otherIDs := seedJudgmentCorpus(t, s)
	target, _ := s.Get(otherIDs[0])
	created, _ := s.CreatePendingJudgments(sourceID, []Observation{*target})

	if err := s.IgnoreJudgment(created[0], "noise", ""); err != nil {
		t.Fatalf("IgnoreJudgment: %v", err)
	}
	pending, _ := s.ListPendingJudgments("korva", 10)
	for _, p := range pending {
		if p.ID == created[0] {
			t.Error("ignored judgment must not appear in pending listing")
		}
	}
}

func TestListPendingJudgments_ScopedToProject(t *testing.T) {
	s := newTestStore(t)
	a1 := quickSave(t, s, "alpha", TypeDecision, "a1")
	a2 := quickSave(t, s, "alpha", TypeDecision, "a2")
	b1 := quickSave(t, s, "beta", TypeDecision, "b1")
	b2 := quickSave(t, s, "beta", TypeDecision, "b2")

	aObs, _ := s.Get(a2)
	bObs, _ := s.Get(b2)
	if _, err := s.CreatePendingJudgments(a1, []Observation{*aObs}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePendingJudgments(b1, []Observation{*bObs}); err != nil {
		t.Fatal(err)
	}

	got, _ := s.ListPendingJudgments("alpha", 10)
	if len(got) != 1 || got[0].Project != "alpha" {
		t.Errorf("alpha-scoped listing leaked: %+v", got)
	}
}

func TestCompareAndStore_NewPairCreatesJudgedRow(t *testing.T) {
	s := newTestStore(t)
	src := quickSave(t, s, "alpha", TypeDecision, "src")
	tgt := quickSave(t, s, "alpha", TypePattern, "tgt")

	id, err := s.CompareAndStore(CompareInput{
		SourceID: src, TargetID: tgt,
		Relation:      RelationRelated,
		Confidence:    0.8,
		MarkedByActor: ActorAgent,
		MarkedByModel: "claude-opus-4-7",
	})
	if err != nil {
		t.Fatalf("CompareAndStore: %v", err)
	}
	got, _ := s.GetJudgment(id)
	if got == nil {
		t.Fatal("expected stored row")
	}
	if got.JudgmentStatus != JudgmentJudged {
		t.Errorf("status = %q, want judged", got.JudgmentStatus)
	}
	if got.MarkedByKind != VerdictLLM {
		t.Errorf("kind = %q, want llm", got.MarkedByKind)
	}
}

func TestCompareAndStore_UpsertsExistingPair(t *testing.T) {
	s := newTestStore(t)
	src := quickSave(t, s, "alpha", TypeDecision, "src")
	tgt := quickSave(t, s, "alpha", TypePattern, "tgt")

	in := CompareInput{
		SourceID: src, TargetID: tgt,
		Relation: RelationRelated, Confidence: 0.5, MarkedByActor: ActorAgent,
	}
	id1, _ := s.CompareAndStore(in)

	in.Relation = RelationSupersedes
	in.Confidence = 0.9
	id2, err := s.CompareAndStore(in)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if id1 != id2 {
		t.Errorf("upsert should keep the same row id, got %s then %s", id1, id2)
	}
	got, _ := s.GetJudgment(id2)
	if got.Relation != RelationSupersedes || got.Confidence < 0.89 {
		t.Errorf("upserted row did not refresh: %+v", got)
	}
}

func TestCompareAndStore_RejectsCrossProject(t *testing.T) {
	s := newTestStore(t)
	a := quickSave(t, s, "alpha", TypeDecision, "src")
	b := quickSave(t, s, "beta", TypeDecision, "tgt")
	_, err := s.CompareAndStore(CompareInput{
		SourceID: a, TargetID: b,
		Relation: RelationRelated, Confidence: 0.5,
	})
	if err == nil || !strings.Contains(err.Error(), "cross-project") {
		t.Errorf("expected cross-project rejection, got %v", err)
	}
}

func TestGetJudgment_NotFoundReturnsNil(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetJudgment("01DOESNOTEXIST")
	if err != nil {
		t.Fatalf("GetJudgment: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
