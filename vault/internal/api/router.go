// Package api implements the Vault HTTP REST API on port 7437.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/store"
)

// RouterConfig holds all dependencies for the Vault HTTP router.
// Using a config struct avoids positional argument churn as the router grows.
type RouterConfig struct {
	// AdminKeyPath is the filesystem path to the admin.key file.
	AdminKeyPath string
	// License is the active license; nil means Community tier.
	License *license.License
	// LicenseStatePath is the path to the persisted license heartbeat state.
	LicenseStatePath string
	// Mailer sends transactional emails (e.g. invite notifications).
	// Use email.NewFromEnv() in production; a noopMailer is fine when unconfigured.
	Mailer email.Mailer
	// HiveClient enables hybrid cloud search when non-nil.
	// Callers pass ?cloud=1 to GET /api/v1/search to merge Hive results.
	HiveClient *hive.Client
	// WebhookURL receives a POST for every saved observation (async, best-effort).
	// Set from VaultConfig.WebhookURL. Empty = disabled.
	WebhookURL string
}

// Router builds the HTTP mux for the Vault API.
// Pass a zero-value RouterConfig for a minimal Community-tier server.
//
// All routes are wrapped with a per-IP rate limiter (120 req/min).
func Router(s *store.Store, cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Resolve a nil mailer to the noop implementation so handlers never nil-check.
	mailer := cfg.Mailer
	if mailer == nil {
		mailer = email.NewFromEnv()
	}

	lic := cfg.License

	// --- Public endpoints ---

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /api/v1/status", withCORS(statusHandler(s, lic)))
	mux.HandleFunc("GET /api/v1/metrics", withCORS(metricsHandler(s)))

	// Observations
	mux.HandleFunc("POST /api/v1/observations", withCORS(saveObservation(s, cfg.WebhookURL)))
	mux.HandleFunc("GET /api/v1/observations/{id}", withCORS(getObservation(s)))
	mux.HandleFunc("GET /api/v1/search", withCORS(searchObservations(s, cfg.HiveClient)))
	mux.HandleFunc("GET /api/v1/context/{project}", withCORS(contextObservations(s)))
	mux.HandleFunc("GET /api/v1/timeline/{project}", withCORS(timeline(s)))

	// Sessions
	mux.HandleFunc("POST /api/v1/sessions", withCORS(startSession(s)))
	mux.HandleFunc("PUT /api/v1/sessions/{id}", withCORS(endSession(s)))

	// Summary and stats
	mux.HandleFunc("GET /api/v1/summary/{project}", withCORS(summary(s)))
	mux.HandleFunc("GET /api/v1/stats", withCORS(stats(s)))

	// OpenSpec — project conventions (GET public, no auth needed)
	mux.HandleFunc("GET /api/v1/openspec/{project}", withCORS(getOpenSpec(s)))
	// SDD phase — (GET public, no auth needed)
	mux.HandleFunc("GET /api/v1/sdd/{project}", withCORS(getSDDPhase(s)))

	// Prompts
	mux.HandleFunc("POST /api/v1/prompts", withCORS(savePrompt(s)))

	// Sessions — all (admin-level listing)
	mux.HandleFunc("GET /api/v1/sessions/all", withCORS(listAllSessions(s)))

	// Auth — public; member redeems invite → session token
	mux.HandleFunc("POST /auth/redeem", withCORS(authRedeem(s)))
	mux.HandleFunc("GET /auth/me", withCORS(authMe(s)))
	mux.HandleFunc("DELETE /auth/session", withCORS(authLogout(s)))

	// --- Admin-protected endpoints (X-Admin-Key required) ---

	adminMW := admin.Middleware(cfg.AdminKeyPath)
	const actor = "admin"

	// OpenSpec PUT + SDD PUT (write requires admin key)
	mux.Handle("PUT /api/v1/openspec/{project}", adminMW(withCORS(putOpenSpec(s))))
	mux.Handle("PUT /api/v1/sdd/{project}", adminMW(withCORS(putSDDPhase(s))))

	mux.Handle("POST /admin/purge", adminMW(withCORS(adminPurgeHandler(s, actor))))
	mux.Handle("GET /admin/export", adminMW(withCORS(adminExport(s, actor))))
	mux.Handle("DELETE /admin/observations/{id}", adminMW(withCORS(adminDeleteObservation(s))))
	mux.Handle("GET /admin/stats", adminMW(withCORS(adminFullStats(s))))

	// License — available to all authenticated admin callers
	mux.Handle("GET /admin/license/status", adminMW(withCORS(licenseStatusHandler(lic, cfg.LicenseStatePath))))

	// --- Teams (feature-gated) ---

	teamsFeat := requireFeature(lic, license.FeatureAdminSkills)

	mux.Handle("GET /admin/teams/profile/active", adminMW(withCORS(adminActiveProfile(s))))
	mux.Handle("GET /admin/teams", adminMW(withCORS(adminListTeams(s))))
	mux.Handle("POST /admin/teams", adminMW(teamsFeat(withCORS(adminCreateTeam(s, actor)))))
	mux.Handle("GET /admin/teams/{team_id}/members", adminMW(withCORS(adminListMembers(s))))
	mux.Handle("POST /admin/teams/{team_id}/members", adminMW(teamsFeat(withCORS(adminAddMember(s, actor, lic)))))
	mux.Handle("DELETE /admin/teams/{team_id}/members/{member_id}", adminMW(teamsFeat(withCORS(adminRemoveMember(s, actor)))))

	// Member invites — email is sent when the mailer is configured
	mux.Handle("GET /admin/teams/{team_id}/invites", adminMW(withCORS(adminListInvites(s))))
	mux.Handle("POST /admin/teams/{team_id}/invites", adminMW(teamsFeat(withCORS(adminCreateInvite(s, actor, mailer)))))
	mux.Handle("DELETE /admin/teams/{team_id}/invites/{invite_id}", adminMW(teamsFeat(withCORS(adminRevokeInvite(s, actor)))))

	// Member sessions — list + force-revoke
	mux.Handle("GET /admin/teams/{team_id}/sessions", adminMW(withCORS(adminListTeamSessions(s))))
	mux.Handle("DELETE /admin/teams/{team_id}/sessions/{session_id}", adminMW(teamsFeat(withCORS(adminRevokeSession(s, actor)))))

	// Audit log
	auditFeat := requireFeature(lic, license.FeatureAuditLog)
	mux.Handle("GET /admin/audit", adminMW(auditFeat(withCORS(adminListAudit(s)))))

	// Skills
	skillsFeat := requireFeature(lic, license.FeatureAdminSkills)
	mux.Handle("GET /admin/skills", adminMW(skillsFeat(withCORS(adminListSkills(s)))))
	mux.Handle("GET /admin/skills/{id}", adminMW(skillsFeat(withCORS(adminGetSkill(s)))))
	mux.Handle("POST /admin/skills", adminMW(skillsFeat(withCORS(adminSaveSkill(s, actor)))))
	mux.Handle("DELETE /admin/skills/{id}", adminMW(skillsFeat(withCORS(adminDeleteSkill(s, actor)))))

	// Private Scrolls
	scrollsFeat := requireFeature(lic, license.FeaturePrivateScrolls)
	mux.Handle("GET /admin/scrolls/private", adminMW(scrollsFeat(withCORS(adminListPrivateScrolls(s)))))
	mux.Handle("POST /admin/scrolls/private", adminMW(scrollsFeat(withCORS(adminSavePrivateScroll(s, actor)))))
	mux.Handle("DELETE /admin/scrolls/private/{scroll_id}", adminMW(scrollsFeat(withCORS(adminDeletePrivateScroll(s, actor)))))

	// --- Team member routes (X-Session-Token required) ---
	// A valid session token is sufficient proof of team membership — the team's
	// existence in the DB implies the vault admin created it with a Teams license.
	// No separate feature gate is needed here; the gate lives on the admin-side
	// invite/create routes that bootstrap the team.
	sessMW := withSession(s, lic)

	mux.Handle("GET /team/skills", sessMW(withCORS(teamListSkills(s))))
	mux.Handle("POST /team/skills", sessMW(withCORS(teamSaveSkill(s))))
	mux.Handle("DELETE /team/skills/{id}", sessMW(withCORS(teamDeleteSkill(s))))

	mux.Handle("GET /team/scrolls", sessMW(withCORS(teamListScrolls(s))))
	mux.Handle("POST /team/scrolls", sessMW(withCORS(teamSaveScroll(s))))
	mux.Handle("DELETE /team/scrolls/{id}", sessMW(withCORS(teamDeleteScroll(s))))

	// Wrap the entire mux with a per-IP fixed-window rate limiter.
	// 120 req/min is generous for AI editor usage; prevents runaway loops.
	limiter := NewRateLimiter(120, time.Minute)
	return limiter.Middleware(mux)
}

