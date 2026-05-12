package store

import (
	"database/sql"
	"fmt"
	"time"
)

// AddRelation creates a directed semantic link between two observations.
// If a relation between the same pair already exists it is replaced (upsert).
// Cross-project relations are rejected.
func (s *Store) AddRelation(sourceID, targetID string, rel RelationType, reason, author string) (*Relation, error) {
	// Validate relation type.
	valid := false
	for _, rt := range AllRelationTypes {
		if string(rel) == rt {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid relation type %q: must be one of %v", rel, AllRelationTypes)
	}

	// Fetch both observations to validate existence and project consistency.
	src, err := s.Get(sourceID)
	if err != nil || src == nil {
		return nil, fmt.Errorf("source observation %q not found", sourceID)
	}
	tgt, err := s.Get(targetID)
	if err != nil || tgt == nil {
		return nil, fmt.Errorf("target observation %q not found", targetID)
	}
	if src.Project != tgt.Project {
		return nil, fmt.Errorf("cross-project relations are not allowed (source=%q, target=%q)", src.Project, tgt.Project)
	}

	id := newID()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		INSERT INTO observation_relations (id, source_id, target_id, relation, status, reason, author, project, created_at)
		VALUES (?, ?, ?, ?, 'confirmed', ?, ?, ?, ?)
		ON CONFLICT(source_id, target_id) DO UPDATE SET
			relation   = excluded.relation,
			reason     = excluded.reason,
			author     = excluded.author,
			status     = 'confirmed',
			created_at = excluded.created_at`,
		id, sourceID, targetID, string(rel), reason, author, src.Project, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting relation: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, now)
	return &Relation{
		ID:        id,
		SourceID:  sourceID,
		TargetID:  targetID,
		Relation:  rel,
		Status:    "confirmed",
		Reason:    reason,
		Author:    author,
		Project:   src.Project,
		CreatedAt: createdAt,
	}, nil
}

// GetRelations returns all relations for a given observation ID (both directions).
func (s *Store) GetRelations(observationID string) (*ObservationRelations, error) {
	result := &ObservationRelations{}

	rows, err := s.db.Query(relationSelectClause+`
		WHERE source_id = ? OR target_id = ?
		ORDER BY created_at DESC`, observationID, observationID)
	if err != nil {
		return nil, fmt.Errorf("querying relations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		if r.SourceID == observationID {
			result.AsSource = append(result.AsSource, *r)
		} else {
			result.AsTarget = append(result.AsTarget, *r)
		}
	}
	return result, rows.Err()
}

// ListRelationsByProject returns all relations for a project, optionally filtered by type.
func (s *Store) ListRelationsByProject(project string, relType RelationType) ([]Relation, error) {
	args := []any{project}
	typeFilter := ""
	if relType != "" {
		typeFilter = " AND relation = ?"
		args = append(args, string(relType))
	}

	rows, err := s.db.Query(relationSelectClause+`
		WHERE project = ?`+typeFilter+`
		ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("querying project relations: %w", err)
	}
	defer rows.Close()

	var result []Relation
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *r)
	}
	return result, rows.Err()
}

// DeleteRelation removes a relation by ID. Returns (true, nil) if deleted.
func (s *Store) DeleteRelation(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM observation_relations WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// relationSelectClause centralises the column list so every callsite reads the
// same fields in the same order. Keeps scanRelation portable.
const relationSelectClause = `
	SELECT id, source_id, target_id, relation, status,
	       COALESCE(reason,''), COALESCE(author,''), project, created_at,
	       judgment_status, confidence, COALESCE(evidence,''),
	       marked_by_actor, marked_by_kind, COALESCE(marked_by_model,''),
	       COALESCE(session_id,''), judged_at
	  FROM observation_relations
`

func scanRelation(row *sql.Rows) (*Relation, error) {
	var r Relation
	var createdAt string
	var judgedAt sql.NullString
	if err := row.Scan(
		&r.ID, &r.SourceID, &r.TargetID, &r.Relation, &r.Status,
		&r.Reason, &r.Author, &r.Project, &createdAt,
		&r.JudgmentStatus, &r.Confidence, &r.Evidence,
		&r.MarkedByActor, &r.MarkedByKind, &r.MarkedByModel,
		&r.SessionID, &judgedAt,
	); err != nil {
		return nil, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if judgedAt.Valid && judgedAt.String != "" {
		if t, err := time.Parse(time.RFC3339, judgedAt.String); err == nil {
			r.JudgedAt = &t
		}
	}
	return &r, nil
}
