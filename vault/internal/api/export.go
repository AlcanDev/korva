package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 5 — Obsidian export admin endpoint.
//
//   POST /admin/export/obsidian   body: {out, project?, type?}
//
// The handler delegates straight to store.ExportObsidian. The CLI is the
// expected caller; the admin key requirement (enforced by the router) means
// callers can only write to paths the operator already controls.

type obsidianExportRequest struct {
	Out     string `json:"out"`
	Project string `json:"project"`
	Type    string `json:"type"`
}

func adminExportObsidian(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req obsidianExportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Out == "" {
			writeError(w, http.StatusBadRequest, "out is required")
			return
		}
		res, err := s.ExportObsidian(req.Out, store.ObsidianExportOptions{
			Project: req.Project,
			Type:    req.Type,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
