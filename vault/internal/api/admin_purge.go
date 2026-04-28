package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

type purgeRequest struct {
	Project string `json:"project"`
	Team    string `json:"team"`
	Type    string `json:"type"`
	// Before is an RFC3339 timestamp; observations created strictly before this
	// time are candidates for deletion.
	Before string `json:"before"`
	// DryRun returns the count without deleting anything.
	DryRun bool `json:"dry_run"`
}

// adminPurgeHandler handles POST /admin/purge.
//
// At least one filter (project, team, type, or before) is required to prevent
// accidental full-table wipes. The dry_run flag lets operators preview the
// impact before committing.
//
// Response:
//
//	{"deleted": 42, "dry_run": false}
func adminPurgeHandler(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body purgeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		opts := store.PurgeOptions{
			Project: body.Project,
			Team:    body.Team,
			Type:    body.Type,
			DryRun:  body.DryRun,
		}
		if body.Before != "" {
			t, err := time.Parse(time.RFC3339, body.Before)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid 'before' date — use RFC3339 (e.g. 2026-01-01T00:00:00Z)")
				return
			}
			opts.Before = t
		}

		n, err := s.Purge(opts)
		if err != nil {
			// Purge returns a user-friendly error when no filter is supplied.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if !body.DryRun {
			writeAudit(s, actor, "purge",
				fmt.Sprintf("project=%s team=%s type=%s before=%s", body.Project, body.Team, body.Type, body.Before),
				"", fmt.Sprintf("deleted=%d", n))
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"deleted": n,
			"dry_run": body.DryRun,
			"project": body.Project,
			"team":    body.Team,
			"type":    body.Type,
			"before":  body.Before,
		})
	}
}
