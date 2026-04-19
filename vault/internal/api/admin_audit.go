package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

type auditRow struct {
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	BeforeHash string `json:"before_hash"`
	AfterHash  string `json:"after_hash"`
	CreatedAt  string `json:"created_at"`
}

func adminListAudit(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, actor, action, target, before_hash, after_hash, created_at
			   FROM audit_logs ORDER BY created_at DESC LIMIT ?`, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		var logs []auditRow
		for rows.Next() {
			var a auditRow
			if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target,
				&a.BeforeHash, &a.AfterHash, &a.CreatedAt); err != nil {
				continue
			}
			logs = append(logs, a)
		}
		if logs == nil {
			logs = []auditRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "count": len(logs)})
	}
}

// writeAudit appends an audit log entry. Errors are silently dropped —
// audit failures must not block the primary operation.
func writeAudit(s *store.Store, actor, action, target, beforeHash, afterHash string) {
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(actor+action+target)))[:16]
	_, _ = s.DB().Exec(
		`INSERT INTO audit_logs(id, actor, action, target, before_hash, after_hash) VALUES(?,?,?,?,?,?)`,
		id, actor, action, target, beforeHash, afterHash)
}

// hashStr returns a short SHA-256 hash of s for audit before/after values.
func hashStr(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:12]
}
