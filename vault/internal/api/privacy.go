package api

import (
	"net/http"

	"github.com/alcandev/korva/internal/privacy"
)

// Phase 9.1 — Privacy meter endpoint.
//
//   GET /admin/privacy/stats
//
// Reads the process-wide redaction counters maintained by
// internal/privacy. Drives the Beacon "Privacy meter" panel — operators
// see exactly how much sensitive material the filter has scrubbed and
// which categories appear most often.
//
// No DB roundtrip; pure atomic counter reads. Safe to call every second.

func adminPrivacyStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, privacy.RedactionStats())
	}
}
