package store

import (
	"testing"
)

// Phase 20.A — direct coverage for code_health.go's scoring helpers.
// The composite formula (50% QA score + 30% gate pass rate + 20%
// pattern signal) drives the admin dashboard; getting any of those
// weightings wrong silently misranks projects.

func insertQACheckpoint(t *testing.T, s *Store, project string, score int, gatePassed int) {
	t.Helper()
	_, err := s.db.Exec(
		`INSERT INTO quality_checkpoints (id, project, phase, score, gate_passed)
		 VALUES (?, ?, ?, ?, ?)`,
		newID(), project, "implement", score, gatePassed,
	)
	if err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}
}

// insertQACheckpointAt is the trend-test variant: lets the caller
// pin the created_at timestamp so the rows have a deterministic
// DESC order. SQLite's `datetime('now')` truncates to seconds, so
// rapid inserts tie and the trend logic reads them in undefined
// order.
func insertQACheckpointAt(t *testing.T, s *Store, project string, score int, gatePassed int, createdAt string) {
	t.Helper()
	_, err := s.db.Exec(
		`INSERT INTO quality_checkpoints (id, project, phase, score, gate_passed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), project, "implement", score, gatePassed, createdAt,
	)
	if err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}
}

func TestScoreGrade_Boundaries(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, "A"}, {90, "A"},
		{89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"},
		{69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"}, {-10, "F"},
	}
	for _, tc := range cases {
		if got := scoreGrade(tc.score); got != tc.want {
			t.Errorf("scoreGrade(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestAvg_HandlesEmptySlice(t *testing.T) {
	if got := avg(nil); got != 0 {
		t.Errorf("avg(nil) = %f, want 0", got)
	}
	if got := avg([]float64{}); got != 0 {
		t.Errorf("avg(empty) = %f, want 0", got)
	}
}

func TestAvg_BasicArithmetic(t *testing.T) {
	if got := avg([]float64{10, 20, 30}); got != 20 {
		t.Errorf("avg = %f, want 20", got)
	}
}

func TestCodeHealthSummary_EmptyStoreReturnsEmptySlice(t *testing.T) {
	s := newCallsStore(t)
	got, err := s.CodeHealthSummary()
	if err != nil {
		t.Fatalf("CodeHealthSummary: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("rows = %d, want 0", len(got))
	}
	// The contract is "empty slice, not nil" — so JSON encoders
	// emit `[]` instead of `null`.
	if got == nil {
		t.Error("CodeHealthSummary should return an empty slice, not nil")
	}
}

func TestCodeHealthSummary_AggregatesPerProject(t *testing.T) {
	s := newCallsStore(t)
	// Project alpha: two checkpoints, scores 80 and 100, both pass.
	insertQACheckpoint(t, s, "alpha", 80, 1)
	insertQACheckpoint(t, s, "alpha", 100, 1)
	// Project beta: one checkpoint, score 50, gate failed.
	insertQACheckpoint(t, s, "beta", 50, 0)

	got, err := s.CodeHealthSummary()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	// Ordered by avg_score DESC: alpha first.
	if got[0].Project != "alpha" {
		t.Errorf("rows[0].Project = %q, want alpha", got[0].Project)
	}
	if got[0].AvgQAScore != 90 {
		t.Errorf("alpha avg = %f, want 90", got[0].AvgQAScore)
	}
	if got[0].GatePassRate != 1.0 {
		t.Errorf("alpha pass rate = %f, want 1.0", got[0].GatePassRate)
	}
	if got[0].RecentCheckpoints != 2 {
		t.Errorf("alpha checkpoints = %d, want 2", got[0].RecentCheckpoints)
	}
	if got[1].Project != "beta" || got[1].AvgQAScore != 50 || got[1].GatePassRate != 0 {
		t.Errorf("beta wrong: %+v", got[1])
	}
}

func TestCodeHealthSummary_CompositeScoreGoodProject(t *testing.T) {
	s := newCallsStore(t)
	// All-green project: 100 score, gate passed, no bugfixes, 1 pattern.
	insertQACheckpoint(t, s, "alpha", 100, 1)
	if _, err := s.Save(Observation{
		Project: "alpha", Title: "good pattern", Content: "x",
		Type: TypePattern, Author: "test",
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := s.CodeHealthSummary()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	// 0.5*100 + 0.3*100 + 0.2*100 (1/(1+0)) = 100.
	if got[0].Score != 100 {
		t.Errorf("score = %d, want 100", got[0].Score)
	}
	if got[0].Grade != "A" {
		t.Errorf("grade = %q, want A", got[0].Grade)
	}
	if got[0].PatternCount != 1 {
		t.Errorf("PatternCount = %d, want 1", got[0].PatternCount)
	}
}

func TestCodeHealthSummary_CompositeScoreBugfixHeavy(t *testing.T) {
	s := newCallsStore(t)
	// Score 100, gate passed, 5 bugfixes 1 pattern.
	insertQACheckpoint(t, s, "alpha", 100, 1)
	for i := 0; i < 5; i++ {
		_, _ = s.Save(Observation{
			Project: "alpha", Title: "bug-" + string(rune('a'+i)),
			Content: "broken-" + string(rune('a'+i)),
			Type:    TypeBugfix, Author: "test",
		})
	}
	_, _ = s.Save(Observation{
		Project: "alpha", Title: "good", Content: "x",
		Type: TypePattern, Author: "test",
	})

	got, _ := s.CodeHealthSummary()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	// Pattern signal = 1 / (1 + 5) ≈ 0.167.
	// Score = 0.5*100 + 0.3*100 + 0.2*100*0.167 ≈ 83.3 → 83.
	if got[0].Score < 80 || got[0].Score > 86 {
		t.Errorf("score = %d, want ~83 (pattern signal dragged down by bugfixes)", got[0].Score)
	}
	if got[0].BugfixCount != 5 || got[0].PatternCount != 1 {
		t.Errorf("counts wrong: bugfix=%d pattern=%d", got[0].BugfixCount, got[0].PatternCount)
	}
}

func TestCodeHealthSummary_TrendDefaultsStableWithoutEnoughData(t *testing.T) {
	s := newCallsStore(t)
	// Only 2 checkpoints — under the 4-row threshold for trend
	// computation. computeTrend should return "stable".
	insertQACheckpoint(t, s, "alpha", 80, 1)
	insertQACheckpoint(t, s, "alpha", 90, 1)

	got, _ := s.CodeHealthSummary()
	if len(got) != 1 {
		t.Fatalf("rows = %d", len(got))
	}
	if got[0].Trend != "stable" {
		t.Errorf("trend = %q, want stable (insufficient data)", got[0].Trend)
	}
}

// timestamps used by the trend tests — distinct seconds so SQLite's
// ORDER BY DESC returns them in well-defined order.
var trendStamps = [6]string{
	"2026-05-14 10:00:00", // oldest
	"2026-05-14 10:01:00",
	"2026-05-14 10:02:00",
	"2026-05-14 10:03:00",
	"2026-05-14 10:04:00",
	"2026-05-14 10:05:00", // newest
}

func TestComputeTrend_DetectsImprovement(t *testing.T) {
	s := newCallsStore(t)
	// Older 3 (oldest first): avg 50. Recent 3 (newest first): avg 90.
	// Difference > 5 → improving.
	for i, sc := range []int{50, 50, 50, 90, 90, 90} {
		insertQACheckpointAt(t, s, "alpha", sc, 1, trendStamps[i])
	}
	if got := computeTrend(s, "alpha"); got != "improving" {
		t.Errorf("trend = %q, want improving", got)
	}
}

func TestComputeTrend_DetectsDecline(t *testing.T) {
	s := newCallsStore(t)
	for i, sc := range []int{90, 90, 90, 40, 40, 40} {
		insertQACheckpointAt(t, s, "alpha", sc, 1, trendStamps[i])
	}
	if got := computeTrend(s, "alpha"); got != "declining" {
		t.Errorf("trend = %q, want declining", got)
	}
}

func TestComputeTrend_StableWhenWithinTolerance(t *testing.T) {
	s := newCallsStore(t)
	// Older 3: avg 80. Recent 3: avg 82. Difference 2 < 5 → stable.
	for i, sc := range []int{80, 80, 80, 82, 82, 82} {
		insertQACheckpointAt(t, s, "alpha", sc, 1, trendStamps[i])
	}
	if got := computeTrend(s, "alpha"); got != "stable" {
		t.Errorf("trend = %q, want stable", got)
	}
}
