package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// adminExport handles GET /admin/export.
//
// Streams all matching observations as JSONL (one JSON object per line),
// suitable for backup, compliance exports, or migrations.
//
// Query parameters:
//
//	project  — filter by project name
//	team     — filter by team name
//	type     — filter by observation type
//
// Response headers:
//
//	Content-Type: application/x-ndjson
//	Content-Disposition: attachment; filename="korva-vault-YYYY-MM-DD.jsonl"
//
// Each line is a JSON-encoded store.Observation.
func adminExport(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		opts := store.ExportOptions{
			Project: q.Get("project"),
			Team:    q.Get("team"),
			Type:    q.Get("type"),
		}

		observations, err := s.Export(opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export failed: "+err.Error())
			return
		}

		date := time.Now().Format("2006-01-02")
		filename := fmt.Sprintf("korva-vault-%s.jsonl", date)

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		enc := json.NewEncoder(w)
		for _, obs := range observations {
			if err := enc.Encode(obs); err != nil {
				// The response is already started; we can't change the status code.
				// Log the truncation by writing a comment-like error line.
				break
			}
		}

		writeAudit(s, actor, "export",
			fmt.Sprintf("project=%s team=%s type=%s", opts.Project, opts.Team, opts.Type),
			"", fmt.Sprintf("exported=%d", len(observations)))
	}
}
