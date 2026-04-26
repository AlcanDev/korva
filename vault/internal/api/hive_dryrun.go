package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/privacy/cloud"
)

// hiveDryRunHandler handles GET /admin/hive/dry-run.
//
// Shows which pending outbox rows would be sent vs. rejected by the privacy
// filter on the next sync — without actually sending anything. Useful for
// verifying privacy filter behavior before enabling Hive sync.
//
// Requires admin key.
func hiveDryRunHandler(outbox *hive.Outbox, filter *cloud.Filter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if outbox == nil || filter == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"dry_run": true,
				"enabled": false,
				"message": "Hive sync is not configured on this instance",
			})
			return
		}

		rows, err := outbox.NextBatch(100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read outbox"})
			return
		}

		type rowPreview struct {
			ObservationID string `json:"observation_id"`
			Decision      string `json:"decision"` // "send" | "reject"
			Reason        string `json:"reason,omitempty"`
		}

		var previews []rowPreview
		sendCount, rejectCount := 0, 0

		for _, row := range rows {
			var input cloud.Input
			if err := json.Unmarshal(row.Payload, &input); err != nil {
				previews = append(previews, rowPreview{
					ObservationID: row.ObservationID,
					Decision:      "reject",
					Reason:        "payload parse error",
				})
				rejectCount++
				continue
			}
			_, dec, reason := filter.Process(input)
			p := rowPreview{ObservationID: row.ObservationID}
			if dec == cloud.Reject {
				p.Decision = "reject"
				p.Reason = reason
				rejectCount++
			} else {
				p.Decision = "send"
				sendCount++
			}
			previews = append(previews, p)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"dry_run":       true,
			"pending_total": len(rows),
			"would_send":    sendCount,
			"would_reject":  rejectCount,
			"rows":          previews,
		})
	}
}
