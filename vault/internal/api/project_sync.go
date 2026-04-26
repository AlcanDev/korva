package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/internal/hive"
)

// projectSyncHandlers returns handlers for per-project Hive sync controls.
// All handlers require an admin key (caller must wrap with adminMW).

// listProjectSyncControls handles GET /admin/hive/projects.
func listProjectSyncControls(outbox *hive.Outbox) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if outbox == nil {
			writeJSON(w, http.StatusOK, map[string]any{"controls": []any{}, "hive_enabled": false})
			return
		}
		controls, err := outbox.ListProjectSyncControls()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list controls"})
			return
		}
		if controls == nil {
			controls = []hive.ProjectSyncControl{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"controls": controls})
	}
}

// pauseProjectSync handles POST /admin/hive/projects/{project}/pause.
func pauseProjectSync(outbox *hive.Outbox) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if outbox == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "hive not configured"})
			return
		}
		project := r.PathValue("project")
		if project == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
			return
		}

		var body struct {
			PausedBy string `json:"paused_by"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			body.PausedBy = "admin"
		}
		if body.PausedBy == "" {
			body.PausedBy = "admin"
		}

		if err := outbox.PauseProjectSync(project, body.PausedBy, body.Reason); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to pause project"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project":      project,
			"sync_enabled": false,
			"paused_by":    body.PausedBy,
			"reason":       body.Reason,
		})
	}
}

// resumeProjectSync handles POST /admin/hive/projects/{project}/resume.
func resumeProjectSync(outbox *hive.Outbox) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if outbox == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "hive not configured"})
			return
		}
		project := r.PathValue("project")
		if project == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
			return
		}

		var body struct {
			ResumedBy string `json:"resumed_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			body.ResumedBy = "admin"
		}
		if body.ResumedBy == "" {
			body.ResumedBy = "admin"
		}

		if err := outbox.ResumeProjectSync(project, body.ResumedBy); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resume project"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project":      project,
			"sync_enabled": true,
			"resumed_by":   body.ResumedBy,
		})
	}
}
