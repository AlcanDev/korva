package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/privacy"
)

// excerptMaxBytes caps prompt and response excerpts to keep DB rows bounded.
// 8 KiB covers ~2K tokens of context — enough for a meaningful preview without
// turning the activity log into a full transcript store.
const excerptMaxBytes = 8 * 1024

// estimatedTokensDivisor is the rough chars-per-token ratio used as a fallback
// when the client did not report `usage` from the Anthropic SDK.
const estimatedTokensDivisor = 4

// Interaction is one prompt round-trip recorded for Observatory analytics.
// Phase 18.C — `Editor` is the optional X-Korva-Editor opt-in tag that
// drives the Beacon adoption widget. Empty when the caller did not
// identify itself; otherwise one of harness.AllEditors.
type Interaction struct {
	ID              string          `json:"id"`
	SessionID       string          `json:"session_id,omitempty"`
	Project         string          `json:"project"`
	Team            string          `json:"team,omitempty"`
	Agent           string          `json:"agent"`
	Editor          string          `json:"editor,omitempty"`
	Model           string          `json:"model"`
	PromptExcerpt   string          `json:"prompt_excerpt"`
	ResponseExcerpt string          `json:"response_excerpt,omitempty"`
	InputTokens     int64           `json:"input_tokens"`
	OutputTokens    int64           `json:"output_tokens"`
	CacheRead       int64           `json:"cache_read"`
	CacheCreation   int64           `json:"cache_creation"`
	DurationMs      int64           `json:"duration_ms"`
	ToolCalls       json.RawMessage `json:"tool_calls"`
	Status          string          `json:"status"`
	ErrorMsg        string          `json:"error_msg,omitempty"`
	Estimated       bool            `json:"estimated"`
	CreatedAt       time.Time       `json:"created_at"`
}

// InteractionFilters constrains ListInteractions queries.
type InteractionFilters struct {
	Project string
	Model   string
	Agent   string
	Editor  string // Phase 18.C — filter by harness editor id
	Status  string
	Query   string // FTS5 query against prompt_excerpt + response_excerpt
	Since   time.Time
	Until   time.Time
	Limit   int
	Offset  int
}

