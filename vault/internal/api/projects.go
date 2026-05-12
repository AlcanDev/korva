package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 4 — project hygiene endpoints.
//
//   GET    /admin/projects                       list with ObservationCount + SessionCount
//   GET    /admin/projects/suggestions           consolidation suggestions (normalize-form groups)
//   POST   /admin/projects/consolidate           body: {canonical, sources:[]}
//   POST   /admin/projects/prune                 body: {apply: bool}
//
// All routes require X-Admin-Key. Suggestions + dry-run lookups are
// read-only; consolidate + prune mutate the store.

// adminListProjects handles GET /admin/projects.
func adminListProjects(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := s.ListProjects()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"projects": projects,
			"count":    len(projects),
		})
	}
}

// adminSuggestConsolidations handles GET /admin/projects/suggestions.
func adminSuggestConsolidations(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		proposals, err := s.SuggestConsolidations()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"proposals": proposals,
			"count":     len(proposals),
		})
	}
}

// consolidateRequest is the wire shape for POST /admin/projects/consolidate.
type consolidateRequest struct {
	Canonical string   `json:"canonical"`
	Sources   []string `json:"sources"`
}

func adminConsolidateProjects(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req consolidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Canonical == "" {
			writeError(w, http.StatusBadRequest, "canonical is required")
			return
		}
		if len(req.Sources) == 0 {
			writeError(w, http.StatusBadRequest, "at least one source is required")
			return
		}
		obsN, sessN, promptsN, err := s.MergeProjects(req.Sources, req.Canonical)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":              "merged",
			"canonical":           req.Canonical,
			"sources":             req.Sources,
			"observations_updated": obsN,
			"sessions_updated":     sessN,
			"prompts_updated":      promptsN,
		})
	}
}

// pruneRequest is the wire shape for POST /admin/projects/prune.
type pruneRequest struct {
	Apply bool `json:"apply"`
}

func adminPruneProjects(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pruneRequest
		// Empty body is fine — dry-run default.
		_ = json.NewDecoder(r.Body).Decode(&req)
		report, err := s.PruneEmptyProjects(store.PruneOptions{Apply: req.Apply})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}
