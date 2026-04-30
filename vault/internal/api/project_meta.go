package api

// project_meta.go exposes two lightweight REST endpoints:
//
//   GET/PUT /api/v1/openspec/{project}
//     Project conventions (stack, rules, test standards) stored as freeform text.
//     Reading is public; writing requires X-Admin-Key.
//     Stores per-project specification metadata.
//
//   GET/PUT /api/v1/sdd/{project}
//     Spec-Driven Development phase tracker per project.
//     Reading is public; writing requires X-Admin-Key or X-Session-Token.
//     Tracks the current Spec-Driven Development phase for a project.

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

// ── OpenSpec ─────────────────────────────────────────────────────────────────

func getOpenSpec(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		spec, err := s.GetOpenSpec(project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, spec)
	}
}

func putOpenSpec(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: expected {\"content\": \"...\"})")
			return
		}
		if err := s.SaveOpenSpec(project, body.Content); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "project": project})
	}
}

// ── SDD phase ─────────────────────────────────────────────────────────────────

func getSDDPhase(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		state, err := s.GetSDDPhase(project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project":    state.Project,
			"phase":      state.Phase,
			"updated_at": state.UpdatedAt,
			"all_phases": store.AllSDDPhases,
		})
	}
}

func putSDDPhase(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var body struct {
			Phase string `json:"phase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Phase == "" {
			writeError(w, http.StatusBadRequest, "phase is required")
			return
		}
		// Validate phase.
		valid := false
		for _, p := range store.AllSDDPhases {
			if p == body.Phase {
				valid = true
				break
			}
		}
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid phase — valid: explore, propose, spec, design, tasks, apply, verify, archive, onboard")
			return
		}
		if err := s.SetSDDPhase(project, store.SDDPhase(body.Phase)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "phase": body.Phase})
	}
}
