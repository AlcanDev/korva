// Package api implements the Vault HTTP REST API on port 7437.
package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/privacy/cloud"
	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/store"
)

// cleanupInterval is how often the rate-limiter sweeps stale per-IP entries.
const cleanupInterval = 5 * time.Minute

// maxBodyBytes is the maximum size we accept for any write request body.
// 1 MiB is generous for all legitimate vault payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// withBodyLimit wraps a handler with a 1 MiB body size limit.
// It prevents memory-bomb attacks on public write endpoints.
func withBodyLimit(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		h.ServeHTTP(w, r)
	}
}

// RouterConfig holds all dependencies for the Vault HTTP router.
// Using a config struct avoids positional argument churn as the router grows.
type RouterConfig struct {
	// AdminKeyPath is the filesystem path to the admin.key file.
	AdminKeyPath string
	// AdminKeyOverride, when non-empty, is used directly instead of reading
	// AdminKeyPath. Set from KORVA_ADMIN_KEY env var for container deployments.
	AdminKeyOverride string
	// License is the active license; nil means Community tier.
	License *license.License
	// LicensePath is the filesystem path to the JWS license file.
	LicensePath string
	// LicenseStatePath is the path to the persisted license heartbeat state.
	LicenseStatePath string
	// ActivationURL is the licensing server endpoint for key exchange.
	ActivationURL string
	// InstallID uniquely identifies this vault installation.
	InstallID string
	// Mailer sends transactional emails (e.g. invite notifications).
	// Use email.NewFromEnv() in production; a noopMailer is fine when unconfigured.
	Mailer email.Mailer
	// HiveClient enables hybrid cloud search when non-nil.
	// Callers pass ?cloud=1 to GET /api/v1/search to merge Hive results.
	HiveClient *hive.Client
	// HiveWorker exposes the worker's live sync status at GET /api/v1/hive/status.
	// Nil when Hive is disabled.
	HiveWorker *hive.Worker
	// HiveOutbox is the outbox queue; used for the dry-run admin endpoint.
	HiveOutbox *hive.Outbox
	// HiveFilter is the cloud privacy filter; used for the dry-run admin endpoint.
	HiveFilter *cloud.Filter
	// WebhookURL receives a POST for every saved observation (async, best-effort).
	// Set from VaultConfig.WebhookURL. Empty = disabled.
	WebhookURL string
	// VaultStartedAt is the wall-clock time the vault process started, used to
	// compute uptime in /admin/system-status. Zero value disables uptime.
	VaultStartedAt time.Time
	// VaultVersion is reported by /admin/system-status. Defaults to "" when unset.
	VaultVersion string
	// VaultPort is the listening port; reported by /admin/system-status.
	VaultPort int
	// ConfigPathLocal is the project-local korva.config.json path used by
	// /admin/system-status (and later by /admin/config). Empty = best-effort.
	ConfigPathLocal string
}

// EventBus is the package-level event bus used to fan out activity to the
// SSE endpoint. Initialized lazily by Router so tests can reuse it without
// spinning up the full router.
var eventBus = NewEventBus()

// PublishEvent is the package-public entry point for the SSE bus. Exported
// so other packages (store decorators, hive worker, …) can publish without
// importing the bus type. No-op when the bus is closed.
func PublishEvent(ev Event) { eventBus.Publish(ev) }