// --- handlers ---

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "korva-vault"})
}

func saveObservation(s *store.Store, webhookURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var obs store.Observation
		if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := s.Save(obs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		obs.ID = id
		notifyWebhook(webhookURL, obs)
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func getObservation(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		obs, err := s.Get(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if obs == nil {
			writeError(w, http.StatusNotFound, "observation not found")
			return
		}
		writeJSON(w, http.StatusOK, obs)
	}
}

// searchHit wraps a local Observation with an explicit source tag so callers
// can distinguish local results from cloud results in a single response.
type searchHit struct {
	store.Observation
	Source string `json:"source"`
}

func searchObservations(s *store.Store, hiveClient *hive.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		limit := 20
		if lStr := q.Get("limit"); lStr != "" {
			if n, err := strconv.Atoi(lStr); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		offset := 0
		if oStr := q.Get("offset"); oStr != "" {
			if n, err := strconv.Atoi(oStr); err == nil && n >= 0 {
				offset = n
			}
		}

		filters := store.SearchFilters{
			Project: q.Get("project"),
			Team:    q.Get("team"),
			Country: q.Get("country"),
			Type:    store.ObservationType(q.Get("type")),
			Limit:   limit,
			Offset:  offset,
		}

		localObs, err := s.Search(q.Get("q"), filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Total count for pagination — use filter without Limit/Offset.
		// Skip the expensive COUNT for FTS queries (non-empty q).
		var total int
		if q.Get("q") == "" {
			total, _ = s.CountObservations(filters)
		} else {
			total = -1 // FTS: total unavailable
		}

		hits := make([]searchHit, 0, len(localObs))
		for _, obs := range localObs {
			hits = append(hits, searchHit{obs, "local"})
		}

		// Optional Hive cloud results — only when ?cloud=1 and the client is wired.
		if q.Get("cloud") == "1" && hiveClient != nil {
			cloudCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			cloudResults, cloudErr := hiveClient.Search(cloudCtx, q.Get("q"), 20)
			if cloudErr == nil && len(cloudResults) > 0 {
				// Deduplicate: skip Hive entries whose ID already appears locally.
				localIDs := make(map[string]bool, len(localObs))
				for _, obs := range localObs {
					localIDs[obs.ID] = true
				}
				for _, cr := range cloudResults {
					if localIDs[cr.ID] {
						continue
					}
					hits = append(hits, searchHit{
						Observation: store.Observation{
							ID:      cr.ID,
							Type:    store.ObservationType(cr.Type),
							Title:   cr.Title,
							Content: cr.Content,
						},
						Source: "hive",
					})
				}
			}
		}

		resp := map[string]any{
			"results": hits,
			"count":   len(hits),
			"limit":   limit,
			"offset":  offset,
		}
		if total >= 0 {
			resp["total"] = total
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func contextObservations(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		results, err := s.Context(project, nil, 10)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"context": results, "project": project})
	}
}

func timeline(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		q := r.URL.Query()

		from := time.Now().Add(-7 * 24 * time.Hour)
		to := time.Now()

		if fromStr := q.Get("from"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				from = t
			}
		}
		if toStr := q.Get("to"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				to = t
			}
		}

		results, err := s.Timeline(project, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"timeline": results, "project": project})
	}
}

func startSession(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Project string `json:"project"`
			Team    string `json:"team"`
			Country string `json:"country"`
			Agent   string `json:"agent"`
			Goal    string `json:"goal"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := s.SessionStart(body.Project, body.Team, body.Country, body.Agent, body.Goal)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"session_id": id})
	}
}

func endSession(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Summary string `json:"summary"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		if err := s.SessionEnd(id, body.Summary); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
	}
}

func summary(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		result, err := s.Summary(project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func stats(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.Stats()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func savePrompt(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name    string   `json:"name"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := s.SavePrompt(body.Name, body.Content, body.Tags); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "saved"})
	}
}

func listAllSessions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions, err := s.ListSessions(100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

// --- admin handlers ---

func adminDeleteObservation(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing observation id")
			return
		}
		deleted, err := s.Delete(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		if !deleted {
			writeError(w, http.StatusNotFound, "observation not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
	}
}

func adminFullStats(s *store.Store) http.HandlerFunc {
	return stats(s)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func corsOrigin() string {
	if v := os.Getenv("KORVA_CORS_ORIGIN"); v != "" {
		return v
	}
	return "http://localhost:5173"
}

func withCORS(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", corsOrigin())
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Key, X-Session-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	}
}
