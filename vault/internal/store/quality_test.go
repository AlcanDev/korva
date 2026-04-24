package store

import (
	"testing"
	"time"
)

// ── QualityCheckpoint store methods ──────────────────────────────────────────

func TestSaveQualityCheckpoint_Basic(t *testing.T) {
	s := newTestStore(t)

	cp := QualityCheckpoint{
		Project:  "korva",
		Phase:    "apply",
		Language: "go",
		Status:   QualityPass,
		Score:    85,
		Findings: []QualityFinding{
			{Rule: "APP-001", Status: "pass", Notes: "all tests written"},
			{Rule: "APP-002", Status: "pass", Notes: "error paths covered"},
		},
		GatePassed: true,
	}

	id, err := s.SaveQualityCheckpoint(cp)
	if err != nil {
		t.Fatalf("SaveQualityCheckpoint: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestSaveQualityCheckpoint_GeneratesID(t *testing.T) {
	s := newTestStore(t)
	cp := QualityCheckpoint{Project: "p", Phase: "verify", Status: QualityFail, Score: 40}
	id, err := s.SaveQualityCheckpoint(cp)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if id == "" {
		t.Fatal("want non-empty id")
	}
}

func TestSaveQualityCheckpoint_PreservesProvidedID(t *testing.T) {
	s := newTestStore(t)
	cp := QualityCheckpoint{
		ID:      "test-id-001",
		Project: "p",
		Phase:   "apply",
		Status:  QualityPartial,
		Score:   60,
	}
	id, err := s.SaveQualityCheckpoint(cp)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if id != "test-id-001" {
		t.Errorf("want id=test-id-001, got %q", id)
	}
}

// ── GetQualityCheckpoints ─────────────────────────────────────────────────────

func TestGetQualityCheckpoints_Empty(t *testing.T) {
	s := newTestStore(t)
	cps, err := s.GetQualityCheckpoints("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetQualityCheckpoints: %v", err)
	}
	if len(cps) != 0 {
		t.Fatalf("want 0, got %d", len(cps))
	}
}

func TestGetQualityCheckpoints_ReturnsInOrder(t *testing.T) {
	s := newTestStore(t)

	// Use explicit timestamps spaced 1 second apart to ensure deterministic ordering.
	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		cp := QualityCheckpoint{
			Project:   "proj",
			Phase:     "apply",
			Status:    QualityPartial,
			Score:     50 + i*10,
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if _, err := s.SaveQualityCheckpoint(cp); err != nil {
			t.Fatalf("save[%d]: %v", i, err)
		}
	}

	cps, err := s.GetQualityCheckpoints("proj", 10)
	if err != nil {
		t.Fatalf("GetQualityCheckpoints: %v", err)
	}
	if len(cps) != 3 {
		t.Fatalf("want 3, got %d", len(cps))
	}
	// Most recent (score=70) should come first.
	if cps[0].Score != 70 {
		t.Errorf("want most-recent score=70 first, got %d", cps[0].Score)
	}
}

func TestGetQualityCheckpoints_LimitRespected(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		s.SaveQualityCheckpoint(QualityCheckpoint{Project: "p", Phase: "apply", Status: QualityPass, Score: 80}) //nolint:errcheck
	}
	cps, err := s.GetQualityCheckpoints("p", 2)
	if err != nil {
		t.Fatalf("GetQualityCheckpoints: %v", err)
	}
	if len(cps) != 2 {
		t.Fatalf("want 2 (limit), got %d", len(cps))
	}
}

// ── GetLatestCheckpointForPhase ───────────────────────────────────────────────

func TestGetLatestCheckpointForPhase_NoneExists(t *testing.T) {
	s := newTestStore(t)
	cp, err := s.GetLatestCheckpointForPhase("proj", "apply")
	if err != nil {
		t.Fatalf("GetLatestCheckpointForPhase: %v", err)
	}
	if cp != nil {
		t.Fatal("want nil when no checkpoints exist")
	}
}