// SaveInteraction persists a new interaction row after applying the privacy filter
// to prompt and response. If input/output tokens are both zero the row is marked
// estimated=1 and tokens are derived from len(prompt)+len(response)/4 as a fallback.
//
// tool_calls must be valid JSON (any shape) or empty — invalid JSON is rejected
// to keep the column queryable. Pass `nil` or `[]byte("[]")` when there are none.
func (s *Store) SaveInteraction(in Interaction) (string, error) {
	in.Project = strings.TrimSpace(in.Project)
	in.Agent = strings.TrimSpace(in.Agent)
	in.Model = strings.TrimSpace(in.Model)

	if in.Project == "" {
		return "", fmt.Errorf("interaction: project is required")
	}
	if in.Agent == "" {
		return "", fmt.Errorf("interaction: agent is required")
	}

	if in.ID == "" {
		in.ID = newID()
	}

	if in.Status == "" {
		in.Status = "ok"
	}

	// Privacy filter on free-text fields before persistence.
	in.PromptExcerpt = truncate(privacy.Filter(in.PromptExcerpt, s.privatePatterns), excerptMaxBytes)
	in.ResponseExcerpt = truncate(privacy.Filter(in.ResponseExcerpt, s.privatePatterns), excerptMaxBytes)
	in.ErrorMsg = privacy.Filter(in.ErrorMsg, s.privatePatterns)

	// Validate tool_calls JSON; default to "[]" when empty.
	toolCallsBytes := []byte(in.ToolCalls)
	if len(toolCallsBytes) == 0 {
		toolCallsBytes = []byte("[]")
	} else {
		var probe any
		if err := json.Unmarshal(toolCallsBytes, &probe); err != nil {
			return "", fmt.Errorf("interaction: tool_calls is not valid JSON: %w", err)
		}
	}

	// Token-fallback estimation when the client did not report usage.
	if in.InputTokens == 0 && in.OutputTokens == 0 && in.CacheRead == 0 && in.CacheCreation == 0 {
		in.Estimated = true
		in.InputTokens = int64(len(in.PromptExcerpt) / estimatedTokensDivisor)
		in.OutputTokens = int64(len(in.ResponseExcerpt) / estimatedTokensDivisor)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}

	estimatedFlag := 0
	if in.Estimated {
		estimatedFlag = 1
	}

	_, err := s.db.Exec(
		`INSERT INTO interactions (
			id, session_id, project, team, agent, editor, model,
			prompt_excerpt, response_excerpt,
			input_tokens, output_tokens, cache_read, cache_creation,
			duration_ms, tool_calls, status, error_msg, estimated,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ID, nullableString(in.SessionID), in.Project, in.Team, in.Agent, in.Editor, in.Model,
		in.PromptExcerpt, in.ResponseExcerpt,
		in.InputTokens, in.OutputTokens, in.CacheRead, in.CacheCreation,
		in.DurationMs, string(toolCallsBytes), in.Status, in.ErrorMsg, estimatedFlag,
		in.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return "", fmt.Errorf("inserting interaction: %w", err)
	}
	return in.ID, nil
}

// GetInteraction returns one interaction by ID, or nil if not found.
func (s *Store) GetInteraction(id string) (*Interaction, error) {
	row := s.db.QueryRow(
		`SELECT id, COALESCE(session_id,''), project, team, agent, editor, model,
		        prompt_excerpt, response_excerpt,
		        input_tokens, output_tokens, cache_read, cache_creation,
		        duration_ms, tool_calls, status, error_msg, estimated,
		        created_at
		   FROM interactions WHERE id = ?`,
		id,
	)
	in, err := scanInteraction(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return in, nil
}

// ListInteractions returns a filtered page of interactions, newest first.
// When f.Query is non-empty the query uses interactions_fts (FTS5) and joins
// back to the base table; otherwise it scans the indexed base table directly.
func (s *Store) ListInteractions(f InteractionFilters) ([]Interaction, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}

	var (
		query strings.Builder
		args  []any
	)

	if strings.TrimSpace(f.Query) != "" {
		query.WriteString(`SELECT i.id, COALESCE(i.session_id,''), i.project, i.team, i.agent, i.editor, i.model,
		                          i.prompt_excerpt, i.response_excerpt,
		                          i.input_tokens, i.output_tokens, i.cache_read, i.cache_creation,
		                          i.duration_ms, i.tool_calls, i.status, i.error_msg, i.estimated,
		                          i.created_at
		                     FROM interactions i
		                     JOIN interactions_fts fts ON fts.rowid = i.rowid
		                    WHERE interactions_fts MATCH ?`)
		args = append(args, f.Query)
	} else {
		query.WriteString(`SELECT id, COALESCE(session_id,''), project, team, agent, editor, model,
		                          prompt_excerpt, response_excerpt,
		                          input_tokens, output_tokens, cache_read, cache_creation,
		                          duration_ms, tool_calls, status, error_msg, estimated,
		                          created_at
		                     FROM interactions WHERE 1=1`)
	}

	if f.Project != "" {
		query.WriteString(" AND project = ?")
		args = append(args, f.Project)
	}
	if f.Model != "" {
		query.WriteString(" AND model = ?")
		args = append(args, f.Model)
	}
	if f.Agent != "" {
		query.WriteString(" AND agent = ?")
		args = append(args, f.Agent)
	}
	if f.Editor != "" {
		query.WriteString(" AND editor = ?")
		args = append(args, f.Editor)
	}
	if f.Status != "" {
		query.WriteString(" AND status = ?")
		args = append(args, f.Status)
	}
	if !f.Since.IsZero() {
		query.WriteString(" AND created_at >= ?")
		args = append(args, f.Since.UTC().Format("2006-01-02 15:04:05"))
	}
	if !f.Until.IsZero() {
		query.WriteString(" AND created_at <= ?")
		args = append(args, f.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	query.WriteString(" ORDER BY created_at DESC LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("listing interactions: %w", err)
	}
	defer rows.Close()

	out := make([]Interaction, 0, f.Limit)
	for rows.Next() {
		in, err := scanInteraction(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *in)
	}
	return out, rows.Err()
}

// CountInteractions returns the total matching `f` without limit/offset.
// FTS queries are not counted (return -1) because COUNT over FTS5 is expensive.
func (s *Store) CountInteractions(f InteractionFilters) (int, error) {
	if strings.TrimSpace(f.Query) != "" {
		return -1, nil
	}

	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`SELECT COUNT(*) FROM interactions WHERE 1=1`)

	if f.Project != "" {
		query.WriteString(" AND project = ?")
		args = append(args, f.Project)
	}
	if f.Model != "" {
		query.WriteString(" AND model = ?")
		args = append(args, f.Model)
	}
	if f.Agent != "" {
		query.WriteString(" AND agent = ?")
		args = append(args, f.Agent)
	}
	if f.Editor != "" {
		query.WriteString(" AND editor = ?")
		args = append(args, f.Editor)
	}
	if f.Status != "" {
		query.WriteString(" AND status = ?")
		args = append(args, f.Status)
	}
	if !f.Since.IsZero() {
		query.WriteString(" AND created_at >= ?")
		args = append(args, f.Since.UTC().Format("2006-01-02 15:04:05"))
	}
	if !f.Until.IsZero() {
		query.WriteString(" AND created_at <= ?")
		args = append(args, f.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	var total int
	if err := s.db.QueryRow(query.String(), args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// TokenStats aggregates token usage across interactions for the Observatory tokens page.
type TokenStats struct {
	InputTokens       int64                       `json:"input_tokens"`
	OutputTokens      int64                       `json:"output_tokens"`
	CacheRead         int64                       `json:"cache_read"`
	CacheCreation     int64                       `json:"cache_creation"`
	InteractionsCount int64                       `json:"interactions_count"`
	EstimatedCount    int64                       `json:"estimated_count"`
	CacheHitPct       float64                     `json:"cache_hit_pct"`
	ByModel           map[string]TokenStatsBucket `json:"by_model"`
	ByProject         map[string]TokenStatsBucket `json:"by_project"`
	Daily             []DailyTokenCount           `json:"daily"`
}

// TokenStatsBucket aggregates totals for one model or one project.
type TokenStatsBucket struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheRead    int64 `json:"cache_read"`
	Count        int64 `json:"count"`
}

// DailyTokenCount is the token totals bucketed per calendar day.
type DailyTokenCount struct {
	Date         string `json:"date"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CacheRead    int64  `json:"cache_read"`
	Estimated    bool   `json:"estimated"`
}

// GetTokenStats returns aggregated token usage for the requested window.
// `from` and `to` may be zero values, in which case the full table is summed.
func (s *Store) GetTokenStats(from, to time.Time) (*TokenStats, error) {
	stats := &TokenStats{
		ByModel:   make(map[string]TokenStatsBucket),
		ByProject: make(map[string]TokenStatsBucket),
	}

	whereClause, args := tokenWindowClause(from, to)

	row := s.db.QueryRow(
		`SELECT
			COALESCE(SUM(input_tokens),0),
			COALESCE(SUM(output_tokens),0),
			COALESCE(SUM(cache_read),0),
			COALESCE(SUM(cache_creation),0),
			COUNT(*),
			COALESCE(SUM(estimated),0)
		   FROM interactions `+whereClause,
		args...,
	)
	if err := row.Scan(
		&stats.InputTokens, &stats.OutputTokens, &stats.CacheRead, &stats.CacheCreation,
		&stats.InteractionsCount, &stats.EstimatedCount,
	); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if denom := stats.InputTokens + stats.CacheRead; denom > 0 {
		stats.CacheHitPct = float64(stats.CacheRead) / float64(denom)
	}

	if err := s.fillTokenBuckets(whereClause, args, "model", stats.ByModel); err != nil {
		return nil, err
	}
	if err := s.fillTokenBuckets(whereClause, args, "project", stats.ByProject); err != nil {
		return nil, err
	}
	daily, err := s.tokenDailyBuckets(whereClause, args)
	if err != nil {
		return nil, err
	}
	stats.Daily = daily

	return stats, nil
}

func (s *Store) fillTokenBuckets(whereClause string, baseArgs []any, groupCol string, dest map[string]TokenStatsBucket) error {
	rows, err := s.db.Query(
		`SELECT `+groupCol+`,
		        COALESCE(SUM(input_tokens),0),
		        COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_read),0),
		        COUNT(*)
		   FROM interactions `+whereClause+`
		  GROUP BY `+groupCol,
		baseArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key string
			b   TokenStatsBucket
		)
		if err := rows.Scan(&key, &b.InputTokens, &b.OutputTokens, &b.CacheRead, &b.Count); err != nil {
			return err
		}
		if key == "" {
			key = "(unknown)"
		}
		dest[key] = b
	}
	return rows.Err()
}

