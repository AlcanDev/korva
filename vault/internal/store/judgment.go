package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Phase 2 — Conflict judgment workflow.
//
// When an AI agent calls vault_save with a new observation, the Vault now
// surfaces the top-N similar observations in the same project as *candidate
// conflicts*. Each candidate produces a pending row in observation_relations
// (judgment_status='pending'). A subsequent vault_judge call records the
// agent / user verdict and flips the row to 'judged'. A vault_compare call
// short-circuits the pending step entirely for verdicts already adjudicated
// by an external LLM.
//
// All operations are project-scoped — judgments never cross project lines.

// candidateBM25Floor is the minimum FTS5 BM25 score (lower = more relevant
// in SQLite FTS5) below which we still consider an observation a candidate.
// Set conservatively so we surface a useful number of candidates without
// drowning the agent in noise. Tunable via FindRelationCandidatesOpts.
const candidateBM25Floor = -2.0

// FindRelationCandidatesOpts controls how the FTS5 candidate search behaves.
type FindRelationCandidatesOpts struct {
	Limit     int     // 0 -> default 3
	BM25Floor float64 // 0 -> default candidateBM25Floor; lower is stricter (more relevant)
}

// FindRelationCandidates searches the FTS5 index for observations in the same
// project that semantically overlap with the source observation. Returns up to
// `opts.Limit` matches ordered by BM25 relevance, excluding the source itself.
func (s *Store) FindRelationCandidates(sourceID string, opts FindRelationCandidatesOpts) ([]Observation, error) {
	if opts.Limit <= 0 {
		opts.Limit = 3
	}
	if opts.BM25Floor == 0 {
		opts.BM25Floor = candidateBM25Floor
	}

	source, err := s.Get(sourceID)
	if err != nil {
		return nil, fmt.Errorf("loading source observation: %w", err)
	}
	if source == nil {
		return nil, fmt.Errorf("source observation %q not found", sourceID)
	}

	query := buildFTSQueryFromObservation(source)
	if query == "" {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT o.id, COALESCE(o.session_id,''), o.project, o.team, o.country, o.type,
		       o.title, o.content, o.tags, o.author, o.created_at,
		       COALESCE(o.topic_key,''), COALESCE(o.working_dir,'')
		  FROM observations_fts
		  JOIN observations o ON o.rowid = observations_fts.rowid
		 WHERE observations_fts MATCH ?
		   AND o.project = ?
		   AND o.id != ?
		   AND bm25(observations_fts) >= ?
		 ORDER BY bm25(observations_fts) ASC
		 LIMIT ?`,
		query, source.Project, sourceID, opts.BM25Floor, opts.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("FTS candidate search: %w", err)
	}
	defer rows.Close()

	var out []Observation
	for rows.Next() {
		obs, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *obs)
	}
	return out, rows.Err()
}

// buildFTSQueryFromObservation extracts a small set of significant tokens from
// the title — enough to drive the BM25 ranker without forcing exact-text
// matching. Title-only keeps the query short and on-topic; content tends to
// pull in too many incidental hits.
func buildFTSQueryFromObservation(obs *Observation) string {
	if obs == nil {
		return ""
	}
	// FTS5 query language: 'word1 OR word2 OR word3' is permissive enough to
	// let the ranker do its job. We strip very short tokens (≤2 chars) and
	// quote each word so punctuation in the title does not break the parser.
	fields := strings.Fields(normalizeForHash(obs.Title))
	parts := make([]string, 0, len(fields))
	for _, w := range fields {
		if len(w) < 3 {
			continue
		}
		// Escape any embedded quotes; FTS5 phrase quoting handles the rest.
		parts = append(parts, `"`+strings.ReplaceAll(w, `"`, ``)+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}

// CreatePendingJudgments inserts pending observation_relations rows linking
// `sourceID` to every candidate ID. Idempotent: if a relation already exists
// for the pair (in any judgment_status), the row is left as-is. Returns the
// IDs of newly-created pending rows so callers can surface them to the agent.
func (s *Store) CreatePendingJudgments(sourceID string, candidates []Observation) ([]string, error) {
	if sourceID == "" || len(candidates) == 0 {
		return nil, nil
	}
	source, err := s.Get(sourceID)
	if err != nil || source == nil {
		return nil, fmt.Errorf("source observation %q not found", sourceID)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var created []string
	now := time.Now().UTC().Format(time.RFC3339)
	for _, c := range candidates {
		if c.ID == sourceID || c.Project != source.Project {
			continue
		}
		id := newID()
		res, err := tx.Exec(`
			INSERT INTO observation_relations
			  (id, source_id, target_id, relation, status, reason, author, project, created_at,
			   judgment_status, confidence, evidence, marked_by_actor, marked_by_kind, marked_by_model)
			VALUES (?, ?, ?, '', 'pending', '', '', ?, ?, 'pending', 0.0, '', 'admin', 'heuristic', '')
			ON CONFLICT(source_id, target_id) DO NOTHING`,
			id, sourceID, c.ID, source.Project, now,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting pending judgment for %s↔%s: %w", sourceID, c.ID, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			created = append(created, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit pending judgments: %w", err)
	}
	return created, nil
}

// JudgeInput captures the verdict recorded by vault_judge.
type JudgeInput struct {
	Relation      RelationType
	Reason        string
	Evidence      string
	Confidence    float64
	MarkedByActor ActorKind
	MarkedByKind  VerdictKind
	MarkedByModel string
	SessionID     string
}

// Validate enforces the contract before we touch the DB. Keeping it close to
// the input struct means every entry path (API, MCP, CLI) gets the same rules.
func (in *JudgeInput) Validate() error {
	if in == nil {
		return fmt.Errorf("judgment input is nil")
	}
	if in.Relation == "" {
		return fmt.Errorf("relation is required")
	}
	known := false
	for _, t := range AllRelationTypes {
		if string(in.Relation) == t {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown relation %q (allowed: %v)", in.Relation, AllRelationTypes)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return fmt.Errorf("confidence must be in [0,1], got %g", in.Confidence)
	}
	switch in.MarkedByActor {
	case "", ActorAgent, ActorUser, ActorAdmin:
	default:
		return fmt.Errorf("unknown marked_by_actor %q", in.MarkedByActor)
	}
	switch in.MarkedByKind {
	case "", VerdictHeuristic, VerdictLLM, VerdictManual:
	default:
		return fmt.Errorf("unknown marked_by_kind %q", in.MarkedByKind)
	}
	return nil
}

// Judge records the verdict on a pending judgment row, flipping it to
// judgment_status='judged'. Returns ErrJudgmentNotFound if the row does not
// exist or has already been judged.
var ErrJudgmentNotFound = fmt.Errorf("judgment not found or already resolved")

func (s *Store) Judge(judgmentID string, in JudgeInput) (*Relation, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	actor := in.MarkedByActor
	if actor == "" {
		actor = ActorAgent
	}
	kind := in.MarkedByKind
	if kind == "" {
		kind = VerdictHeuristic
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		UPDATE observation_relations
		   SET relation = ?,
		       status = 'confirmed',
		       reason = ?,
		       evidence = ?,
		       confidence = ?,
		       judgment_status = 'judged',
		       marked_by_actor = ?,
		       marked_by_kind = ?,
		       marked_by_model = ?,
		       session_id = ?,
		       judged_at = ?
		 WHERE id = ? AND judgment_status = 'pending'`,
		string(in.Relation), in.Reason, in.Evidence, in.Confidence,
		string(actor), string(kind), in.MarkedByModel,
		nullString(in.SessionID), now, judgmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating judgment: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrJudgmentNotFound
	}

	rows, err := s.db.Query(relationSelectClause+` WHERE id = ?`, judgmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("judged row vanished mid-update")
	}
	return scanRelation(rows)
}

