package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 14.2 — REST surface for the harness state mirror that Phase
// 14.1 introduced. Beacon's harness dashboard consumes these.
//
// Security model (max-strict):
//   - Every route sits behind withSession (X-Session-Token + active
//     Teams license). No anonymous access.
//   - Every read is filtered by the session's team_id. Cross-team
//     access produces a 404 indistinguishable from "doesn't exist" so
//     attackers can't enumerate other teams' projects.
//   - Path / query parameters are length-capped and rejected when they
//     contain control characters (NUL, newlines, …) — defensive against
//     malformed clients and log-injection attempts.
//   - Transition listings clamp limit to [1, 1000] to bound payload
//     size; defaults to 100 when absent or invalid.
//   - Every read is audit-logged via writeAudit so the operator has a
//     trail of who fetched what (defensible under "max security" mandate
//     — overhead is negligible since audit writes are best-effort).
//   - JSON responses are written via writeJSON which sets explicit
//     Content-Type and never streams unbounded payloads.

// Length caps for user-supplied identifiers. Project / root come from
// the harness CLI's `project` arg + filesystem path — both are bounded
// in practice by SQLite's row size, but we cap them at 200 chars for
// defense-in-depth.
const harnessIdentifierMax = 200

// validHarnessIdentifier returns true when s fits within the length cap
// and contains no control characters (defense against log-injection +
// malformed clients).
func validHarnessIdentifier(s string) bool {
	if s == "" || len(s) > harnessIdentifierMax {
		return false
	}
	for _, r := range s {
		// Reject NUL, newlines, tabs, and other C0 control codes. ASCII
		// printable + extended UTF-8 letters / numbers / hyphens / dots
		// / slashes (for filesystem paths) are all allowed.
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// harnessListProjects: GET /api/v1/harness/projects
//
// Returns the dashboard roll-up scoped to the caller's team — one row
// per (project, root) with the latest transition's target status
// joined. Empty when the team has never persisted a snapshot.
func harnessListProjects(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		summaries, err := s.ListHarnessProjectSummariesForTeam(sess.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeAudit(s, sess.email, "harness_list_projects", sess.teamID, "", "")
		writeJSON(w, http.StatusOK, map[string]any{
			"projects": summaries,
			"count":    len(summaries),
		})
	}
}

// harnessGetProject: GET /api/v1/harness/projects/{project}?root=...
//
// Returns the full snapshot payload for one (project, root) pair owned
// by the caller's team. Returns 404 (not 403) when the row doesn't
// belong to the team — anti-enumeration.
func harnessGetProject(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		project := r.PathValue("project")
		root := r.URL.Query().Get("root")

		if !validHarnessIdentifier(project) {
			writeError(w, http.StatusBadRequest, "invalid project name")
			return
		}
		if !validHarnessIdentifier(root) {
			writeError(w, http.StatusBadRequest, "invalid or missing 'root' query parameter")
			return
		}

		snap, err := s.GetHarnessSnapshot(sess.teamID, project, root)
		if errors.Is(err, sql.ErrNoRows) {
			// Same response whether the row doesn't exist OR belongs to
			// another team. No enumeration via response-shape diffing.
			writeError(w, http.StatusNotFound, "harness snapshot not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeAudit(s, sess.email, "harness_get_project", project+":"+root, hashStr(sess.teamID), "")
		writeJSON(w, http.StatusOK, snap)
	}
}

// harnessListTransitions: GET /api/v1/harness/transitions?project=...&limit=...
//
// Team-scoped transition log. `project` is optional (empty → team-wide
// timeline). `limit` is clamped to [1, 1000] with a default of 100.
func harnessListTransitions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		project := r.URL.Query().Get("project")
		// project is optional; only validate when present.
		if project != "" && !validHarnessIdentifier(project) {
			writeError(w, http.StatusBadRequest, "invalid project filter")
			return
		}
		limit := parseHarnessLimit(r.URL.Query().Get("limit"))

		rows, err := s.ListHarnessTransitionsForTeam(sess.teamID, project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		auditTarget := sess.teamID
		if project != "" {
			auditTarget = project
		}
		writeAudit(s, sess.email, "harness_list_transitions", auditTarget, "", "")
		writeJSON(w, http.StatusOK, map[string]any{
			"transitions": rows,
			"count":       len(rows),
			"limit":       limit,
		})
	}
}

// parseHarnessLimit normalizes the ?limit= query param. Empty / invalid
// / non-positive → 100. > 1000 → 1000. The store also clamps but doing
// it here lets us echo the effective limit back to the client.
func parseHarnessLimit(raw string) int {
	const defaultLimit = 100
	const maxLimit = 1000
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}
