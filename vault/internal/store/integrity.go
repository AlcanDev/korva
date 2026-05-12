package store

import (
	"fmt"
	"strings"
	"time"
)

// IntegrityStatus is the per-check verdict surfaced by DiagnoseIntegrity.
type IntegrityStatus string

const (
	IntegrityOK      IntegrityStatus = "ok"
	IntegrityWarning IntegrityStatus = "warning"
	IntegrityError   IntegrityStatus = "error"
)

// RepairMode controls the side-effects of RepairIntegrity. The three modes
// mirror what an operator expects from a doctor-style tool: "show me what
// you would do", "go through the motions but do not commit", and "fix it".
type RepairMode string

const (
	RepairModePlan   RepairMode = "plan"
	RepairModeDryRun RepairMode = "dry_run"
	RepairModeApply  RepairMode = "apply"
)

// staleOutboxAge is the threshold above which a pending cloud_outbox row is
// flagged as stuck. 24h covers normal Hive jitter while making real stalls
// loud.
const staleOutboxAge = 24 * time.Hour

// deadOutboxAttempts is the retry ceiling beyond which a row is considered
// dead — Doctor flags these so the operator can decide to purge or replay.
const deadOutboxAttempts = 5

// IntegrityCheck is one entry in the diagnostic report.
type IntegrityCheck struct {
	Name          string          `json:"name"`
	Status        IntegrityStatus `json:"status"`
	Detail        string          `json:"detail,omitempty"`
	AffectedCount int             `json:"affected_count"`
	// Repair, when non-empty, is the RepairOperation that RepairIntegrity
	// can execute to clean up the problem this check surfaced.
	Repair string `json:"repair,omitempty"`
}