// IgnoreJudgment closes a pending judgment as "not actually a conflict". The
// row stays in the table for audit but is hidden from default listings.
func (s *Store) IgnoreJudgment(judgmentID, reason, sessionID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		UPDATE observation_relations
		   SET judgment_status = 'ignored',
		       reason = ?,
		       session_id = ?,
		       judged_at = ?,
		       marked_by_actor = 'user',
		       marked_by_kind = 'manual'
		 WHERE id = ? AND judgment_status = 'pending'`,
		reason, nullString(sessionID), now, judgmentID,
	)
	if err != nil {
		return fmt.Errorf("ignoring judgment: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrJudgmentNotFound
	}
	return nil
}

// ListPendingJudgments returns pending judgments scoped to a project. An empty
// project returns global pending judgments — useful for admin dashboards.
func (s *Store) ListPendingJudgments(project string, limit int) ([]Relation, error) {
	return s.ListJudgmentsByStatus(project, JudgmentPending, limit)
}

// ListJudgmentsByStatus is the generic project-scoped listing used by the
// admin / CLI surfaces that need to inspect judged/orphaned/ignored rows in
// addition to the default pending listing.
func (s *Store) ListJudgmentsByStatus(project string, status JudgmentStatus, limit int) ([]Relation, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if status == "" {
		status = JudgmentPending
	}
	known := false
	for _, allowed := range AllJudgmentStatuses {
		if string(status) == allowed {
			known = true
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("unknown judgment status %q (allowed: %v)", status, AllJudgmentStatuses)
	}

	args := []any{string(status)}
	clause := ` WHERE judgment_status = ?`
	if project != "" {
		clause += ` AND project = ?`
		args = append(args, project)
	}
	clause += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(relationSelectClause+clause, args...)
	if err != nil {
		return nil, fmt.Errorf("listing judgments: %w", err)
	}
	defer rows.Close()
	var out []Relation
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetJudgment returns a single relation row by ID (any judgment_status).
func (s *Store) GetJudgment(id string) (*Relation, error) {
	rows, err := s.db.Query(relationSelectClause+` WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanRelation(rows)
}

