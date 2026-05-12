package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 2 — Conflict judgment endpoints.
//
//   GET    /admin/conflicts                       list pending judgments
//   GET    /admin/conflicts/{id}                  detail (judgment + source + target)
//   POST   /admin/conflicts/{id}/judge            record verdict
//   POST   /admin/conflicts/{id}/ignore           dismiss as not-a-conflict
//   POST   /admin/conflicts/compare               upsert an LLM-evaluated verdict
//
// All routes require X-Admin-Key. Bodies are JSON. The detail endpoint
// embeds the full source/target observations so Beacon can render a side-
// by-side without a second round-trip.

// adminListConflicts handles GET /admin/conflicts.
//
// Query params:
//   - status   (pending|judged|orphaned|ignored, default pending)
//   - project  (optional; empty → all projects)
//   - limit    (default 50, max 500)
func adminListConflicts(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		project := q.Get("project")
		status := q.Get("status")
		if status == "" {
			status = string(store.JudgmentPending)
		}
		limit := 50
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}

		rels, err := s.ListJudgmentsByStatus(project, store.JudgmentStatus(status), limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"conflicts": rels,
			"count":     len(rels),
			"status":    status,
			"project":   project,
		})
	}
}

// adminGetConflict handles GET /admin/conflicts/{id} — returns the relation
// plus both observations so a UI can render a side-by-side diff.
func adminGetConflict(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		j, err := s.GetJudgment(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if j == nil {
			writeError(w, http.StatusNotFound, "conflict not found")
			return
		}
		source, _ := s.Get(j.SourceID)
		target, _ := s.Get(j.TargetID)
		writeJSON(w, http.StatusOK, map[string]any{
			"conflict": j,
			"source":   source,
			"target":   target,
		})
	}
}

// judgeRequest is the wire shape for POST /admin/conflicts/{id}/judge.
type judgeRequest struct {
	Relation      string  `json:"relation"`
	Reason        string  `json:"reason,omitempty"`
	Evidence      string  `json:"evidence,omitempty"`
	Confidence    float64 `json:"confidence"`
	MarkedByActor string  `json:"marked_by_actor,omitempty"`
	MarkedByKind  string  `json:"marked_by_kind,omitempty"`
	MarkedByModel string  `json:"marked_by_model,omitempty"`
	SessionID     string  `json:"session_id,omitempty"`
}

func adminJudgeConflict(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req judgeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		got, err := s.Judge(id, store.JudgeInput{
			Relation:      store.RelationType(req.Relation),
			Reason:        req.Reason,
			Evidence:      req.Evidence,
			Confidence:    req.Confidence,
			MarkedByActor: store.ActorKind(req.MarkedByActor),
			MarkedByKind:  store.VerdictKind(req.MarkedByKind),
			MarkedByModel: req.MarkedByModel,
			SessionID:     req.SessionID,
		})
		if errors.Is(err, store.ErrJudgmentNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// ignoreRequest is the wire shape for POST /admin/conflicts/{id}/ignore.
type ignoreRequest struct {
	Reason    string `json:"reason,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func adminIgnoreConflict(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req ignoreRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		err := s.IgnoreJudgment(id, req.Reason, req.SessionID)
		if errors.Is(err, store.ErrJudgmentNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "id": id})
	}
}

// compareRequest is the wire shape for POST /admin/conflicts/compare.
type compareRequest struct {
	SourceID      string  `json:"source_id"`
	TargetID      string  `json:"target_id"`
	Relation      string  `json:"relation"`
	Reason        string  `json:"reason,omitempty"`
	Evidence      string  `json:"evidence,omitempty"`
	Confidence    float64 `json:"confidence"`
	MarkedByActor string  `json:"marked_by_actor,omitempty"`
	MarkedByModel string  `json:"marked_by_model,omitempty"`
	SessionID     string  `json:"session_id,omitempty"`
}

func adminCompareConflict(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req compareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := s.CompareAndStore(store.CompareInput{
			SourceID:      req.SourceID,
			TargetID:      req.TargetID,
			Relation:      store.RelationType(req.Relation),
			Reason:        req.Reason,
			Evidence:      req.Evidence,
			Confidence:    req.Confidence,
			MarkedByActor: store.ActorKind(req.MarkedByActor),
			MarkedByModel: req.MarkedByModel,
			SessionID:     req.SessionID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		got, _ := s.GetJudgment(id)
		writeJSON(w, http.StatusCreated, got)
	}
}
