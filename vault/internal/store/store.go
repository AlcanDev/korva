package store

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alcandev/korva/internal/db"
	"github.com/alcandev/korva/internal/privacy"
)

// HiveEnqueuer is implemented by hive.Outbox. The store calls it after
// every successful Save so cloud sync is decoupled from local persistence.
// A nil enqueuer disables Hive sync (Community installs that opted out).
type HiveEnqueuer interface {
	Enqueue(observationID string, payload []byte) error
}

// Store provides all Vault persistence operations over SQLite.
type Store struct {
	db              *sql.DB
	privatePatterns []string
	hive            HiveEnqueuer
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

// AttachHive wires a Hive outbox to receive enqueue calls after every Save.
// Pass nil to disable Hive sync for this store.
func (s *Store) AttachHive(h HiveEnqueuer) {
	s.hive = h
}

// DB exposes the underlying *sql.DB for callers that need to construct
// peer components (e.g. hive.Outbox) over the same database file.
func (s *Store) DB() *sql.DB {
	return s.db
}

// PreviewFilter applies the privacy filter to title and content without saving.
// Use this for dry-run previews so callers can inspect what would be stored.
func (s *Store) PreviewFilter(title, content string) (filteredTitle, filteredContent string) {
	return privacy.Filter(title, s.privatePatterns), privacy.Filter(content, s.privatePatterns)
}

// --- vault_save ---

// Save stores an observation after running the privacy filter.
//
// Two guards run before the INSERT:
//  1. Ghost detection (claude-mem): observations with no title AND no content are
//     silently dropped — they carry zero information and pollute search results.
//  2. Content-hash deduplication (claude-mem): if an identical observation already
//     exists (same title, content, and project) the existing ID is returned without
//     a new row, preventing observation loops in multi-turn sessions.
func (s *Store) Save(obs Observation) (string, error) {
	// 1. Ghost detection: at least one of title/content must be non-empty.
	if strings.TrimSpace(obs.Title) == "" && strings.TrimSpace(obs.Content) == "" {
		return "", fmt.Errorf("vault_save: observation has neither title nor content")
	}

	if obs.ID == "" {
		obs.ID = newID()
	}

	// Apply privacy filter to both title and content.
	obs.Title = privacy.Filter(obs.Title, s.privatePatterns)
	obs.Content = privacy.Filter(obs.Content, s.privatePatterns)

	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = time.Now().UTC()
	}

	// 2. Content-hash deduplication (claude-mem pattern).
	//    Prevents observation LOOPS: when an AI agent re-processes the same tool output
	//    within the same session it would otherwise create identical duplicates.
	//    Scoped to the active session_id to avoid rejecting deliberate re-saves across
	//    different sessions (those are intentional and should always be stored).
	hash := obsContentHash(obs.Title, obs.Content, obs.Project)
	if obs.SessionID != "" {
		var existingID string
		if err := s.db.QueryRow(
			`SELECT id FROM observations WHERE content_hash = ? AND session_id = ? LIMIT 1`,
			hash, obs.SessionID,
		).Scan(&existingID); err == nil {
			// Identical observation already exists in this session — return its ID silently.
			return existingID, nil
		}
	}

	tags, err := json.Marshal(obs.Tags)
	if err != nil {
		return "", fmt.Errorf("serializing tags: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO observations (id, session_id, project, team, country, type, title, content, tags, author, created_at, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID, nullString(obs.SessionID), obs.Project, obs.Team, obs.Country,
		string(obs.Type), obs.Title, obs.Content, string(tags), obs.Author,
		obs.CreatedAt.UTC().Format(time.RFC3339), hash,
	)
	if err != nil {
		return "", fmt.Errorf("inserting observation: %w", err)
	}

	// Hive enqueue is best-effort: Save must never fail because cloud sync misbehaved.
	if s.hive != nil {
		if payload, mErr := json.Marshal(obs); mErr == nil {
			_ = s.hive.Enqueue(obs.ID, payload)
		}
	}

	return obs.ID, nil
}

// obsContentHash returns a stable 32-hex-char fingerprint for an observation.
// Scoped by project so identical text in different projects does not deduplicate.
func obsContentHash(title, content, project string) string {
	h := sha256.Sum256([]byte(title + "|" + content + "|" + project))
	return fmt.Sprintf("%x", h)[:32]
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

// Delete removes an observation by ID. Returns (true, nil) if deleted,
// (false, nil) if the ID was not found, or (false, err) on DB error.
func (s *Store) Delete(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM observations WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete observation %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// --- vault_search ---

// CountObservations returns the total number of observations matching the
// given filters, ignoring Limit and Offset. Used for pagination metadata.
func (s *Store) CountObservations(filters SearchFilters) (int, error) {
	args := []any{}
	where := buildWhereClause(filters, &args)
	if where != "" {
		where = "WHERE " + strings.TrimPrefix(where, " AND ")
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM observations o `+where, args...).Scan(&n)
	return n, err
}

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
		LIMIT ? OFFSET ?`

	args = append(args, filters.Limit, filters.Offset)
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
		LIMIT ? OFFSET ?`

	args = append(args, filters.Limit, filters.Offset)
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

// ContextSince returns observations for the project that were saved after sinceID
// (exclusive). ULIDs are lexicographically monotonic so a simple id > sinceID
// comparison gives correct ordering without a timestamp index.
func (s *Store) ContextSince(project, sinceID string, limit int) ([]Observation, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT id, COALESCE(session_id, ''), project, team, country,
		       type, title, content, tags, author, created_at
		FROM observations
		WHERE project = ? AND id > ?
		ORDER BY id DESC
		LIMIT ?`, project, sinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying context since: %w", err)
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

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE project = ?`, project).
		Scan(&summary.Observations); err != nil {
		return nil, fmt.Errorf("summary observations count: %w", err)
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project = ?`, project).
		Scan(&summary.Sessions); err != nil {
		return nil, fmt.Errorf("summary sessions count: %w", err)
	}

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
		ByCountry: make(map[string]int),
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&stats.TotalObservations); err != nil {
		return nil, fmt.Errorf("stats observations count: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.TotalSessions); err != nil {
		return nil, fmt.Errorf("stats sessions count: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM prompts`).Scan(&stats.TotalPrompts); err != nil {
		return nil, fmt.Errorf("stats prompts count: %w", err)
	}

	if err := scanGroupCount(s.db, `SELECT type, COUNT(*) FROM observations GROUP BY type`, stats.ByType); err != nil {
		return nil, fmt.Errorf("stats by type: %w", err)
	}
	if err := scanGroupCount(s.db, `SELECT project, COUNT(*) FROM observations WHERE project != '' GROUP BY project`, stats.ByProject); err != nil {
		return nil, fmt.Errorf("stats by project: %w", err)
	}
	if err := scanGroupCount(s.db, `SELECT team, COUNT(*) FROM observations WHERE team != '' GROUP BY team`, stats.ByTeam); err != nil {
		return nil, fmt.Errorf("stats by team: %w", err)
	}
	if err := scanGroupCount(s.db, `SELECT country, COUNT(*) FROM observations WHERE country != '' GROUP BY country`, stats.ByCountry); err != nil {
		return nil, fmt.Errorf("stats by country: %w", err)
	}

	return stats, nil
}

// scanGroupCount runs a "SELECT key, COUNT(*) … GROUP BY" query and populates dest.
func scanGroupCount(db *sql.DB, query string, dest map[string]int) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		dest[k] = n
	}
	return rows.Err()
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
	if !f.Since.IsZero() {
		parts = append(parts, "o.created_at >= ?")
		*args = append(*args, f.Since.UTC().Format(time.RFC3339))
	}
	if !f.Until.IsZero() {
		parts = append(parts, "o.created_at <= ?")
		*args = append(*args, f.Until.UTC().Format(time.RFC3339))
	}
	if len(parts) == 0 {
		return ""
	}
	return " AND " + strings.Join(parts, " AND ")
}

// --- vault_purge ---

// Purge deletes observations that match opts. At least one filter is required.
// When DryRun is true, it returns the count that would be deleted without
// performing the actual deletion.
func (s *Store) Purge(opts PurgeOptions) (int, error) {
	var conds []string
	var args []any

	if opts.Project != "" {
		conds = append(conds, "project = ?")
		args = append(args, opts.Project)
	}
	if opts.Team != "" {
		conds = append(conds, "team = ?")
		args = append(args, opts.Team)
	}
	if opts.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, opts.Type)
	}
	if !opts.Before.IsZero() {
		conds = append(conds, "created_at < ?")
		args = append(args, opts.Before.UTC().Format(time.RFC3339))
	}
	if len(conds) == 0 {
		return 0, fmt.Errorf("purge requires at least one filter (project, team, type, or before) to prevent accidental full deletion")
	}

	where := " WHERE " + strings.Join(conds, " AND ")

	if opts.DryRun {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM observations"+where, args...).Scan(&n); err != nil {
			return 0, fmt.Errorf("purge dry-run: %w", err)
		}
		return n, nil
	}

	res, err := s.db.Exec("DELETE FROM observations"+where, args...)
	if err != nil {
		return 0, fmt.Errorf("purge: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// --- vault_export ---

// Export returns all observations matching opts, ordered by creation time ascending.
// All fields are optional — empty values mean "no filter".
func (s *Store) Export(opts ExportOptions) ([]Observation, error) {
	var conds []string
	var args []any

	if opts.Project != "" {
		conds = append(conds, "project = ?")
		args = append(args, opts.Project)
	}
	if opts.Team != "" {
		conds = append(conds, "team = ?")
		args = append(args, opts.Team)
	}
	if opts.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, opts.Type)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	rows, err := s.db.Query(`
		SELECT id, COALESCE(session_id,''), project, team, country, type,
		       title, content, tags, author, created_at
		FROM observations`+where+` ORDER BY created_at ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// --- vault_clean (dedup) ---

// Dedup detects observations with an identical (project, type, content) triplet
// and removes all but the oldest one (by ULID, which encodes creation time).
// When DryRun is true, it counts duplicates without deleting anything.
// When project is empty, the operation runs across the entire vault.
func (s *Store) Dedup(project string, dryRun bool) (DedupResult, error) {
	var totalQuery string
	var dupQuery string
	var deleteQuery string
	var qArgs []any

	if project != "" {
		totalQuery = `SELECT COUNT(*) FROM observations WHERE project = ?`
		dupQuery = `
			SELECT COUNT(*) FROM observations
			WHERE project = ?
			AND id NOT IN (
				SELECT MIN(id) FROM observations
				WHERE project = ?
				GROUP BY project, type, lower(trim(content))
			)`
		deleteQuery = `
			DELETE FROM observations
			WHERE project = ?
			AND id NOT IN (
				SELECT MIN(id) FROM observations
				WHERE project = ?
				GROUP BY project, type, lower(trim(content))
			)`
		qArgs = []any{project, project}
	} else {
		totalQuery = `SELECT COUNT(*) FROM observations`
		dupQuery = `
			SELECT COUNT(*) FROM observations
			WHERE id NOT IN (
				SELECT MIN(id) FROM observations
				GROUP BY project, type, lower(trim(content))
			)`
		deleteQuery = `
			DELETE FROM observations
			WHERE id NOT IN (
				SELECT MIN(id) FROM observations
				GROUP BY project, type, lower(trim(content))
			)`
		qArgs = nil
	}

	var result DedupResult
	result.DryRun = dryRun

	totalArgs := qArgs[:0:0] // reuse backing array but reset len
	if project != "" {
		totalArgs = []any{project}
	}
	if err := s.db.QueryRow(totalQuery, totalArgs...).Scan(&result.Total); err != nil {
		return result, fmt.Errorf("dedup count total: %w", err)
	}
	if err := s.db.QueryRow(dupQuery, qArgs...).Scan(&result.Duplicates); err != nil {
		return result, fmt.Errorf("dedup count duplicates: %w", err)
	}
	if dryRun || result.Duplicates == 0 {
		return result, nil
	}

	// Wrap delete in a transaction so concurrent inserts don't cause
	// inconsistent counts between the COUNT query and the DELETE.
	tx, err := s.db.Begin()
	if err != nil {
		return result, fmt.Errorf("dedup begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.Exec(deleteQuery, qArgs...)
	if err != nil {
		return result, fmt.Errorf("dedup delete: %w", err)
	}
	n, _ := res.RowsAffected()
	result.Removed = int(n)

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("dedup commit: %w", err)
	}
	return result, nil
}

// ── SDD phase (gentle-ai) ─────────────────────────────────────────────────────

// GetSDDPhase returns the current SDD phase for a project.
// Returns SDDExplore as default when no phase has been set.
func (s *Store) GetSDDPhase(project string) (*SDDState, error) {
	var st SDDState
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project, phase, updated_at FROM sdd_state WHERE project = ?`, project,
	).Scan(&st.Project, &st.Phase, &updatedAt)
	if err == sql.ErrNoRows {
		return &SDDState{Project: project, Phase: SDDExplore, UpdatedAt: time.Now()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sdd phase: %w", err)
	}
	st.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &st, nil
}

// SetSDDPhase updates (or inserts) the SDD phase for a project.
// Returns an error if phase is not a valid SDD phase name.
func (s *Store) SetSDDPhase(project string, phase SDDPhase) error {
	valid := false
	for _, p := range AllSDDPhases {
		if p == string(phase) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid SDD phase %q — valid phases: %s",
			phase, strings.Join(AllSDDPhases, ", "))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO sdd_state(project, phase, updated_at) VALUES(?,?,?)
		ON CONFLICT(project) DO UPDATE SET phase=excluded.phase, updated_at=excluded.updated_at`,
		project, string(phase), now)
	return err
}

// ── OpenSpec (gentle-ai project conventions) ─────────────────────────────────

// GetOpenSpec returns the project conventions stored for project.
// Returns an empty OpenSpec (no error) when nothing has been set yet.
func (s *Store) GetOpenSpec(project string) (*OpenSpec, error) {
	var sp OpenSpec
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project, content, updated_at FROM openspec WHERE project = ?`, project,
	).Scan(&sp.Project, &sp.Content, &updatedAt)
	if err == sql.ErrNoRows {
		return &OpenSpec{Project: project}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get openspec: %w", err)
	}
	sp.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &sp, nil
}

// SaveOpenSpec writes (or replaces) the project conventions.
func (s *Store) SaveOpenSpec(project, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO openspec(project, content, updated_at) VALUES(?,?,?)
		ON CONFLICT(project) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		project, content, now)
	return err
}