// Router builds the HTTP mux for the Vault API.
// ctx is used to stop the background rate-limiter cleanup goroutine when the
// server shuts down. Pass a zero-value RouterConfig for a minimal Community-tier
// server.
//
// All routes are wrapped with a per-IP rate limiter (120 req/min).
func Router(ctx context.Context, s *store.Store, cfg RouterConfig) http.Handler {
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

	// --- Hive-compatible ingest API (/v1/...) ---
	// These routes mirror the paths used by the Hive client so that any deployed
	// Korva vault can act as a Hive sync target without a separate backend.
	mux.HandleFunc("GET /v1/health", withCORS(http.HandlerFunc(hiveHealth)))
	mux.HandleFunc("POST /v1/observations/batch", withBodyLimit(withCORS(hiveBatchIngest(s))))
	mux.HandleFunc("GET /v1/observations", withCORS(listObservationsSince(s)))
	mux.HandleFunc("GET /v1/search", withCORS(searchObservations(s, cfg.HiveClient)))

	// Observations
	mux.HandleFunc("POST /api/v1/observations", withBodyLimit(withCORS(saveObservation(s, cfg.WebhookURL))))
	mux.HandleFunc("GET /api/v1/observations/{id}", withCORS(getObservation(s)))
	mux.HandleFunc("GET /api/v1/search", withCORS(searchObservations(s, cfg.HiveClient)))
	mux.HandleFunc("GET /api/v1/context/{project}", withCORS(contextObservations(s)))
	mux.HandleFunc("GET /api/v1/timeline/{project}", withCORS(timeline(s)))

	// Sessions
	mux.HandleFunc("GET /api/v1/sessions", withCORS(listSessions(s)))
	mux.HandleFunc("POST /api/v1/sessions", withBodyLimit(withCORS(startSession(s))))
	mux.HandleFunc("PUT /api/v1/sessions/{id}", withBodyLimit(withCORS(endSession(s))))

	// Summary and stats
	mux.HandleFunc("GET /api/v1/summary/{project}", withCORS(summary(s)))
	mux.HandleFunc("GET /api/v1/stats", withCORS(stats(s)))

	// OpenSpec — project conventions (GET public, no auth needed)
	mux.HandleFunc("GET /api/v1/openspec/{project}", withCORS(getOpenSpec(s)))
	// SDD phase — (GET public, no auth needed)
	mux.HandleFunc("GET /api/v1/sdd/{project}", withCORS(getSDDPhase(s)))

	// Hive sync status — unauthenticated, safe for dashboards
	mux.HandleFunc("GET /api/v1/hive/status", withCORS(hiveStatusHandler(cfg.HiveWorker)))

	// Lore export moved to authenticated team route — see sessMW block below

	// Prompts — write is unauthenticated so MCP can save without admin key
	mux.HandleFunc("POST /api/v1/prompts", withBodyLimit(withCORS(savePrompt(s))))

	// Observatory — interactions ingest (Observatory dashboard)
	// Public endpoint: any IDE wrapper can POST a prompt round-trip with token usage.
	// The global rate limiter (120 req/min) caps abuse; privacy filter is applied
	// inside the store layer.
	mux.HandleFunc("POST /api/v1/interactions", withBodyLimit(withCORS(ingestInteraction(s))))

	// Auth — public; member redeems invite → session token
	mux.HandleFunc("POST /auth/redeem", withCORS(authRedeem(s)))
	mux.HandleFunc("GET /auth/me", withCORS(authMe(s)))
	mux.HandleFunc("DELETE /auth/session", withCORS(authLogout(s)))

	// --- Admin-protected endpoints (X-Admin-Key required) ---

	adminMW := withAdminOrSessionAdmin(cfg.AdminKeyPath, cfg.AdminKeyOverride, s)
	const actor = "admin"

	// OpenSpec PUT + SDD PUT (write requires admin key)
	mux.Handle("PUT /api/v1/openspec/{project}", adminMW(withCORS(putOpenSpec(s))))
	mux.Handle("PUT /api/v1/sdd/{project}", adminMW(withCORS(putSDDPhase(s))))

	mux.Handle("GET /admin/hive/dry-run", adminMW(withCORS(hiveDryRunHandler(cfg.HiveOutbox, cfg.HiveFilter))))
	mux.Handle("GET /admin/hive/projects", adminMW(withCORS(listProjectSyncControls(cfg.HiveOutbox))))
	mux.Handle("POST /admin/hive/projects/{project}/pause", adminMW(withCORS(pauseProjectSync(cfg.HiveOutbox))))
	mux.Handle("POST /admin/hive/projects/{project}/resume", adminMW(withCORS(resumeProjectSync(cfg.HiveOutbox))))
	mux.Handle("POST /admin/purge", adminMW(withCORS(adminPurgeHandler(s, actor))))
	mux.Handle("GET /admin/export", adminMW(withCORS(adminExport(s, actor))))
	mux.Handle("DELETE /admin/observations/{id}", adminMW(withCORS(adminDeleteObservation(s))))
	mux.Handle("GET /admin/stats", adminMW(withCORS(stats(s))))
	mux.Handle("GET /admin/sessions", adminMW(withCORS(adminListSessions(s))))
	// Sessions — all — admin-only (was previously unauthenticated at /api/v1/sessions/all)
	mux.Handle("GET /admin/sessions/all", adminMW(withCORS(listAllSessions(s))))

	// Prompts — admin CRUD (read + delete)
	mux.Handle("GET /admin/prompts", adminMW(withCORS(adminListPrompts(s))))
	mux.Handle("GET /admin/prompts/{name}", adminMW(withCORS(adminGetPrompt(s))))
	mux.Handle("DELETE /admin/prompts/{name}", adminMW(withCORS(adminDeletePrompt(s))))

	// License — available to all authenticated admin callers
	mux.Handle("GET /admin/license/status", adminMW(withCORS(licenseStatusHandler(cfg.License, cfg.LicenseStatePath))))
	mux.Handle("POST /admin/license/activate", adminMW(withCORS(licenseActivateHandler(cfg.ActivationURL, cfg.InstallID, cfg.LicensePath, cfg.LicenseStatePath))))
	mux.Handle("POST /admin/license/deactivate", adminMW(withCORS(licenseDeactivateHandler(cfg.LicensePath, cfg.LicenseStatePath))))

	// --- Teams (feature-gated) ---

	teamsFeat := requireFeature(cfg.License, license.FeatureAdminSkills)

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
	auditFeat := requireFeature(cfg.License, license.FeatureAuditLog)
	mux.Handle("GET /admin/audit", adminMW(auditFeat(withCORS(adminListAudit(s)))))

	// Skills
	skillsFeat := requireFeature(cfg.License, license.FeatureAdminSkills)
	mux.Handle("GET /admin/code-health", adminMW(withCORS(adminCodeHealth(s))))
	mux.Handle("GET /admin/skills", adminMW(skillsFeat(withCORS(adminListSkills(s)))))
	mux.Handle("GET /admin/skills/sync-status", adminMW(skillsFeat(withCORS(adminSkillsSyncStatus(s)))))
	mux.Handle("GET /admin/skills/{id}", adminMW(skillsFeat(withCORS(adminGetSkill(s)))))
	mux.Handle("GET /admin/skills/{id}/history", adminMW(skillsFeat(withCORS(adminListSkillHistory(s)))))
	mux.Handle("POST /admin/skills", adminMW(skillsFeat(withCORS(adminSaveSkill(s, actor)))))
	mux.Handle("DELETE /admin/skills/{id}", adminMW(skillsFeat(withCORS(adminDeleteSkill(s, actor)))))

	// Private Scrolls
	scrollsFeat := requireFeature(cfg.License, license.FeaturePrivateScrolls)
	mux.Handle("GET /admin/scrolls/private", adminMW(scrollsFeat(withCORS(adminListPrivateScrolls(s)))))
	mux.Handle("POST /admin/scrolls/private", adminMW(scrollsFeat(withCORS(adminSavePrivateScroll(s, actor)))))
	mux.Handle("DELETE /admin/scrolls/private/{scroll_id}", adminMW(scrollsFeat(withCORS(adminDeletePrivateScroll(s, actor)))))

	// Admin lore export — unrestricted team_id, for backup/compliance
	mux.Handle("GET /admin/lore/export", adminMW(withCORS(adminLoreExportHandler(s))))

	// MCP call interactions log — query and aggregate tool usage
	mux.Handle("GET /admin/interactions", adminMW(withCORS(adminListInteractions(s))))
	mux.Handle("GET /admin/interactions/stats", adminMW(withCORS(adminInteractionStats(s))))

	// Observatory — prompt-level activity timeline + token analytics
	mux.Handle("GET /admin/activity", adminMW(withCORS(adminListActivity(s))))
	mux.Handle("GET /admin/activity/{id}", adminMW(withCORS(adminGetActivity(s))))
	mux.Handle("GET /admin/tokens/stats", adminMW(withCORS(adminTokenStats(s))))

	// Observatory — single-fetch system status (IDE, Vault, Hive, Sentinel, Lore, Skills, License, counts)
	mux.Handle("GET /admin/system-status", adminMW(withCORS(adminSystemStatus(systemStatusInputs{
		Store:           s,
		HiveWorker:      cfg.HiveWorker,
		License:         cfg.License,
		StartedAt:       cfg.VaultStartedAt,
		Version:         cfg.VaultVersion,
		Port:            cfg.VaultPort,
		ConfigPathLocal: cfg.ConfigPathLocal,
	}))))

	// Observatory — Configuration editor (read + write korva.config.json)
	configEP := newConfigEndpoint(s, cfg.ConfigPathLocal)
	mux.Handle("GET /admin/config", adminMW(withCORS(adminGetConfig(configEP))))
	mux.Handle("PUT /admin/config", adminMW(withBodyLimit(withCORS(adminPutConfig(configEP)))))
	mux.Handle("GET /admin/config/snapshots", adminMW(withCORS(adminListConfigSnapshots(s))))

	// Observatory — Vault restart (replaces the running process with a fresh copy)
	mux.Handle("POST /admin/vault/restart", adminMW(withCORS(adminRestartVault())))

	// Observatory — Sentinel rules editor (custom YAML rules + dry-run playground)
	mux.Handle("GET /admin/sentinel/rules", adminMW(withCORS(adminGetSentinelRules(cfg.ConfigPathLocal))))
	mux.Handle("PUT /admin/sentinel/rules", adminMW(withBodyLimit(withCORS(adminPutSentinelRules(cfg.ConfigPathLocal)))))
	mux.Handle("POST /admin/sentinel/test", adminMW(withBodyLimit(withCORS(adminTestSentinelRule()))))

	// Observatory — Integrity diagnostics + repair (doctor v2)
	mux.Handle("GET /admin/integrity", adminMW(withCORS(adminGetIntegrity(s))))
	mux.Handle("POST /admin/integrity/repair", adminMW(withBodyLimit(withCORS(adminRepairIntegrity(s)))))

	// Observatory — Conflict judgment workflow
	mux.Handle("GET /admin/conflicts", adminMW(withCORS(adminListConflicts(s))))
	mux.Handle("GET /admin/conflicts/{id}", adminMW(withCORS(adminGetConflict(s))))
	mux.Handle("POST /admin/conflicts/{id}/judge", adminMW(withBodyLimit(withCORS(adminJudgeConflict(s)))))
	mux.Handle("POST /admin/conflicts/{id}/ignore", adminMW(withBodyLimit(withCORS(adminIgnoreConflict(s)))))
	mux.Handle("POST /admin/conflicts/compare", adminMW(withBodyLimit(withCORS(adminCompareConflict(s)))))
	mux.Handle("POST /admin/observations/{id}/scan-conflicts", adminMW(withBodyLimit(withCORS(adminScanConflicts(s)))))

	// Observatory — Project hygiene (Phase 4)
	mux.Handle("GET /admin/projects", adminMW(withCORS(adminListProjects(s))))
	mux.Handle("GET /admin/projects/suggestions", adminMW(withCORS(adminSuggestConsolidations(s))))
	mux.Handle("POST /admin/projects/consolidate", adminMW(withBodyLimit(withCORS(adminConsolidateProjects(s)))))
	mux.Handle("POST /admin/projects/prune", adminMW(withBodyLimit(withCORS(adminPruneProjects(s)))))

	// Observatory — Export surface (Phase 5)
	mux.Handle("POST /admin/export/obsidian", adminMW(withBodyLimit(withCORS(adminExportObsidian(s)))))

	// Observatory — One-click command runner (Phase 7)
	mux.Handle("GET /admin/commands", adminMW(withCORS(adminListCommands())))
	mux.Handle("POST /admin/commands/run", adminMW(withBodyLimit(withCORS(adminRunCommand()))))

	// Observatory — Real-time event stream (Phase 8.5)
	mux.Handle("GET /admin/events", adminMW(withCORS(adminEventsSSE(eventBus))))

	// Observatory — Cost & ROI (Phase 8.6)
	mux.Handle("GET /admin/cost/summary", adminMW(withCORS(adminCostSummary(s))))

	// Observatory — Deferred-apply queue (cloud sync resilience)
	mux.Handle("GET /admin/cloud/deferred", adminMW(withCORS(adminListDeferred(s))))
	mux.Handle("POST /admin/cloud/deferred/{sync_id}/retry", adminMW(withBodyLimit(withCORS(adminRetryDeferred(s)))))
	mux.Handle("POST /admin/cloud/deferred/{sync_id}/applied", adminMW(withCORS(adminMarkDeferredApplied(s))))
	mux.Handle("DELETE /admin/cloud/deferred/{sync_id}", adminMW(withCORS(adminDeleteDeferred(s))))

	// --- Team member routes (X-Session-Token required) ---
	// A valid session token is sufficient proof of team membership — the team's
	// existence in the DB implies the vault admin created it with a Teams license.
	// No separate feature gate is needed here; the gate lives on the admin-side
	// invite/create routes that bootstrap the team.
	sessMW := withSession(s, lic)

	mux.Handle("GET /team/skills", sessMW(withCORS(teamListSkills(s))))
	mux.Handle("GET /team/skills/sync", sessMW(withCORS(teamSyncSkills(s))))
	mux.Handle("POST /team/skills/sync/report", sessMW(withCORS(teamReportSkillSync(s))))
	mux.Handle("POST /team/skills", sessMW(withCORS(teamSaveSkill(s))))
	mux.Handle("GET /team/skills/{id}/history", sessMW(withCORS(teamGetSkillHistory(s))))
	mux.Handle("DELETE /team/skills/{id}", sessMW(withCORS(teamDeleteSkill(s))))

	mux.Handle("GET /team/scrolls", sessMW(withCORS(teamListScrolls(s))))
	mux.Handle("POST /team/scrolls", sessMW(withCORS(teamSaveScroll(s))))
	mux.Handle("DELETE /team/scrolls/{id}", sessMW(withCORS(teamDeleteScroll(s))))

	// Lore export — scoped to the authenticated team (session required)
	mux.Handle("GET /team/lore/export", sessMW(withCORS(loreExportHandler(s))))

	// Wrap the entire mux with a per-IP fixed-window rate limiter.
	// 120 req/min is generous for AI editor usage; prevents runaway loops.
	limiter := NewRateLimiter(120, time.Minute)
	limiter.StartCleanup(ctx, cleanupInterval)
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
		if err := validateObservation(&obs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		id, err := s.Save(obs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		obs.ID = id
		notifyWebhook(webhookURL, obs)
		// Fan out to the SSE bus so live-dashboards update instantly.
		PublishEvent(Event{
			Kind:    EventObservationSaved,
			Project: obs.Project,
			Title:   obs.Title,
			Actor:   obs.Author,
			Meta:    map[string]any{"id": id, "type": string(obs.Type)},
		})
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
		if err := validateSession(body.Project, body.Team, body.Country, body.Agent, body.Goal); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
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
		// Body is optional — an empty or malformed body is treated as no summary.
		_ = json.NewDecoder(r.Body).Decode(&body)
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
		if err := validatePrompt(body.Name, body.Content); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
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

func adminListSessions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions, err := s.ListSessionsWithStats(100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "total": len(sessions)})
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		log.Printf("vault API error %d: %s", status, msg)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- prompts admin handlers ---

func adminListPrompts(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prompts, err := s.ListPrompts()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"prompts": prompts, "total": len(prompts)})
	}
}

