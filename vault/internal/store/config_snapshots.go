package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// ConfigSnapshot is a recorded mutation of korva.config.json.
// One row is inserted on every successful PUT /admin/config so the user can
// inspect, diff, or roll back configuration history.
type ConfigSnapshot struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	Scope      string    `json:"scope"`     // "local" | "global"
	FilePath   string    `json:"file_path"` // resolved absolute path
	BeforeHash string    `json:"before_hash"`
	AfterHash  string    `json:"after_hash"`
	BeforeJSON string    `json:"before_json"`
	AfterJSON  string    `json:"after_json"`
	CreatedAt  time.Time `json:"created_at"`
}

// SaveConfigSnapshot inserts a new row. before/after hashes are derived from the
// JSON contents — callers may pass empty hashes and they will be computed here.
func (s *Store) SaveConfigSnapshot(snap ConfigSnapshot) (string, error) {
	if snap.Scope == "" {
		return "", fmt.Errorf("config_snapshot: scope is required")
	}
	if snap.FilePath == "" {
		return "", fmt.Errorf("config_snapshot: file_path is required")
	}
	if snap.ID == "" {
		snap.ID = newID()
	}
	if snap.BeforeHash == "" {
		snap.BeforeHash = sha256Hex(snap.BeforeJSON)
	}
	if snap.AfterHash == "" {
		snap.AfterHash = sha256Hex(snap.AfterJSON)
	}
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO config_snapshots (
			id, actor, scope, file_path, before_hash, after_hash,
			before_json, after_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.Actor, snap.Scope, snap.FilePath, snap.BeforeHash, snap.AfterHash,
		snap.BeforeJSON, snap.AfterJSON,
		snap.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return "", fmt.Errorf("inserting config snapshot: %w", err)
	}
	return snap.ID, nil
}

// ListConfigSnapshots returns the newest `limit` snapshots filtered by scope
// (empty scope = all). Use limit=0 for the default of 50.
func (s *Store) ListConfigSnapshots(scope string, limit int) ([]ConfigSnapshot, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	query := `SELECT id, actor, scope, file_path, before_hash, after_hash,
	                  before_json, after_json, created_at
	             FROM config_snapshots WHERE 1=1`
	args := []any{}
	if scope != "" {
		query += " AND scope = ?"
		args = append(args, scope)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing config snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]ConfigSnapshot, 0, limit)
	for rows.Next() {
		snap, err := scanConfigSnapshot(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *snap)
	}
	return out, rows.Err()
}

// GetConfigSnapshot returns a single snapshot by ID, or nil if not found.
func (s *Store) GetConfigSnapshot(id string) (*ConfigSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, actor, scope, file_path, before_hash, after_hash,
		        before_json, after_json, created_at
		   FROM config_snapshots WHERE id = ?`,
		id,
	)
	snap, err := scanConfigSnapshot(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// LatestConfigSnapshot returns the most recent snapshot for `scope`, or nil
// if no snapshot exists for that scope yet.
func (s *Store) LatestConfigSnapshot(scope string) (*ConfigSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, actor, scope, file_path, before_hash, after_hash,
		        before_json, after_json, created_at
		   FROM config_snapshots WHERE scope = ?
		  ORDER BY created_at DESC LIMIT 1`,
		scope,
	)
	snap, err := scanConfigSnapshot(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func scanConfigSnapshot(scan func(...any) error) (*ConfigSnapshot, error) {
	var (
		snap      ConfigSnapshot
		createdAt string
	)
	err := scan(
		&snap.ID, &snap.Actor, &snap.Scope, &snap.FilePath,
		&snap.BeforeHash, &snap.AfterHash,
		&snap.BeforeJSON, &snap.AfterJSON,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}
	snap.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &snap, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
