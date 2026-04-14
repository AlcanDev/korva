package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alcandev/korva/internal/db"
	"github.com/alcandev/korva/internal/privacy"
)

// Store provides all Vault persistence operations over SQLite.
type Store struct {
	db              *sql.DB
	privatePatterns []string
}

// New opens a Store backed by the SQLite database at dbPath.
func New(dbPath string, privatePatterns []string) (*Store, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening vault database: %w", err)
	}
	return &Store{db: database, privatePatterns: privatePatterns}, nil
}

// NewMemory opens an in-memory Store for testing.
func NewMemory(privatePatterns []string) (*Store, error) {
	database, err := db.OpenMemory()
	if err != nil {
		return nil, fmt.Errorf("opening in-memory vault: %w", err)
	}
	return &Store{db: database, privatePatterns: privatePatterns}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- vault_save ---

// Save stores an observation after running the privacy filter.
func (s *Store) Save(obs Observation) (string, error) {
	if obs.ID == "" {
		obs.ID = newID()
	}

	// Apply privacy filter to both title and content
	obs.Title = privacy.Filter(obs.Title, s.privatePatterns)
	obs.Content = privacy.Filter(obs.Content, s.privatePatterns)

	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = time.Now().UTC()
	}

	tags, err := json.Marshal(obs.Tags)
	if err != nil {
		return "", fmt.Errorf("serializing tags: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO observations (id, session_id, project, team, country, type, title, content, tags, author, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID, nullString(obs.SessionID), obs.Project, obs.Team, obs.Country,
		string(obs.Type), obs.Title, obs.Content, string(tags), obs.Author,
		obs.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("inserting observation: %w", err)
	}

	return obs.ID, nil
}

// --- vault_get ---

// Get retrieves a single observation by ID.
func (s *Store) Get(id string) (*Observation, error) {
	row := s.db.QueryRow(`
		SELECT id, COALESCE(session_id, ''), project, team, country, type,
		       title, content, tags, author, created_at
		FROM observations WHERE id = ?`, id)

	obs, err := scanObservation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return obs, err
}

// --- vault_search ---

// Search performs a full-text search with optional filters.
// If query is empty, returns recent observations matching the filters.
func (s *Store) Search(query string, filters SearchFilters) ([]Observation, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}

	var rows *sql.Rows
	var err error

	if strings.TrimSpace(query) != "" {
		rows, err = s.searchFTS(query, filters)
	} else {
		rows, err = s.searchRecent(filters)
	}

	if err != nil {
		return nil, fmt.Errorf("searching observations: %w", err)
	}
	defer rows.Close()

	return scanObservations(rows)
}

func (s *Store) searchFTS(query string, filters SearchFilters) (*sql.Rows, error) {
	args := []any{query}
	where := buildWhereClause(filters, &args)

	q := `
		SELECT o.id, COALESCE(o.session_id, ''), o.project, o.team, o.country,
		       o.type, o.title, o.content, o.tags, o.author, o.created_at
		FROM observations o
		JOIN observations_fts fts ON o.rowid = fts.rowid
		WHERE observations_fts MATCH ?` + where + `
		ORDER BY rank
		LIMIT ?`

	args = append(args, filters.Limit)
	return s.db.Query(q, args...)
}

func (s *Store) searchRecent(filters SearchFilters) (*sql.Rows, error) {
	args := []any{}
	where := buildWhereClause(filters, &args)
	if where != "" {
		where = "WHERE " + strings.TrimPrefix(where, " AND ")
	}

	q := `
		SELECT o.id, COALESCE(o.session_id, ''), o.project, o.team, o.country,
		       o.type, o.title, o.content, o.tags, o.author, o.created_at
		FROM observations o ` + where + `
		ORDER BY o.created_at DESC
		LIMIT ?`

	args = append(args, filters.Limit)
	return s.db.Query(q, args...)
}

// --- vault_context ---

// Context returns the most recent observations for a project.
func (s *Store) Context(project string, types []ObservationType, limit int) ([]Observation, error) {
	if limit <= 0 {
		limit = 10
	}

	args := []any{project}
	typeFilter := ""
	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i, t := range types {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		typeFilter = " AND type IN (" + strings.Join(placeholders, ",") + ")"
	}

	args = append(args, limit)
	rows, err := s.db.Query(`
		SELECT id, COALESCE(session_id, ''), project, team, country,
		       type, title, content, tags, author, created_at
		FROM observations
		WHERE project = ?`+typeFilter+`
		ORDER BY created_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("querying context: %w", err)
	}
	defer rows.Close()

	return scanObservations(rows)
}

// --- vault_timeline ---

// Timeline returns observations within a date range.
func (s *Store) Timeline(project string, from, to time.Time) ([]Observation, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(session_id, ''), project, team, country,
		       type, title, content, tags, author, created_at
		FROM observations
		WHERE project = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at ASC`,
		project,
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("querying timeline: %w", err)
	}
	defer rows.Close()

	return scanObservations(rows)
}

// --- vault_session_start ---

