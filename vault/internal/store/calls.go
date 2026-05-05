package store

import (
	"database/sql"
	"time"
)

// CallLog is one MCP tool invocation record.
type CallLog struct {
	ID        string
	Tool      string
	Project   string
	Author    string
	Status    string // "ok" | "error"
	LatencyMs int64
	ErrorMsg  string
	CreatedAt time.Time
}

// CallFilters controls which rows ListCalls returns.
type CallFilters struct {
	Tool    string // empty = all tools
	Project string // empty = all projects
	Author  string // empty = all authors
	Status  string // empty = all statuses
	Limit   int    // 0 → default 100
	Offset  int
}

// CallStats aggregates call metrics.
type CallStats struct {
	Total      int64
	ErrorCount int64
	AvgLatency float64            // milliseconds
	ByTool     map[string]int64   // call count per tool
	ByStatus   map[string]int64   // "ok" / "error" counts
}

// LogCall persists a single MCP call record. Failures are silently ignored
// so that a logging error never breaks the tool response path.
func (s *Store) LogCall(c CallLog) error {
	c.ID = newID()
	_, err := s.db.Exec(
		`INSERT INTO mcp_calls (id, tool, project, author, status, latency_ms, error_msg)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Tool, c.Project, c.Author, c.Status, c.LatencyMs, c.ErrorMsg,
	)
	return err
}

// ListCalls returns a filtered page of call logs, newest first.
func (s *Store) ListCalls(f CallFilters) ([]CallLog, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	query := `SELECT id, tool, project, author, status, latency_ms, error_msg, created_at
	           FROM mcp_calls WHERE 1=1`
	args := []any{}

	if f.Tool != "" {
		query += " AND tool = ?"
		args = append(args, f.Tool)
	}
	if f.Project != "" {
		query += " AND project = ?"
		args = append(args, f.Project)
	}
	if f.Author != "" {
		query += " AND author = ?"
		args = append(args, f.Author)
	}
	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, f.Status)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CallLog
	for rows.Next() {
		var c CallLog
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Tool, &c.Project, &c.Author, &c.Status,
			&c.LatencyMs, &c.ErrorMsg, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCallStats returns aggregate metrics across all call logs.
func (s *Store) GetCallStats() (*CallStats, error) {
	stats := &CallStats{
		ByTool:   make(map[string]int64),
		ByStatus: make(map[string]int64),
	}

	// Totals + average latency
	row := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='error' THEN 1 ELSE 0 END),0),
		        COALESCE(AVG(latency_ms),0)
		 FROM mcp_calls`,
	)
	if err := row.Scan(&stats.Total, &stats.ErrorCount, &stats.AvgLatency); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Per-tool breakdown
	rows, err := s.db.Query(`SELECT tool, COUNT(*) FROM mcp_calls GROUP BY tool ORDER BY COUNT(*) DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tool string
		var cnt int64
		if err := rows.Scan(&tool, &cnt); err != nil {
			return nil, err
		}
		stats.ByTool[tool] = cnt
	}

	// Per-status breakdown
	rows2, err := s.db.Query(`SELECT status, COUNT(*) FROM mcp_calls GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var st string
		var cnt int64
		if err := rows2.Scan(&st, &cnt); err != nil {
			return nil, err
		}
		stats.ByStatus[st] = cnt
	}

	return stats, nil
}