// ── Quality gate checkpoints ─────────────────────────────────────────────────

// SaveQualityCheckpoint persists a QA assessment result.
// If cp.ID is empty a new ULID is generated.
func (s *Store) SaveQualityCheckpoint(cp QualityCheckpoint) (string, error) {
	if cp.ID == "" {
		cp.ID = newID()
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}

	findings, err := json.Marshal(cp.Findings)
	if err != nil {
		return "", fmt.Errorf("serializing findings: %w", err)
	}

	gatePassed := 0
	if cp.GatePassed {
		gatePassed = 1
	}

	_, err = s.db.Exec(`
		INSERT INTO quality_checkpoints
		(id, project, session_id, phase, language, status, score, findings, notes, gate_passed, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cp.ID, cp.Project, cp.SessionID, cp.Phase, cp.Language,
		string(cp.Status), cp.Score, string(findings), cp.Notes,
		gatePassed, cp.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("inserting quality checkpoint: %w", err)
	}
	return cp.ID, nil
}

// GetQualityCheckpoints returns the most recent quality checkpoints for a project.
func (s *Store) GetQualityCheckpoints(project string, limit int) ([]QualityCheckpoint, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT id, project, COALESCE(session_id,''), phase, COALESCE(language,''),
		       status, score, findings, COALESCE(notes,''), gate_passed, created_at
		FROM quality_checkpoints
		WHERE project = ?
		ORDER BY created_at DESC
		LIMIT ?`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("querying checkpoints: %w", err)
	}
	defer rows.Close()
	return scanCheckpoints(rows)
}