func adminGetPrompt(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		p, err := s.GetPrompt(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if p == nil {
			writeError(w, http.StatusNotFound, "prompt not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func adminDeletePrompt(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		deleted, err := s.DeletePrompt(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !deleted {
			writeError(w, http.StatusNotFound, "prompt not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
	}
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

// ── Hive-compatible ingest endpoints (/v1/...) ───────────────────────────────
// These allow any Korva vault instance to act as a sync target for the Hive
// client without requiring a separate cloud backend.

// hiveHealth responds to GET /v1/health with a JSON ok so the Hive client's
// online probe passes.
func hiveHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// hiveBatchIngest accepts a gzip-encoded BatchRequest from the Hive client and
// saves each observation into the local vault. Duplicate content-hashes are
// silently skipped (the store dedup guard handles them).
func hiveBatchIngest(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		var err error

		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, e := newGzipReader(r.Body)
			if e != nil {
				writeError(w, http.StatusBadRequest, "invalid gzip body")
				return
			}
			defer gr.Close()
			body, err = readLimited(gr, maxBodyBytes)
		} else {
			body, err = readLimited(r.Body, maxBodyBytes)
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "could not read body")
			return
		}

		// BatchRequest: {client_id, batch_id, schema, observations:[]}
		// Each observation is a map — we extract only the fields the store needs.
		var batch struct {
			Observations []json.RawMessage `json:"observations"`
		}
		if err := json.Unmarshal(body, &batch); err != nil {
			writeError(w, http.StatusBadRequest, "invalid batch JSON")
			return
		}

		accepted := 0
		var skipped []string
		for _, raw := range batch.Observations {
			var obs store.Observation
			if err := json.Unmarshal(raw, &obs); err != nil {
				skipped = append(skipped, "parse_error")
				continue
			}
			if obs.Title == "" || obs.Content == "" {
				skipped = append(skipped, obs.ID)
				continue
			}
			if obs.Type == "" {
				obs.Type = store.TypeLearning
			}
			if _, err := s.Save(obs); err != nil {
				skipped = append(skipped, obs.ID)
				continue
			}
			accepted++
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"accepted": accepted,
			"skipped":  skipped,
		})
	}
}

func newGzipReader(r io.Reader) (*gzip.Reader, error) {
	return gzip.NewReader(r)
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