// IntegrityReport aggregates every check the doctor ran.
type IntegrityReport struct {
	Healthy     bool             `json:"healthy"`
	Checks      []IntegrityCheck `json:"checks"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// RepairOperation names the discrete fixers RepairIntegrity knows how to run.
// Doctor v2 keeps the set small and conservative — each operation must be
// idempotent and safe to re-run.
const (
	RepairRebuildFTS         = "rebuild_fts5"
	RepairPurgeOrphanRels    = "purge_orphan_relations"
	RepairExpireDeadOutbox   = "expire_dead_outbox"
	RepairPruneStaleSnapshot = "prune_stale_snapshots"
)

// RepairAction is what RepairIntegrity says it will do (plan) or did (apply).
type RepairAction struct {
	Operation     string `json:"operation"`
	Description   string `json:"description"`
	EstimatedRows int    `json:"estimated_rows"`
	AppliedRows   int    `json:"applied_rows,omitempty"`
	Error         string `json:"error,omitempty"`
}

// RepairReport summarises a RepairIntegrity call.
type RepairReport struct {
	Mode        RepairMode     `json:"mode"`
	Actions     []RepairAction `json:"actions"`
	GeneratedAt time.Time      `json:"generated_at"`
}

// RepairOptions filters which operations the repairer touches. An empty
// Operations slice means "every repairable check".
type RepairOptions struct {
	Mode       RepairMode
	Operations []string
	// SnapshotRetentionDays caps how old config_snapshots can grow before
	// prune_stale_snapshots is willing to delete them. Zero disables the
	// snapshot retention check entirely (so a fresh install does not lose
	// rollback history).
	SnapshotRetentionDays int
}

// ── DiagnoseIntegrity ───────────────────────────────────────────────────────

// DiagnoseIntegrity runs the read-only health probes Doctor relies on.
//
// Probes added incrementally to keep the report short and actionable:
//   - schema_drift:        new migration columns are present
//   - sqlite_integrity:    PRAGMA integrity_check
//   - fts_observations:    observations_fts row count matches observations
//   - orphan_relations:    relations whose source/target was deleted
//   - stale_outbox:        cloud_outbox rows pending > staleOutboxAge
//   - dead_outbox:         cloud_outbox rows with attempts > deadOutboxAttempts
func (s *Store) DiagnoseIntegrity() (*IntegrityReport, error) {
	report := &IntegrityReport{GeneratedAt: time.Now().UTC()}

	for _, probe := range []func() (IntegrityCheck, error){
		s.checkSchemaDrift,
		s.checkSQLiteIntegrity,
		s.checkFTSObservations,
		s.checkOrphanRelations,
		s.checkStaleOutbox,
		s.checkDeadOutbox,
	} {
		check, err := probe()
		if err != nil {
			return nil, err
		}
		report.Checks = append(report.Checks, check)
	}

	report.Healthy = true
	for _, c := range report.Checks {
		if c.Status != IntegrityOK {
			report.Healthy = false
			break
		}
	}
	return report, nil
}

// checkSchemaDrift confirms the newest migration columns landed. A missing
// column means the binary started before the latest migration ran — which
// shouldn't happen in normal flow but is worth flagging because everything
// else assumes the schema is current.
func (s *Store) checkSchemaDrift() (IntegrityCheck, error) {
	requiredColumns := map[string][]string{
		"observations": {"normalized_hash", "duplicate_count", "last_seen_at"},
	}
	missing := []string{}
	for table, cols := range requiredColumns {
		got, err := s.tableColumns(table)
		if err != nil {
			return IntegrityCheck{}, err
		}
		for _, c := range cols {
			if !contains(got, c) {
				missing = append(missing, table+"."+c)
			}
		}
	}
	if len(missing) > 0 {
		return IntegrityCheck{
			Name:          "schema_drift",
			Status:        IntegrityError,
			AffectedCount: len(missing),
			Detail:        "missing migration columns: " + strings.Join(missing, ", "),
		}, nil
	}
	return IntegrityCheck{Name: "schema_drift", Status: IntegrityOK}, nil
}

// checkSQLiteIntegrity runs `PRAGMA integrity_check`. SQLite returns "ok" on
// a healthy database; anything else is a structural problem we cannot fix
// from Go (the operator likely has to restore from backup).
func (s *Store) checkSQLiteIntegrity() (IntegrityCheck, error) {
	rows, err := s.db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return IntegrityCheck{}, fmt.Errorf("integrity_check: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return IntegrityCheck{}, err
		}
		lines = append(lines, v)
	}
	if len(lines) == 1 && lines[0] == "ok" {
		return IntegrityCheck{Name: "sqlite_integrity", Status: IntegrityOK}, nil
	}
	return IntegrityCheck{
		Name:          "sqlite_integrity",
		Status:        IntegrityError,
		AffectedCount: len(lines),
		Detail:        strings.Join(lines, "; "),
	}, nil
}

// checkFTSObservations runs FTS5's built-in `integrity-check` command, which
// verifies the segments / shadow tables of the index without comparing to
// the base table — that comparison is unreliable for external-content FTS5
// because COUNT(*) on the virtual table forwards to the base table. The
// command raises an error when the FTS index is structurally corrupted,
// and the rebuild_fts5 repair restores it.
func (s *Store) checkFTSObservations() (IntegrityCheck, error) {
	_, err := s.db.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('integrity-check')`)
	if err == nil {
		return IntegrityCheck{Name: "fts_observations", Status: IntegrityOK}, nil
	}
	return IntegrityCheck{
		Name:   "fts_observations",
		Status: IntegrityWarning,
		Detail: fmt.Sprintf("FTS5 integrity-check failed: %v", err),
		Repair: RepairRebuildFTS,
	}, nil
}

