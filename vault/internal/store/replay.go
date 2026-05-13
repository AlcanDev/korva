package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Phase 10.3 — query helpers used by the session-replay endpoint.
//
// Three small reads:
//   - GetSession: lookup one session by id (returns nil, nil if missing)
//   - ListObservationsBySession: every observation tied to this session_id
//   - ListInteractionsBySession: every interaction tied to this session_id
//
// Each is a thin SQL wrapper — the replay handler stitches them into a
// single chronological timeline.

// GetSession returns the session row with the given id, or (nil, nil) when
// the id doesn't exist. Errors are real DB problems only.
func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`
		SELECT id, project, team, country, agent, goal, COALESCE(summary, ''),
		       started_at, ended_at
		FROM sessions
		WHERE id = ?`, id)

	var sess Session
	var startedAt string
	var endedAtNull sql.NullString
	err := row.Scan(
		&sess.ID, &sess.Project, &sess.Team, &sess.Country,
		&sess.Agent, &sess.Goal, &sess.Summary,
		&startedAt, &endedAtNull,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	sess.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if endedAtNull.Valid {
		t, _ := time.Parse(time.RFC3339, endedAtNull.String)
		sess.EndedAt = &t
	}
	return &sess, nil
}

// ListObservationsBySession returns every observation whose session_id
// matches. Ordered ascending by creation time so the replay timeline reads
// naturally.
func (s *Store) ListObservationsBySession(sessionID string) ([]Observation, error) {
	rows, err := s.db.Query(`
		SELECT o.id, COALESCE(o.session_id, ''), o.project, o.team, o.country,
		       o.type, o.title, o.content, o.tags, o.author, o.created_at,
		       COALESCE(o.topic_key,''), COALESCE(o.working_dir,'')
		FROM observations o
		WHERE o.session_id = ?
		ORDER BY o.created_at ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list observations by session: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanObservations(rows)
}

// ListInteractionsBySession returns every interaction whose session_id
// matches, ordered ascending by creation time.
func (s *Store) ListInteractionsBySession(sessionID string) ([]Interaction, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(session_id, ''), project, COALESCE(team, ''),
		       agent, model, prompt_excerpt, COALESCE(response_excerpt, ''),
		       input_tokens, output_tokens, cache_read, cache_creation,
		       duration_ms, COALESCE(tool_calls, '[]'), status,
		       COALESCE(error_msg, ''), estimated, created_at
		FROM interactions
		WHERE session_id = ?
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list interactions by session: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Interaction
	for rows.Next() {
		var in Interaction
		var createdAt string
		var toolCalls []byte
		var estimated int
		err := rows.Scan(
			&in.ID, &in.SessionID, &in.Project, &in.Team,
			&in.Agent, &in.Model, &in.PromptExcerpt, &in.ResponseExcerpt,
			&in.InputTokens, &in.OutputTokens, &in.CacheRead, &in.CacheCreation,
			&in.DurationMs, &toolCalls, &in.Status,
			&in.ErrorMsg, &estimated, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		in.ToolCalls = toolCalls
		in.Estimated = estimated == 1
		in.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, in)
	}
	return out, rows.Err()
}
