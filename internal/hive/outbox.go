package hive

import (
	cryptorand "crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Outbox is the persistent queue of observations awaiting Hive sync.
type Outbox struct {
	db *sql.DB
}

// NewOutbox wraps the given sql.DB. The cloud_outbox table must already exist
// (created by internal/db migrations).
func NewOutbox(db *sql.DB) *Outbox {
	return &Outbox{db: db}
}

// EnqueueForProject stores an observation payload for future Hive delivery,
// but only if the project's sync is enabled. Returns (false, nil) when the
// project is paused — callers should log the skip at debug level.
func (o *Outbox) EnqueueForProject(observationID, project string, payload []byte) (bool, error) {
	enabled, err := o.IsProjectSyncEnabled(project)
	if err != nil {
		return false, err
	}
	if !enabled {
		return false, nil
	}
	return true, o.Enqueue(observationID, payload)
}

// Enqueue stores an observation payload for future Hive delivery.
// Errors are surfaced but the caller is expected to log-and-continue —
// Hive sync must never block the local Save flow.
func (o *Outbox) Enqueue(observationID string, payload []byte) error {
	if observationID == "" {
		return errors.New("hive outbox: empty observation id")
	}
	id := newOutboxID()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := o.db.Exec(`
		INSERT INTO cloud_outbox (id, observation_id, payload, status, next_attempt_at, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?)`,
		id, observationID, payload, now, now, now,
	)
	if err != nil {
		return fmt.Errorf("hive outbox enqueue: %w", err)
	}
	return nil
}

// NextBatch returns up to limit pending rows whose next_attempt_at has elapsed.
func (o *Outbox) NextBatch(limit int) ([]Row, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := o.db.Query(`
		SELECT id, observation_id, payload, attempts, next_attempt_at, created_at
		FROM cloud_outbox
		WHERE status = 'pending' AND next_attempt_at <= ?
		ORDER BY created_at ASC
		LIMIT ?`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("hive outbox query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Row
	for rows.Next() {
		var r Row
		var nextAt, createdAt string
		if err := rows.Scan(&r.ID, &r.ObservationID, &r.Payload, &r.Attempts, &nextAt, &createdAt); err != nil {
			return nil, err
		}
		r.NextAttemptAt, _ = time.Parse(time.RFC3339, nextAt)
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkSent transitions a row to status='sent'.
func (o *Outbox) MarkSent(id string) error {
	return o.update(id, StatusSent, "", time.Time{}, false)
}

// MarkRejected stores a privacy rejection reason. Rejected rows are NOT retried.
func (o *Outbox) MarkRejected(id, reason string) error {
	return o.update(id, StatusRejected, reason, time.Time{}, false)
}

// MarkFailed bumps attempts and sets the next exponential backoff window.
// `attempts` is the row's CURRENT attempt count (before this failure).
// On the 6th consecutive failure the row is parked at status='failed' and
// requires `korva hive retry` to re-enqueue.
func (o *Outbox) MarkFailed(id string, attempts int, errMsg string) error {
	if attempts+1 >= 6 {
		return o.update(id, StatusFailed, errMsg, time.Time{}, true)
	}
	next := time.Now().UTC().Add(backoff(attempts))
	return o.update(id, StatusPending, errMsg, next, true)
}

// Retry re-enqueues all 'failed' rows for another shot.
func (o *Outbox) Retry() (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := o.db.Exec(`
		UPDATE cloud_outbox
		SET status='pending', attempts=0, last_error='', next_attempt_at=?, updated_at=?
		WHERE status='failed'`, now, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Status returns aggregate counts across all rows.
func (o *Outbox) Status() (StatusCounts, error) {
	var c StatusCounts
	rows, err := o.db.Query(`SELECT status, COUNT(*) FROM cloud_outbox GROUP BY status`)
	if err != nil {
		return c, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return c, err
		}
		switch s {
		case StatusPending:
			c.Pending = n
		case StatusSent:
			c.Sent = n
		case StatusRejected:
			c.Rejected = n
		case StatusFailed:
			c.Failed = n
		}
	}
	return c, rows.Err()
}

func (o *Outbox) update(id, status, errMsg string, nextAt time.Time, bumpAttempts bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	nextStr := now
	if !nextAt.IsZero() {
		nextStr = nextAt.Format(time.RFC3339)
	}
	if bumpAttempts {
		_, err := o.db.Exec(`
			UPDATE cloud_outbox
			SET status=?, last_error=?, next_attempt_at=?, attempts=attempts+1, updated_at=?
			WHERE id=?`, status, errMsg, nextStr, now, id)
		return err
	}
	_, err := o.db.Exec(`
		UPDATE cloud_outbox
		SET status=?, last_error=?, next_attempt_at=?, updated_at=?
		WHERE id=?`, status, errMsg, nextStr, now, id)
	return err
}

// --- per-project sync controls ---

// ProjectSyncControl describes the sync state for a project.
type ProjectSyncControl struct {
	Project     string     `json:"project"`
	SyncEnabled bool       `json:"sync_enabled"`
	PausedBy    string     `json:"paused_by,omitempty"`
	PausedAt    *time.Time `json:"paused_at,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsProjectSyncEnabled returns false when the project has been explicitly paused.
// An absent row defaults to true (fail-open: unknown projects are allowed to sync).
// Returns (false, err) on DB errors — callers should log and treat as paused.
func (o *Outbox) IsProjectSyncEnabled(project string) (bool, error) {
	var enabled int
	err := o.db.QueryRow(
		`SELECT sync_enabled FROM project_sync_controls WHERE project = ?`, project,
	).Scan(&enabled)
	if err == sql.ErrNoRows {
		return true, nil // safe default: no control row = sync enabled
	}
	if err != nil {
		return false, fmt.Errorf("hive outbox: IsProjectSyncEnabled: %w", err)
	}
	return enabled == 1, nil
}

// PauseProjectSync disables Hive sync for the given project.
// Future Enqueue calls for this project will be skipped.
func (o *Outbox) PauseProjectSync(project, pausedBy, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := o.db.Exec(`
		INSERT INTO project_sync_controls (project, sync_enabled, paused_by, paused_at, reason, updated_at)
		VALUES (?, 0, ?, ?, ?, ?)
		ON CONFLICT(project) DO UPDATE SET
			sync_enabled = 0,
			paused_by    = excluded.paused_by,
			paused_at    = excluded.paused_at,
			reason       = excluded.reason,
			updated_at   = excluded.updated_at`,
		project, pausedBy, now, reason, now,
	)
	return err
}

// ResumeProjectSync re-enables Hive sync for the given project.
func (o *Outbox) ResumeProjectSync(project, resumedBy string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := o.db.Exec(`
		INSERT INTO project_sync_controls (project, sync_enabled, paused_by, paused_at, reason, updated_at)
		VALUES (?, 1, ?, NULL, '', ?)
		ON CONFLICT(project) DO UPDATE SET
			sync_enabled = 1,
			paused_by    = excluded.paused_by,
			paused_at    = NULL,
			reason       = '',
			updated_at   = excluded.updated_at`,
		project, resumedBy, now,
	)
	return err
}

// ListProjectSyncControls returns all rows from project_sync_controls.
func (o *Outbox) ListProjectSyncControls() ([]ProjectSyncControl, error) {
	rows, err := o.db.Query(`
		SELECT project, sync_enabled, paused_by, paused_at, reason, updated_at
		FROM project_sync_controls
		ORDER BY project ASC`)
	if err != nil {
		return nil, fmt.Errorf("hive outbox: ListProjectSyncControls: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ProjectSyncControl
	for rows.Next() {
		var c ProjectSyncControl
		var enabled int
		var pausedAt sql.NullString
		var updatedAt, pausedBy, reason string
		if err := rows.Scan(&c.Project, &enabled, &pausedBy, &pausedAt, &reason, &updatedAt); err != nil {
			return nil, err
		}
		c.SyncEnabled = enabled == 1
		c.PausedBy = pausedBy
		c.Reason = reason
		if pausedAt.Valid && pausedAt.String != "" {
			if t, err := time.Parse(time.RFC3339, pausedAt.String); err == nil {
				c.PausedAt = &t
			}
		}
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- per-project sync controls end ---

// backoff returns the delay before retrying after `attempts` failures.
// Schedule: 30s → 2m → 10m → 1h → 6h → 24h, then park at 'failed'.
func backoff(attempts int) time.Duration {
	switch attempts {
	case 0, 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	case 3:
		return 10 * time.Minute
	case 4:
		return time.Hour
	case 5:
		return 6 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// outboxEntropy is a process-wide monotonic ULID source. Mutex serializes
// access since ulid.Monotonic is not safe for concurrent reads.
//
// Same fix as store.newID and worker.newBatchID: math/rand seeded with
// time.Now().UnixNano() per call produces duplicate IDs on Windows
// (16ms time resolution).
var (
	outboxEntropyMu sync.Mutex
	outboxEntropy   = ulid.Monotonic(cryptorand.Reader, 0)
)

func newOutboxID() string {
	outboxEntropyMu.Lock()
	defer outboxEntropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), outboxEntropy).String()
}
