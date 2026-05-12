package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Phase 2 — Deferred apply queue.
//
// When a pulled Hive mutation cannot be applied locally (e.g. a relation
// references an observation we have not received yet), the caller stores the
// payload here so the pull cursor can advance without stalling. A background
// replay re-attempts each row; rows that exceed the retry ceiling are marked
// 'dead' and surfaced by Doctor for the operator to inspect.

// deadDeferredAttempts is the retry ceiling: once exceeded, the row stops
// being retried automatically. Operators can still replay 'dead' rows by hand.
const deadDeferredAttempts = 5

// DeferredApplyStatus values mirror the apply_status column.
const (
	DeferredStatusDeferred = "deferred"
	DeferredStatusApplied  = "applied"
	DeferredStatusDead     = "dead"
)

// DeferredApplyEntity tags the payload kind. Kept as plain strings so the
// table stays human-debuggable.
const (
	DeferredEntityObservation = "observation"
	DeferredEntityRelation    = "relation"
)

// DeferredApply is a row from cloud_apply_deferred.
type DeferredApply struct {
	SyncID          string     `json:"sync_id"`
	Entity          string     `json:"entity"`
	Payload         []byte     `json:"payload"`
	ApplyStatus     string     `json:"apply_status"`
	RetryCount      int        `json:"retry_count"`
	LastError       string     `json:"last_error,omitempty"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	LastAttemptedAt *time.Time `json:"last_attempted_at,omitempty"`
}

// DeferApply records a payload that could not be applied immediately. The
// upsert keeps existing retry counters intact while bumping the LastError for
// fresh visibility into why the row is stuck.
func (s *Store) DeferApply(syncID, entity string, payload []byte, lastError string) error {
	if syncID == "" {
		return fmt.Errorf("sync_id is required")
	}
	switch entity {
	case DeferredEntityObservation, DeferredEntityRelation:
	default:
		return fmt.Errorf("unknown deferred entity %q", entity)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO cloud_apply_deferred
		  (sync_id, entity, payload, apply_status, retry_count, last_error, first_seen_at, last_attempted_at)
		VALUES (?, ?, ?, 'deferred', 0, ?, ?, NULL)
		ON CONFLICT(sync_id) DO UPDATE SET
		    last_error        = excluded.last_error,
		    last_attempted_at = ?`,
		syncID, entity, payload, lastError, now, now,
	)
	if err != nil {
		return fmt.Errorf("deferring apply for %s: %w", syncID, err)
	}
	return nil
}

// ListDeferred returns rows matching `status` (empty = all statuses), oldest
// first so retry order is FIFO.
func (s *Store) ListDeferred(status string, limit int) ([]DeferredApply, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{}
	clause := ""
	if status != "" {
		clause = ` WHERE apply_status = ?`
		args = append(args, status)
	}
	args = append(args, limit)

	rows, err := s.db.Query(`
		SELECT sync_id, entity, payload, apply_status, retry_count,
		       COALESCE(last_error,''), first_seen_at, last_attempted_at
		  FROM cloud_apply_deferred`+clause+`
		 ORDER BY first_seen_at ASC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing deferred: %w", err)
	}
	defer rows.Close()

	var out []DeferredApply
	for rows.Next() {
		row, err := scanDeferredApply(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	return out, rows.Err()
}

// GetDeferred returns a single row by sync_id, or nil when missing.
func (s *Store) GetDeferred(syncID string) (*DeferredApply, error) {
	rows, err := s.db.Query(`
		SELECT sync_id, entity, payload, apply_status, retry_count,
		       COALESCE(last_error,''), first_seen_at, last_attempted_at
		  FROM cloud_apply_deferred
		 WHERE sync_id = ?`, syncID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanDeferredApply(rows)
}

// MarkDeferredApplied is called by the replay path when the dependent state
// has arrived and the payload was successfully applied locally. The row stays
// in the table for audit so operators can confirm "we did eventually catch up".
func (s *Store) MarkDeferredApplied(syncID string) error {
	res, err := s.db.Exec(
		`UPDATE cloud_apply_deferred
		    SET apply_status = 'applied', last_attempted_at = ?
		  WHERE sync_id = ?`,
		time.Now().UTC().Format(time.RFC3339), syncID,
	)
	if err != nil {
		return fmt.Errorf("marking applied: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no deferred row with sync_id %q", syncID)
	}
	return nil
}

// IncrementDeferredRetry bumps retry_count, records last_error, and flips
// apply_status to 'dead' when the count exceeds deadDeferredAttempts. The
// status transition is what stops the replay loop from spinning forever on
// a payload that cannot possibly apply.
func (s *Store) IncrementDeferredRetry(syncID, lastError string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// Single round-trip: bump count, set last_error/last_attempted_at, and
	// flip to 'dead' atomically when the new count crosses the ceiling.
	res, err := s.db.Exec(`
		UPDATE cloud_apply_deferred
		   SET retry_count       = retry_count + 1,
		       last_error        = ?,
		       last_attempted_at = ?,
		       apply_status      = CASE
		           WHEN retry_count + 1 > ? THEN 'dead'
		           ELSE apply_status
		       END
		 WHERE sync_id = ?`,
		lastError, now, deadDeferredAttempts, syncID,
	)
	if err != nil {
		return fmt.Errorf("bumping retry: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no deferred row with sync_id %q", syncID)
	}
	return nil
}

// DeleteDeferred removes a row outright (used by manual operator workflows
// once a 'dead' payload is no longer worth keeping around for inspection).
func (s *Store) DeleteDeferred(syncID string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM cloud_apply_deferred WHERE sync_id = ?`, syncID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanDeferredApply(rows *sql.Rows) (*DeferredApply, error) {
	var d DeferredApply
	var firstSeen string
	var lastAttempted sql.NullString
	if err := rows.Scan(
		&d.SyncID, &d.Entity, &d.Payload, &d.ApplyStatus, &d.RetryCount,
		&d.LastError, &firstSeen, &lastAttempted,
	); err != nil {
		return nil, err
	}
	d.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeen)
	if lastAttempted.Valid && lastAttempted.String != "" {
		if t, err := time.Parse(time.RFC3339, lastAttempted.String); err == nil {
			d.LastAttemptedAt = &t
		}
	}
	return &d, nil
}