// checkOrphanRelations finds observation_relations rows whose source or target
// observation no longer exists. Deletes from the base table are supposed to
// cascade via FK but if a manual SQL bypass left orphans, surface them.
func (s *Store) checkOrphanRelations() (IntegrityCheck, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM observation_relations r
		 WHERE NOT EXISTS (SELECT 1 FROM observations o WHERE o.id = r.source_id)
		    OR NOT EXISTS (SELECT 1 FROM observations o WHERE o.id = r.target_id)`,
	).Scan(&n)
	if err != nil {
		return IntegrityCheck{}, err
	}
	if n == 0 {
		return IntegrityCheck{Name: "orphan_relations", Status: IntegrityOK}, nil
	}
	return IntegrityCheck{
		Name:          "orphan_relations",
		Status:        IntegrityWarning,
		AffectedCount: n,
		Detail:        fmt.Sprintf("%d relation(s) reference a deleted observation", n),
		Repair:        RepairPurgeOrphanRels,
	}, nil
}

// checkStaleOutbox flags cloud_outbox rows that have been pending longer than
// staleOutboxAge — a strong signal Hive sync is jammed.
func (s *Store) checkStaleOutbox() (IntegrityCheck, error) {
	cutoff := time.Now().UTC().Add(-staleOutboxAge).Format(time.RFC3339)
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM cloud_outbox
		  WHERE status = 'pending' AND created_at < ?`, cutoff,
	).Scan(&n)
	if err != nil {
		return IntegrityCheck{}, err
	}
	if n == 0 {
		return IntegrityCheck{Name: "stale_outbox", Status: IntegrityOK}, nil
	}
	return IntegrityCheck{
		Name:          "stale_outbox",
		Status:        IntegrityWarning,
		AffectedCount: n,
		Detail:        fmt.Sprintf("%d pending outbox row(s) older than %s", n, staleOutboxAge),
	}, nil
}

// checkDeadOutbox flags rows that exceeded the retry ceiling. Dead rows are
// not auto-purged because operators may want to inspect what failed; the
// expire_dead_outbox repair clears them once they confirm.
func (s *Store) checkDeadOutbox() (IntegrityCheck, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM cloud_outbox
		  WHERE attempts > ? AND status != 'sent'`,
		deadOutboxAttempts,
	).Scan(&n)
	if err != nil {
		return IntegrityCheck{}, err
	}
	if n == 0 {
		return IntegrityCheck{Name: "dead_outbox", Status: IntegrityOK}, nil
	}
	return IntegrityCheck{
		Name:          "dead_outbox",
		Status:        IntegrityWarning,
		AffectedCount: n,
		Detail:        fmt.Sprintf("%d outbox row(s) exceeded %d retry attempts", n, deadOutboxAttempts),
		Repair:        RepairExpireDeadOutbox,
	}, nil
}

// ── RepairIntegrity ─────────────────────────────────────────────────────────

// RepairIntegrity executes the repair operations matching `opts.Operations`,
// or all known operations when the list is empty. Mode controls side-effects:
//   - plan:    describe what would be done; do not read or write rows
//   - dry_run: count the rows each operation would touch; do not write
//   - apply:   execute and report rows changed
func (s *Store) RepairIntegrity(opts RepairOptions) (*RepairReport, error) {
	if opts.Mode == "" {
		opts.Mode = RepairModePlan
	}
	switch opts.Mode {
	case RepairModePlan, RepairModeDryRun, RepairModeApply:
	default:
		return nil, fmt.Errorf("unknown repair mode %q", opts.Mode)
	}

	wanted := opts.Operations
	if len(wanted) == 0 {
		wanted = []string{
			RepairRebuildFTS, RepairPurgeOrphanRels, RepairExpireDeadOutbox,
		}
		// Snapshot prune is opt-in to avoid surprise data loss on fresh installs.
		if opts.SnapshotRetentionDays > 0 {
			wanted = append(wanted, RepairPruneStaleSnapshot)
		}
	}

	report := &RepairReport{Mode: opts.Mode, GeneratedAt: time.Now().UTC()}
	for _, op := range wanted {
		action, err := s.runRepair(op, opts)
		if err != nil {
			// Surface the per-operation error in the action; keep going so the
			// caller sees the whole story instead of bailing on the first hiccup.
			action = RepairAction{Operation: op, Error: err.Error()}
		}
		report.Actions = append(report.Actions, action)
	}
	return report, nil
}

func (s *Store) runRepair(op string, opts RepairOptions) (RepairAction, error) {
	switch op {
	case RepairRebuildFTS:
		return s.repairRebuildFTS(opts.Mode)
	case RepairPurgeOrphanRels:
		return s.repairPurgeOrphanRels(opts.Mode)
	case RepairExpireDeadOutbox:
		return s.repairExpireDeadOutbox(opts.Mode)
	case RepairPruneStaleSnapshot:
		return s.repairPruneStaleSnapshots(opts)
	}
	return RepairAction{}, fmt.Errorf("unknown repair operation %q", op)
}

func (s *Store) repairRebuildFTS(mode RepairMode) (RepairAction, error) {
	action := RepairAction{
		Operation:   RepairRebuildFTS,
		Description: "Rebuild observations_fts from the base table",
	}
	var obsCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&obsCount); err != nil {
		return action, err
	}
	action.EstimatedRows = obsCount
	if mode == RepairModeApply {
		if _, err := s.db.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('rebuild')`); err != nil {
			return action, err
		}
		action.AppliedRows = obsCount
	}
	return action, nil
}

