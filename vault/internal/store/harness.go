package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Phase 14.1 — server-side persistence of per-repo Harness Engineering
// state. The harness state machine itself lives in each repo's
// `feature_list.json`; this layer is the **vault-side mirror** that
// powers Beacon's multi-project dashboard.
//
// Shape:
//   - HarnessSnapshot is the most-recent feature_list payload for one
//     (project, root) pair. Upserted on every MCP transition.
//   - HarnessTransition is one entry in the append-only state change
//     log. Powers the timeline + lead-time analytics in Beacon.
//
// Both are populated automatically by the MCP harness tools when the
// caller passes a `project` arg — the harness CLI continues to work
// fully offline; only when the agent talks to the vault does the data
// flow upstream.

// HarnessSnapshot is one (project, root) row of harness state. Payload
// is the marshaled feature_list.json — Beacon parses it client-side
// rather than us re-marshaling on every read.
type HarnessSnapshot struct {
	Project   string    `json:"project"`
	Root      string    `json:"root"`
	Payload   string    `json:"payload"` // raw JSON from feature_list.json
	UpdatedAt time.Time `json:"updated_at"`
}

// HarnessTransition is one entry in the state change log. The id is a
// short random hex prefix — collisions are negligible at this scale and
// the column carries no business meaning beyond "uniquely identify a row
// for delete / drill-in".
type HarnessTransition struct {
	ID         string    `json:"id"`
	Project    string    `json:"project"`
	Root       string    `json:"root"`
	FeatureID  int       `json:"feature_id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Owner      string    `json:"owner,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// HarnessProjectSummary is the roll-up Beacon's project list view
// consumes. Keeps the wire shape small — when the user clicks into a
// project the detail handler returns the full snapshot.
type HarnessProjectSummary struct {
	Project          string    `json:"project"`
	Root             string    `json:"root"`
	UpdatedAt        time.Time `json:"updated_at"`
	LastTransitionAt time.Time `json:"last_transition_at,omitempty"`
	LastTransitionTo string    `json:"last_transition_to,omitempty"`
}

// SaveHarnessSnapshot upserts one (project, root) snapshot.
// `payload` is the raw bytes of the on-disk feature_list.json — store
// it verbatim so Beacon parses the same shape the CLI emits.
func (s *Store) SaveHarnessSnapshot(project, root string, payload string) error {
	if project == "" {
		return fmt.Errorf("project is required")
	}
	if root == "" {
		return fmt.Errorf("root is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO harness_snapshots(project, root, payload, updated_at) VALUES(?,?,?,?)
		ON CONFLICT(project, root) DO UPDATE SET
			payload    = excluded.payload,
			updated_at = excluded.updated_at`,
		project, root, payload, now)
	if err != nil {
		return fmt.Errorf("upsert harness_snapshots: %w", err)
	}
	return nil
}

// GetHarnessSnapshot returns the most-recent snapshot for (project, root).
// Returns sql.ErrNoRows if the project has never been mirrored — let
// callers decide how to surface that (404 in the REST handler).
func (s *Store) GetHarnessSnapshot(project, root string) (*HarnessSnapshot, error) {
	var snap HarnessSnapshot
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project, root, payload, updated_at
		   FROM harness_snapshots WHERE project=? AND root=?`,
		project, root,
	).Scan(&snap.Project, &snap.Root, &snap.Payload, &updatedAt)
	if err != nil {
		return nil, err
	}
	snap.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &snap, nil
}

// ListHarnessSnapshots returns every (project, root) snapshot. Beacon's
// dashboard calls this to render the cards; per-project drill-in uses
// GetHarnessSnapshot.
func (s *Store) ListHarnessSnapshots() ([]HarnessSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT project, root, payload, updated_at
		   FROM harness_snapshots
		  ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list harness_snapshots: %w", err)
	}
	defer rows.Close()
	var out []HarnessSnapshot
	for rows.Next() {
		var snap HarnessSnapshot
		var updatedAt string
		if err := rows.Scan(&snap.Project, &snap.Root, &snap.Payload, &updatedAt); err != nil {
			return nil, err
		}
		snap.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, snap)
	}
	return out, rows.Err()
}