func TestGetLatestCheckpointForPhase_ReturnsLatest(t *testing.T) {
	s := newTestStore(t)

	// Save an older failing checkpoint.
	old := QualityCheckpoint{Project: "p", Phase: "apply", Status: QualityFail, Score: 40}
	old.CreatedAt = time.Now().Add(-1 * time.Hour)
	s.SaveQualityCheckpoint(old) //nolint:errcheck

	// Save a newer passing checkpoint.
	newer := QualityCheckpoint{Project: "p", Phase: "apply", Status: QualityPass, Score: 85, GatePassed: true}
	s.SaveQualityCheckpoint(newer) //nolint:errcheck

	cp, err := s.GetLatestCheckpointForPhase("p", "apply")
	if err != nil {
		t.Fatalf("GetLatestCheckpointForPhase: %v", err)
	}
	if cp == nil {
		t.Fatal("want checkpoint, got nil")
	}
	if !cp.GatePassed {
		t.Error("want latest (passing) checkpoint, got the old failing one")
	}
	if cp.Score != 85 {
		t.Errorf("want score=85, got %d", cp.Score)
	}
}

func TestGetLatestCheckpointForPhase_IsolatedByPhase(t *testing.T) {
	s := newTestStore(t)
	s.SaveQualityCheckpoint(QualityCheckpoint{Project: "p", Phase: "apply", Status: QualityPass, Score: 90, GatePassed: true})  //nolint:errcheck
	s.SaveQualityCheckpoint(QualityCheckpoint{Project: "p", Phase: "verify", Status: QualityFail, Score: 30, GatePassed: false}) //nolint:errcheck

	cp, err := s.GetLatestCheckpointForPhase("p", "apply")
	if err != nil {
		t.Fatalf("GetLatestCheckpointForPhase: %v", err)
	}
	if cp == nil || !cp.GatePassed {
		t.Error("apply checkpoint should be gate_passed=true")
	}

	cp2, err := s.GetLatestCheckpointForPhase("p", "verify")
	if err != nil {
		t.Fatalf("GetLatestCheckpointForPhase verify: %v", err)
	}
	if cp2 == nil || cp2.GatePassed {
		t.Error("verify checkpoint should be gate_passed=false")
	}
}

// ── GetProjectQualityScore ────────────────────────────────────────────────────

func TestGetProjectQualityScore_NoCheckpoints(t *testing.T) {
	s := newTestStore(t)
	ps, err := s.GetProjectQualityScore("empty-project")
	if err != nil {
		t.Fatalf("GetProjectQualityScore: %v", err)
	}
	if ps.TotalChecks != 0 {
		t.Errorf("want 0 checks, got %d", ps.TotalChecks)
	}
	if ps.LatestScore != 0 {
		t.Errorf("want 0 score, got %d", ps.LatestScore)
	}
}

func TestGetProjectQualityScore_Aggregates(t *testing.T) {
	s := newTestStore(t)

	checks := []struct {
		score      int
		gatePassed bool
	}{
		{80, true},
		{60, false},
		{90, true},
	}
	for _, c := range checks {
		s.SaveQualityCheckpoint(QualityCheckpoint{ //nolint:errcheck
			Project: "p", Phase: "apply", Status: QualityPass,
			Score: c.score, GatePassed: c.gatePassed,
		})
	}

	ps, err := s.GetProjectQualityScore("p")
	if err != nil {
		t.Fatalf("GetProjectQualityScore: %v", err)
	}
	if ps.TotalChecks != 3 {
		t.Errorf("want 3 checks, got %d", ps.TotalChecks)
	}
	if ps.PassedGates != 2 {
		t.Errorf("want 2 passed gates, got %d", ps.PassedGates)
	}
	wantAvg := (80 + 60 + 90) / 3
	if ps.AverageScore != wantAvg {
		t.Errorf("want avg=%d, got %d", wantAvg, ps.AverageScore)
	}
}