// CompareInput captures an LLM-evaluated comparison between two observations
// that the agent feeds back to Vault via vault_compare. Idempotent — re-calls
// with the same (source, target) pair update the existing row.
type CompareInput struct {
	SourceID      string
	TargetID      string
	Relation      RelationType
	Reason        string
	Evidence      string
	Confidence    float64
	MarkedByActor ActorKind
	MarkedByModel string
	SessionID     string
}

// CompareAndStore upserts an already-adjudicated comparison. The resulting row
// is stored with judgment_status='judged' directly — there is no pending step
// because the actor has already produced a verdict.
func (s *Store) CompareAndStore(in CompareInput) (string, error) {
	if in.Relation == "" {
		return "", fmt.Errorf("relation is required")
	}
	known := false
	for _, t := range AllRelationTypes {
		if string(in.Relation) == t {
			known = true
			break
		}
	}
	if !known {
		return "", fmt.Errorf("unknown relation %q", in.Relation)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return "", fmt.Errorf("confidence must be in [0,1], got %g", in.Confidence)
	}
	source, err := s.Get(in.SourceID)
	if err != nil || source == nil {
		return "", fmt.Errorf("source observation %q not found", in.SourceID)
	}
	target, err := s.Get(in.TargetID)
	if err != nil || target == nil {
		return "", fmt.Errorf("target observation %q not found", in.TargetID)
	}
	if source.Project != target.Project {
		return "", fmt.Errorf("cross-project comparisons are not allowed")
	}

	actor := in.MarkedByActor
	if actor == "" {
		actor = ActorAgent
	}
	id := newID()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		INSERT INTO observation_relations
		  (id, source_id, target_id, relation, status, reason, author, project, created_at,
		   judgment_status, confidence, evidence, marked_by_actor, marked_by_kind, marked_by_model,
		   session_id, judged_at)
		VALUES (?, ?, ?, ?, 'confirmed', ?, ?, ?, ?, 'judged', ?, ?, ?, 'llm', ?, ?, ?)
		ON CONFLICT(source_id, target_id) DO UPDATE SET
		    relation        = excluded.relation,
		    reason          = excluded.reason,
		    evidence        = excluded.evidence,
		    confidence      = excluded.confidence,
		    judgment_status = 'judged',
		    marked_by_actor = excluded.marked_by_actor,
		    marked_by_kind  = excluded.marked_by_kind,
		    marked_by_model = excluded.marked_by_model,
		    session_id      = excluded.session_id,
		    judged_at       = excluded.judged_at`,
		id, in.SourceID, in.TargetID, string(in.Relation), in.Reason, "", source.Project, now,
		in.Confidence, in.Evidence, string(actor), in.MarkedByModel,
		nullString(in.SessionID), now,
	)
	if err != nil {
		return "", fmt.Errorf("upserting comparison: %w", err)
	}

	// Return the canonical ID — when the upsert hit an existing row we still
	// want to surface its primary key so callers can fetch the latest state.
	var got string
	if err := s.db.QueryRow(
		`SELECT id FROM observation_relations WHERE source_id = ? AND target_id = ?`,
		in.SourceID, in.TargetID,
	).Scan(&got); err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if got != "" {
		return got, nil
	}
	return id, nil
}