// SessionStart creates a new session and returns its ID.
func (s *Store) SessionStart(project, team, country, agent, goal string) (string, error) {
	id := newID()
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, project, team, country, agent, goal, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, project, team, country, agent, goal,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("starting session: %w", err)
	}
	return id, nil
}

// --- vault_session_end ---

// SessionEnd finalizes a session with a summary.
func (s *Store) SessionEnd(id, summary string) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET summary = ?, ended_at = ? WHERE id = ?`,
		privacy.Filter(summary, s.privatePatterns),
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	return err
}

// --- vault_summary ---

// Summary returns a project-level summary of stored knowledge.
func (s *Store) Summary(project string) (*ProjectSummary, error) {
	summary := &ProjectSummary{Project: project}

	// Count observations
	s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ?`, project).
		Scan(&summary.Observations)

	// Count sessions
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project = ?`, project).
		Scan(&summary.Sessions)

	// Recent observations
	recent, err := s.Context(project, nil, 5)
	if err != nil {
		return nil, err
	}
	summary.Recent = recent

	// Key decisions
	decisions, err := s.Context(project, []ObservationType{TypeDecision}, 5)
	if err != nil {
		return nil, err
	}
	summary.Decisions = decisions

	return summary, nil
}

// --- vault_save_prompt ---

// SavePrompt stores or updates a reusable prompt template.
func (s *Store) SavePrompt(name, content string, tags []string) error {
	id := newID()
	tagsJSON, _ := json.Marshal(tags)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO prompts (id, name, content, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			content    = excluded.content,
			tags       = excluded.tags,
			updated_at = excluded.updated_at`,
		id, name, privacy.Filter(content, s.privatePatterns),
		string(tagsJSON), now, now,
	)
	return err
}

// --- list_sessions ---

// ListSessions returns the most recent sessions, up to limit.
func (s *Store) ListSessions(limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, project, team, country, agent, goal, COALESCE(summary, ''),
		       started_at, ended_at
		FROM sessions
		ORDER BY started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var startedAt, endedAt string
		var endedAtNull sql.NullString
		err := rows.Scan(
			&sess.ID, &sess.Project, &sess.Team, &sess.Country,
			&sess.Agent, &sess.Goal, &sess.Summary,
			&startedAt, &endedAtNull,
		)
		if err != nil {
			return nil, err
		}
		_ = endedAt
		sess.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if endedAtNull.Valid {
			t, _ := time.Parse(time.RFC3339, endedAtNull.String)
			sess.EndedAt = &t
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// --- vault_stats ---

// Stats returns aggregate statistics for the entire vault.
func (s *Store) Stats() (*VaultStats, error) {
	stats := &VaultStats{
		ByType:    make(map[string]int),
		ByProject: make(map[string]int),
		ByTeam:    make(map[string]int),
	}

	s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&stats.TotalObservations)
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.TotalSessions)
	s.db.QueryRow(`SELECT COUNT(*) FROM prompts`).Scan(&stats.TotalPrompts)

	rows, err := s.db.Query(`SELECT type, COUNT(*) FROM observations GROUP BY type`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var n int
			rows.Scan(&t, &n)
			stats.ByType[t] = n
		}
	}

	rows, err = s.db.Query(`SELECT project, COUNT(*) FROM observations WHERE project != '' GROUP BY project`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			var n int
			rows.Scan(&p, &n)
			stats.ByProject[p] = n
		}
	}

	rows, err = s.db.Query(`SELECT team, COUNT(*) FROM observations WHERE team != '' GROUP BY team`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var n int
			rows.Scan(&t, &n)
			stats.ByTeam[t] = n
		}
	}

	return stats, nil
}

// --- helpers ---

func buildWhereClause(f SearchFilters, args *[]any) string {
	var parts []string
	if f.Project != "" {
		parts = append(parts, "o.project = ?")
		*args = append(*args, f.Project)
	}
	if f.Team != "" {
		parts = append(parts, "o.team = ?")
		*args = append(*args, f.Team)
	}
	if f.Country != "" {
		parts = append(parts, "o.country = ?")
		*args = append(*args, f.Country)
	}
	if f.Type != "" {
		parts = append(parts, "o.type = ?")
		*args = append(*args, string(f.Type))
	}
	if f.Author != "" {
		parts = append(parts, "o.author = ?")
		*args = append(*args, f.Author)
	}
	if len(parts) == 0 {
		return ""
	}
	return " AND " + strings.Join(parts, " AND ")
}

func scanObservations(rows *sql.Rows) ([]Observation, error) {
	var result []Observation
	for rows.Next() {
		obs, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *obs)
	}
	return result, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanObservation(s scanner) (*Observation, error) {
	var obs Observation
	var tagsJSON string
	var createdAt string

	err := s.Scan(
		&obs.ID, &obs.SessionID, &obs.Project, &obs.Team, &obs.Country,
		&obs.Type, &obs.Title, &obs.Content, &tagsJSON, &obs.Author, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(tagsJSON), &obs.Tags)
	if obs.Tags == nil {
		obs.Tags = []string{}
	}

	obs.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &obs, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newID() string {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