func (s *Store) tokenDailyBuckets(whereClause string, baseArgs []any) ([]DailyTokenCount, error) {
	rows, err := s.db.Query(
		`SELECT substr(created_at, 1, 10) AS day,
		        COALESCE(SUM(input_tokens),0),
		        COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_read),0),
		        MAX(estimated)
		   FROM interactions `+whereClause+`
		  GROUP BY day ORDER BY day ASC`,
		baseArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyTokenCount
	for rows.Next() {
		var d DailyTokenCount
		var est int
		if err := rows.Scan(&d.Date, &d.InputTokens, &d.OutputTokens, &d.CacheRead, &est); err != nil {
			return nil, err
		}
		d.Estimated = est == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

func tokenWindowClause(from, to time.Time) (string, []any) {
	clause := "WHERE 1=1"
	args := []any{}
	if !from.IsZero() {
		clause += " AND created_at >= ?"
		args = append(args, from.UTC().Format("2006-01-02 15:04:05"))
	}
	if !to.IsZero() {
		clause += " AND created_at <= ?"
		args = append(args, to.UTC().Format("2006-01-02 15:04:05"))
	}
	return clause, args
}

// PurgeInteractionsOlderThan deletes interactions older than `cutoff` and returns
// the number of rows removed. Used by the retention enforcer.
func (s *Store) PurgeInteractionsOlderThan(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM interactions WHERE created_at < ?`,
		cutoff.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// scanInteraction reads one row using the provided Scan func (works for both
// sql.Row and sql.Rows).
func scanInteraction(scan func(...any) error) (*Interaction, error) {
	var (
		in        Interaction
		toolCalls string
		createdAt string
		estimated int
	)
	err := scan(
		&in.ID, &in.SessionID, &in.Project, &in.Team, &in.Agent, &in.Editor, &in.Model,
		&in.PromptExcerpt, &in.ResponseExcerpt,
		&in.InputTokens, &in.OutputTokens, &in.CacheRead, &in.CacheCreation,
		&in.DurationMs, &toolCalls, &in.Status, &in.ErrorMsg, &estimated,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}
	in.ToolCalls = json.RawMessage(toolCalls)
	in.Estimated = estimated == 1
	in.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &in, nil
}

// EditorAdoptionRow is one row of the Phase 18.C adoption aggregation:
// editor id + count of interactions in the requested window. Rows with
// `Editor == ""` represent interactions that did not opt in to the
// telemetry header.
type EditorAdoptionRow struct {
	Editor string `json:"editor"`
	Count  int    `json:"count"`
}

// EditorAdoption returns one row per distinct editor (including the
// empty-string anonymous bucket) over the last `since` window.
// Ordered by descending count; small enough to return verbatim.
//
// Drives the Beacon "Editor adoption" widget.
func (s *Store) EditorAdoption(since time.Time) ([]EditorAdoptionRow, error) {
	var sinceStr string
	if !since.IsZero() {
		sinceStr = since.UTC().Format("2006-01-02 15:04:05")
	}
	var (
		rows *sql.Rows
		err  error
	)
	if sinceStr != "" {
		rows, err = s.db.Query(`
			SELECT editor, COUNT(*) AS n
			  FROM interactions
			 WHERE created_at >= ?
			 GROUP BY editor
			 ORDER BY n DESC, editor ASC`, sinceStr)
	} else {
		rows, err = s.db.Query(`
			SELECT editor, COUNT(*) AS n
			  FROM interactions
			 GROUP BY editor
			 ORDER BY n DESC, editor ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("editor adoption: %w", err)
	}
	defer rows.Close()
	var out []EditorAdoptionRow
	for rows.Next() {
		var r EditorAdoptionRow
		if err := rows.Scan(&r.Editor, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