func (s *Store) repairPurgeOrphanRels(mode RepairMode) (RepairAction, error) {
	action := RepairAction{
		Operation:   RepairPurgeOrphanRels,
		Description: "Delete observation_relations whose endpoints no longer exist",
	}
	const cond = `WHERE NOT EXISTS (SELECT 1 FROM observations o WHERE o.id = r.source_id)
                  OR NOT EXISTS (SELECT 1 FROM observations o WHERE o.id = r.target_id)`
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observation_relations r `+cond).Scan(&n); err != nil {
		return action, err
	}
	action.EstimatedRows = n
	if mode == RepairModeApply && n > 0 {
		res, err := s.db.Exec(`DELETE FROM observation_relations AS r ` + cond)
		if err != nil {
			return action, err
		}
		applied, _ := res.RowsAffected()
		action.AppliedRows = int(applied)
	}
	return action, nil
}

func (s *Store) repairExpireDeadOutbox(mode RepairMode) (RepairAction, error) {
	action := RepairAction{
		Operation:   RepairExpireDeadOutbox,
		Description: fmt.Sprintf("Mark cloud_outbox rows with attempts > %d as failed", deadOutboxAttempts),
	}
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM cloud_outbox WHERE attempts > ? AND status != 'sent' AND status != 'failed'`,
		deadOutboxAttempts,
	).Scan(&n); err != nil {
		return action, err
	}
	action.EstimatedRows = n
	if mode == RepairModeApply && n > 0 {
		res, err := s.db.Exec(
			`UPDATE cloud_outbox SET status = 'failed', updated_at = datetime('now')
			  WHERE attempts > ? AND status != 'sent' AND status != 'failed'`,
			deadOutboxAttempts,
		)
		if err != nil {
			return action, err
		}
		applied, _ := res.RowsAffected()
		action.AppliedRows = int(applied)
	}
	return action, nil
}

func (s *Store) repairPruneStaleSnapshots(opts RepairOptions) (RepairAction, error) {
	action := RepairAction{
		Operation: RepairPruneStaleSnapshot,
		Description: fmt.Sprintf("Delete config_snapshots older than %d days",
			opts.SnapshotRetentionDays),
	}
	if opts.SnapshotRetentionDays <= 0 {
		// Caller passed Operations explicitly but no retention — return a
		// no-op action so the report stays honest.
		action.Description = "snapshot retention not configured — skipping"
		return action, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -opts.SnapshotRetentionDays).Format("2006-01-02 15:04:05")
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM config_snapshots WHERE created_at < ?`, cutoff,
	).Scan(&n); err != nil {
		return action, err
	}
	action.EstimatedRows = n
	if opts.Mode == RepairModeApply && n > 0 {
		res, err := s.db.Exec(`DELETE FROM config_snapshots WHERE created_at < ?`, cutoff)
		if err != nil {
			return action, err
		}
		applied, _ := res.RowsAffected()
		action.AppliedRows = int(applied)
	}
	return action, nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (s *Store) tableColumns(table string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