// ListHarnessProjectSummaries returns the dashboard-friendly roll-up:
// one row per (project, root) joined with its last transition. Beacon
// uses this for the card grid; the snapshot payload only loads on
// drill-in.
func (s *Store) ListHarnessProjectSummaries() ([]HarnessProjectSummary, error) {
	rows, err := s.db.Query(`
		SELECT
		  s.project,
		  s.root,
		  s.updated_at,
		  COALESCE(t.last_at, '')  AS last_transition_at,
		  COALESCE(t.last_to, '')  AS last_transition_to
		FROM harness_snapshots s
		LEFT JOIN (
		  SELECT project, root,
		         MAX(occurred_at) AS last_at,
		         (SELECT to_status
		            FROM harness_transitions t2
		           WHERE t2.project = harness_transitions.project
		             AND t2.root    = harness_transitions.root
		           ORDER BY occurred_at DESC LIMIT 1) AS last_to
		    FROM harness_transitions
		   GROUP BY project, root
		) t ON t.project = s.project AND t.root = s.root
		ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list harness summaries: %w", err)
	}
	defer rows.Close()
	var out []HarnessProjectSummary
	for rows.Next() {
		var sum HarnessProjectSummary
		var updated, lastAt string
		if err := rows.Scan(&sum.Project, &sum.Root, &updated, &lastAt, &sum.LastTransitionTo); err != nil {
			return nil, err
		}
		sum.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		if lastAt != "" {
			sum.LastTransitionAt, _ = time.Parse(time.RFC3339, lastAt)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// RecordHarnessTransition appends a row to the transition log. The
// caller already knows the from/to states because they just performed
// the SetStatus call on the in-memory feature list — passing both
// sides explicitly keeps this layer trivially testable (no extra query).
func (s *Store) RecordHarnessTransition(t HarnessTransition) error {
	if t.Project == "" || t.Root == "" {
		return fmt.Errorf("project and root are required")
	}
	if t.FeatureID <= 0 {
		return fmt.Errorf("feature_id must be > 0")
	}
	if t.ToStatus == "" {
		return fmt.Errorf("to_status is required")
	}
	id := t.ID
	if id == "" {
		id = newTransitionID()
	}
	occurred := t.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO harness_transitions
		  (id, project, root, feature_id, from_status, to_status, owner, occurred_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		id, t.Project, t.Root, t.FeatureID,
		t.FromStatus, t.ToStatus, t.Owner,
		occurred.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert harness_transitions: %w", err)
	}
	return nil
}

// ListHarnessTransitions returns transitions for a project, newest
// first, capped at `limit`. When project is empty it returns every
// project's transitions interleaved (used by a global timeline view).
// limit ≤ 0 falls back to 100.
func (s *Store) ListHarnessTransitions(project string, limit int) ([]HarnessTransition, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if project == "" {
		rows, err = s.db.Query(`
			SELECT id, project, root, feature_id, from_status, to_status, owner, occurred_at
			  FROM harness_transitions
			 ORDER BY occurred_at DESC
			 LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, project, root, feature_id, from_status, to_status, owner, occurred_at
			  FROM harness_transitions
			 WHERE project = ?
			 ORDER BY occurred_at DESC
			 LIMIT ?`, project, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list harness_transitions: %w", err)
	}
	defer rows.Close()
	var out []HarnessTransition
	for rows.Next() {
		var t HarnessTransition
		var occurred string
		if err := rows.Scan(&t.ID, &t.Project, &t.Root, &t.FeatureID,
			&t.FromStatus, &t.ToStatus, &t.Owner, &occurred); err != nil {
			return nil, err
		}
		t.OccurredAt, _ = time.Parse(time.RFC3339, occurred)
		out = append(out, t)
	}
	return out, rows.Err()
}

// newTransitionID generates a 16-char random hex id. Same length as the
// audit_logs id helper — collisions are negligible at the volumes this
// table sees (tens to hundreds per project per day).
func newTransitionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Crypto/rand failing means the kernel entropy source is broken;
		// fall back to a timestamp prefix so the row still inserts (the
		// uniqueness guarantee is degraded but that's better than refusing
		// to record the transition).
		return fmt.Sprintf("ts%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