// ── GetQualityChecklist (quality_rules.go) ────────────────────────────────────

func TestGetQualityChecklist_ApplyGeneral(t *testing.T) {
	cl := GetQualityChecklist("apply", "")
	if cl.Phase != "apply" {
		t.Errorf("phase = %q, want apply", cl.Phase)
	}
	if len(cl.Criteria) == 0 {
		t.Fatal("apply phase should have criteria")
	}
	// APP-001 (unit tests required) must be present.
	found := false
	for _, c := range cl.Criteria {
		if c.ID == "APP-001" {
			found = true
			if !c.Required {
				t.Error("APP-001 must be Required=true")
			}
		}
	}
	if !found {
		t.Error("APP-001 not found in apply checklist")
	}
}

func TestGetQualityChecklist_ApplyGo_MergesLangCriteria(t *testing.T) {
	cl := GetQualityChecklist("apply", "go")
	if cl.Language != "go" {
		t.Errorf("language = %q, want go", cl.Language)
	}
	// Must have both general (APP-*) and Go-specific (GO-APP-*) criteria.
	hasGeneral, hasGoSpecific := false, false
	for _, c := range cl.Criteria {
		if c.ID == "APP-001" {
			hasGeneral = true
		}
		if c.ID == "GO-APP-001" {
			hasGoSpecific = true
		}
	}
	if !hasGeneral {
		t.Error("expected general APP-001 criterion in go/apply checklist")
	}
	if !hasGoSpecific {
		t.Error("expected Go-specific GO-APP-001 criterion in go/apply checklist")
	}
}

func TestGetQualityChecklist_VerifyE2ERequired(t *testing.T) {
	cl := GetQualityChecklist("verify", "")
	if !cl.E2ERequired {
		t.Error("verify phase must have E2ERequired=true")
	}
}

func TestGetQualityChecklist_UnknownPhase(t *testing.T) {
	cl := GetQualityChecklist("unknown-phase", "")
	if cl.Phase != "unknown-phase" {
		t.Errorf("phase = %q, want unknown-phase", cl.Phase)
	}
	// Should return empty criteria, not panic.
	if cl.Criteria == nil {
		t.Error("criteria should be nil-safe (not panic)")
	}
}

// ── IsGatedTransition ─────────────────────────────────────────────────────────

func TestIsGatedTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"apply", "verify", true},
		{"verify", "archive", true},
		{"explore", "propose", false},
		{"spec", "design", false},
		{"apply", "archive", false}, // skipping verify — still gated on apply→verify not apply→archive
		{"verify", "onboard", false},
	}
	for _, tc := range cases {
		got := IsGatedTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("IsGatedTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

// ── Findings round-trip ───────────────────────────────────────────────────────

func TestSaveQualityCheckpoint_FindingsRoundtrip(t *testing.T) {
	s := newTestStore(t)

	findings := []QualityFinding{
		{Rule: "APP-001", Status: "pass", Notes: "100% covered"},
		{Rule: "APP-007", Status: "fail", Notes: "found hardcoded API key in fixture"},
	}
	cp := QualityCheckpoint{
		Project:  "p",
		Phase:    "apply",
		Status:   QualityFail,
		Score:    55,
		Findings: findings,
	}
	id, err := s.SaveQualityCheckpoint(cp)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	cps, err := s.GetQualityCheckpoints("p", 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(cps) == 0 {
		t.Fatal("want 1 checkpoint, got 0")
	}
	got := cps[0]
	if got.ID != id {
		t.Errorf("id mismatch: got %q want %q", got.ID, id)
	}
	if len(got.Findings) != 2 {
		t.Fatalf("want 2 findings, got %d", len(got.Findings))
	}
	if got.Findings[0].Rule != "APP-001" {
		t.Errorf("findings[0].Rule = %q, want APP-001", got.Findings[0].Rule)
	}
	if got.Findings[1].Status != "fail" {
		t.Errorf("findings[1].Status = %q, want fail", got.Findings[1].Status)
	}
}