// GetLatestCheckpointForPhase returns the most recent checkpoint for the given project+phase.
// Returns (nil, nil) when none exists.
func (s *Store) GetLatestCheckpointForPhase(project, phase string) (*QualityCheckpoint, error) {
	row := s.db.QueryRow(`
		SELECT id, project, COALESCE(session_id,''), phase, COALESCE(language,''),
		       status, score, findings, COALESCE(notes,''), gate_passed, created_at
		FROM quality_checkpoints
		WHERE project = ? AND phase = ?
		ORDER BY created_at DESC
		LIMIT 1`, project, phase)
	cp, err := scanCheckpoint(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cp, err
}

// GetProjectQualityScore computes a quality score summary for the project.
func (s *Store) GetProjectQualityScore(project string) (*ProjectQualityScore, error) {
	ps := &ProjectQualityScore{Project: project}

	rows, err := s.db.Query(`
		SELECT score, gate_passed, phase, created_at
		FROM quality_checkpoints
		WHERE project = ?
		ORDER BY created_at DESC`, project)
	if err != nil {
		return nil, fmt.Errorf("querying quality score: %w", err)
	}
	defer rows.Close()

	var total, sum, passed int
	var latest int
	var latestPhase string
	var latestAt time.Time
	first := true

	for rows.Next() {
		var score, gatePassed int
		var phase, createdAt string
		if err := rows.Scan(&score, &gatePassed, &phase, &createdAt); err != nil {
			return nil, err
		}
		total++
		sum += score
		if gatePassed == 1 {
			passed++
		}
		if first {
			latest = score
			latestPhase = phase
			latestAt, _ = time.Parse(time.RFC3339, createdAt)
			first = false
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ps.TotalChecks = total
	ps.PassedGates = passed
	if total > 0 {
		ps.LatestScore = latest
		ps.AverageScore = sum / total
		ps.LastCheckPhase = latestPhase
		ps.LastCheckedAt = latestAt
	}
	return ps, nil
}

func scanCheckpoints(rows *sql.Rows) ([]QualityCheckpoint, error) {
	var result []QualityCheckpoint
	for rows.Next() {
		cp, err := scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *cp)
	}
	return result, rows.Err()
}

type cpScanner interface {
	Scan(dest ...any) error
}

func scanCheckpoint(sc cpScanner) (*QualityCheckpoint, error) {
	var cp QualityCheckpoint
	var gatePassed int
	var findingsJSON, createdAt string

	err := sc.Scan(
		&cp.ID, &cp.Project, &cp.SessionID, &cp.Phase, &cp.Language,
		(*string)(&cp.Status), &cp.Score, &findingsJSON, &cp.Notes,
		&gatePassed, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	cp.GatePassed = gatePassed == 1
	if findingsJSON != "" {
		if err := json.Unmarshal([]byte(findingsJSON), &cp.Findings); err != nil {
			return nil, fmt.Errorf("scan checkpoint findings: %w", err)
		}
	}
	if cp.Findings == nil {
		cp.Findings = []QualityFinding{}
	}
	cp.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &cp, nil
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

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &obs.Tags); err != nil {
			return nil, fmt.Errorf("scan observation tags: %w", err)
		}
	}
	if obs.Tags == nil {
		obs.Tags = []string{}
	}

	obs.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &obs, nil
}

// ScrollExportNote is one scroll in a lore export response.
type ScrollExportNote struct {
	Path      string    `json:"path"`    // e.g. "private/auth-patterns"
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	TeamID    string    `json:"team_id,omitempty"`
	Hash      string    `json:"hash"`       // SHA-256[:12] for change detection
	UpdatedAt time.Time `json:"updated_at"`
	Deleted   bool      `json:"deleted,omitempty"`
}

// ExportScrollsOptions constrains which scrolls are included.
type ExportScrollsOptions struct {
	// TeamID limits results to a specific team. Empty returns all teams.
	TeamID string
	// Since returns only scrolls updated after this time. Zero = all.
	Since time.Time
	// Limit is the max number of results (0 = default 100).
	Limit int
	// Offset is the number of results to skip for pagination.
	Offset int
}

// ExportScrolls returns private scrolls from the DB, suitable for incremental
// export to external tools. The caller uses the Hash field to detect changes.
// Returns (notes, total, error) where total is the count without limit/offset.
func (s *Store) ExportScrolls(opts ExportScrollsOptions) ([]ScrollExportNote, int, error) {
	var args []any
	var conditions []string

	if opts.TeamID != "" {
		conditions = append(conditions, "team_id = ?")
		args = append(args, opts.TeamID)
	}
	if !opts.Since.IsZero() {
		conditions = append(conditions, "updated_at > ?")
		args = append(args, opts.Since.UTC().Format(time.RFC3339))
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM private_scrolls"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("export scrolls count: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT name, content, team_id, updated_at FROM private_scrolls` +
		where + " ORDER BY team_id ASC, name ASC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, opts.Offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("export scrolls: %w", err)
	}
	defer rows.Close()

	var out []ScrollExportNote
	for rows.Next() {
		var n ScrollExportNote
		var updatedAt string
		if err := rows.Scan(&n.Name, &n.Content, &n.TeamID, &updatedAt); err != nil {
			return nil, 0, err
		}
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		n.Path = "private/" + n.Name
		sum := sha256.Sum256([]byte(n.Content))
		n.Hash = fmt.Sprintf("%x", sum[:6]) // 12 hex chars
		out = append(out, n)
	}
	if out == nil {
		out = []ScrollExportNote{}
	}
	return out, total, rows.Err()
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// newID returns a fresh ULID. The entropy source is shared across calls and
// uses ulid.Monotonic so two ULIDs minted in the same millisecond are strictly
// ordered. This is critical on Windows where time.Now() resolution can be
// ~16ms — without monotonic entropy we would generate duplicate IDs and hit
// "UNIQUE constraint failed: observations.id" under tight save loops.
//
// The mutex serialises access to the monotonic source which is not safe for
// concurrent reads.
var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(cryptorand.Reader, 0)
)

func newID() string {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
