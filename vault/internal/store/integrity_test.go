package store

import (
	"testing"
	"time"
)

func TestDiagnoseIntegrity_HealthyStoreReportsAllOK(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		quickSave(t, s, "p", TypeDecision, "obs-"+string(rune('a'+i)))
	}

	got, err := s.DiagnoseIntegrity()
	if err != nil {
		t.Fatalf("DiagnoseIntegrity: %v", err)
	}
	if !got.Healthy {
		for _, c := range got.Checks {
			t.Logf("%s = %s — %s", c.Name, c.Status, c.Detail)
		}
		t.Fatal("healthy store should report Healthy=true")
	}
	if len(got.Checks) == 0 {
		t.Fatal("report should include checks")
	}
}

// TestRepairIntegrity_RebuildFTS_IsIdempotent confirms the rebuild operation
// works on a healthy store: plan reports the base-table row count without
// touching anything, and apply runs the FTS5 rebuild without raising. We do
// not assert "drift disappeared" because for external-content FTS5 the row
// count of observations_fts forwards to the base table — drift detection
// happens in DiagnoseIntegrity via FTS5's integrity-check command.
func TestRepairIntegrity_RebuildFTS_IsIdempotent(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		quickSave(t, s, "p", TypeDecision, "obs-"+string(rune('a'+i)))
	}

	plan, err := s.RepairIntegrity(RepairOptions{
		Mode: RepairModePlan, Operations: []string{RepairRebuildFTS},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Actions[0].EstimatedRows != 5 {
		t.Errorf("plan should estimate 5 rows, got %+v", plan.Actions[0])
	}

	applied, err := s.RepairIntegrity(RepairOptions{
		Mode: RepairModeApply, Operations: []string{RepairRebuildFTS},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.Actions[0].AppliedRows != 5 {
		t.Errorf("apply should report 5 rebuilt rows, got %d", applied.Actions[0].AppliedRows)
	}

	// Re-running apply must be safe (idempotent).
	if _, err := s.RepairIntegrity(RepairOptions{
		Mode: RepairModeApply, Operations: []string{RepairRebuildFTS},
	}); err != nil {
		t.Errorf("second apply should be idempotent, got %v", err)
	}

	// Diagnose still healthy after rebuilds.
	if rep, _ := s.DiagnoseIntegrity(); !rep.Healthy {
		for _, c := range rep.Checks {
			t.Logf("%s = %s — %s", c.Name, c.Status, c.Detail)
		}
		t.Error("post-rebuild diagnose should remain healthy")
	}
}

func TestRepairIntegrity_PurgeOrphanRelations(t *testing.T) {
	s := newTestStore(t)
	a := quickSave(t, s, "p", TypeDecision, "a")
	b := quickSave(t, s, "p", TypeDecision, "b")

	// Create a normal relation.
	if _, err := s.db.Exec(
		`INSERT INTO observation_relations (id, source_id, target_id, relation, project)
		 VALUES (?, ?, ?, 'related', 'p')`,
		newID(), a, b,
	); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	// And a relation pointing at a deleted observation. FK constraints are on
	// in production, so we briefly disable them here to stage the orphan —
	// this simulates a row that survived a manual SQL bypass or a corrupt sync.
	if _, err := s.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable fks: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO observation_relations (id, source_id, target_id, relation, project)
		 VALUES (?, ?, ?, 'related', 'p')`,
		newID(), a, "01OBSDOESNOTEXIST",
	); err != nil {
		t.Fatalf("seed orphan relation: %v", err)
	}
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("re-enable fks: %v", err)
	}

	// Plan reports 1 affected.
	plan, _ := s.RepairIntegrity(RepairOptions{
		Mode: RepairModePlan, Operations: []string{RepairPurgeOrphanRels},
	})
	if plan.Actions[0].EstimatedRows != 1 {
		t.Errorf("plan should find 1 orphan, got %d", plan.Actions[0].EstimatedRows)
	}

	// Apply deletes it.
	res, err := s.RepairIntegrity(RepairOptions{
		Mode: RepairModeApply, Operations: []string{RepairPurgeOrphanRels},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Actions[0].AppliedRows != 1 {
		t.Errorf("apply should report 1 deleted, got %d", res.Actions[0].AppliedRows)
	}

	var remaining int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM observation_relations`).Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 valid relation to survive, got %d", remaining)
	}
}

func TestRepairIntegrity_ExpireDeadOutbox(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.db.Exec(
		`INSERT INTO cloud_outbox (id, observation_id, payload, status, attempts)
		 VALUES (?, 'obs-1', x'00', 'pending', ?)`,
		newID(), deadOutboxAttempts+2,
	); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}

	res, err := s.RepairIntegrity(RepairOptions{
		Mode: RepairModeApply, Operations: []string{RepairExpireDeadOutbox},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Actions[0].AppliedRows != 1 {
		t.Errorf("expected 1 row marked failed, got %d", res.Actions[0].AppliedRows)
	}

	var status string
	_ = s.db.QueryRow(`SELECT status FROM cloud_outbox LIMIT 1`).Scan(&status)
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
}

func TestRepairIntegrity_PruneStaleSnapshots_RespectsRetention(t *testing.T) {
	s := newTestStore(t)
	// One ancient snapshot, one recent.
	oldTime := time.Now().UTC().AddDate(0, 0, -90).Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(
		`INSERT INTO config_snapshots (id, scope, file_path, before_json, after_json, created_at)
		 VALUES (?, 'local', '/tmp/x.json', '{}', '{}', ?)`,
		newID(), oldTime,
	); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if _, err := s.SaveConfigSnapshot(ConfigSnapshot{
		Scope: "local", FilePath: "/tmp/x.json",
		BeforeJSON: "{}", AfterJSON: "{}",
	}); err != nil {
		t.Fatalf("seed recent: %v", err)
	}

	res, err := s.RepairIntegrity(RepairOptions{
		Mode:                  RepairModeApply,
		Operations:            []string{RepairPruneStaleSnapshot},
		SnapshotRetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Actions[0].AppliedRows != 1 {
		t.Errorf("expected 1 old snapshot deleted, got %d", res.Actions[0].AppliedRows)
	}

	var remaining int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM config_snapshots`).Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 recent snapshot to survive, got %d", remaining)
	}
}

func TestRepairIntegrity_UnknownMode(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.RepairIntegrity(RepairOptions{Mode: RepairMode("magic")}); err == nil {
		t.Error("unknown mode should error")
	}
}

// findCheck returns a pointer to the first check matching name, or nil.
func findCheck(checks []IntegrityCheck, name string) *IntegrityCheck {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}
