package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

// adminGetIntegrity handles GET /admin/integrity — runs the read-only doctor
// probes and returns the report. Backs the `korva doctor` integrity section
// and the Observatory "System Health → Integrity" tile.
func adminGetIntegrity(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := s.DiagnoseIntegrity()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

// repairIntegrityRequest is the wire shape for POST /admin/integrity/repair.
type repairIntegrityRequest struct {
	Mode                  string   `json:"mode"` // plan | dry_run | apply
	Operations            []string `json:"operations,omitempty"`
	SnapshotRetentionDays int      `json:"snapshot_retention_days,omitempty"`
}

// adminRepairIntegrity handles POST /admin/integrity/repair. The request body
// selects which operations to run and the desired mode; the response carries
// per-operation estimated/applied row counts plus any per-op errors. Errors
// in a single operation do not fail the whole request — the caller sees the
// complete picture, which matches the doctor UX users expect.
func adminRepairIntegrity(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req repairIntegrityRequest
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
		}
		mode := store.RepairMode(req.Mode)
		if mode == "" {
			mode = store.RepairModePlan
		}
		report, err := s.RepairIntegrity(store.RepairOptions{
			Mode:                  mode,
			Operations:            req.Operations,
			SnapshotRetentionDays: req.SnapshotRetentionDays,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}
