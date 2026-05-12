package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migrate applies all schema migrations to the database.
// It is idempotent — safe to call on every startup.
//
// SQLite does not support ALTER TABLE ADD COLUMN IF NOT EXISTS, so we ignore
// "duplicate column name" errors to keep migrations append-only and idempotent.
func Migrate(db *sql.DB) error {
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// ALTER TABLE ADD COLUMN fails with "duplicate column name: <X>" when the
			// column already exists. Treat it as a no-op so the list stays idempotent.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("applying migration: %w", err)
		}
	}
	return nil
}

// migrations holds the ordered list of SQL statements to apply.
// New migrations are always appended — never modify existing ones.
var migrations = []string{
	// --- sessions ---
	`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		project    TEXT NOT NULL DEFAULT '',
		team       TEXT NOT NULL DEFAULT '',
		country    TEXT NOT NULL DEFAULT '',
		agent      TEXT NOT NULL DEFAULT '',
		goal       TEXT NOT NULL DEFAULT '',
		summary    TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL DEFAULT (datetime('now')),
		ended_at   TEXT
	)`,

	// --- observations ---
	`CREATE TABLE IF NOT EXISTS observations (
		id         TEXT PRIMARY KEY,
		session_id TEXT,
		project    TEXT NOT NULL DEFAULT '',
		team       TEXT NOT NULL DEFAULT '',
		country    TEXT NOT NULL DEFAULT '',
		type       TEXT NOT NULL,
		title      TEXT NOT NULL,
		content    TEXT NOT NULL,
		tags       TEXT NOT NULL DEFAULT '[]',
		author     TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	)`,

	// --- observations FTS5 virtual table ---
	`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
		title,
		content,
		tags,
		content='observations',
		content_rowid='rowid'
	)`,

	// --- FTS5 sync triggers ---
	`CREATE TRIGGER IF NOT EXISTS observations_ai
		AFTER INSERT ON observations BEGIN
			INSERT INTO observations_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END`,

	`CREATE TRIGGER IF NOT EXISTS observations_ad
		AFTER DELETE ON observations BEGIN
			INSERT INTO observations_fts(observations_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
		END`,

	`CREATE TRIGGER IF NOT EXISTS observations_au
		AFTER UPDATE ON observations BEGIN
			INSERT INTO observations_fts(observations_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
			INSERT INTO observations_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END`,

	// --- prompts ---
	`CREATE TABLE IF NOT EXISTS prompts (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL UNIQUE,
		content    TEXT NOT NULL,
		tags       TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// --- indexes ---
	`CREATE INDEX IF NOT EXISTS idx_observations_project   ON observations(project)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_team      ON observations(team)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_country   ON observations(country)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_type      ON observations(type)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_session   ON observations(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_created   ON observations(created_at)`,

	// --- cloud_outbox: queues observations for Korva Hive sync ---
	// status: pending | sent | rejected_privacy | failed
	`CREATE TABLE IF NOT EXISTS cloud_outbox (
		id              TEXT PRIMARY KEY,
		observation_id  TEXT NOT NULL,
		payload         BLOB NOT NULL,
		status          TEXT NOT NULL DEFAULT 'pending',
		attempts        INTEGER NOT NULL DEFAULT 0,
		last_error      TEXT NOT NULL DEFAULT '',
		next_attempt_at TEXT NOT NULL DEFAULT (datetime('now')),
		created_at      TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_outbox_status_next ON cloud_outbox(status, next_attempt_at)`,
	`CREATE INDEX IF NOT EXISTS idx_outbox_observation ON cloud_outbox(observation_id)`,

	// --- teams: Korva for Teams tenancy ---
	`CREATE TABLE IF NOT EXISTS teams (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		owner       TEXT NOT NULL DEFAULT '',
		license_id  TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// --- team_members ---
	`CREATE TABLE IF NOT EXISTS team_members (
		id         TEXT PRIMARY KEY,
		team_id    TEXT NOT NULL,
		email      TEXT NOT NULL,
		role       TEXT NOT NULL DEFAULT 'member',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members(team_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_team_members_email ON team_members(team_id, email)`,

	// --- audit_logs: every admin mutation is recorded ---
	`CREATE TABLE IF NOT EXISTS audit_logs (
		id          TEXT PRIMARY KEY,
		actor       TEXT NOT NULL,
		action      TEXT NOT NULL,
		target      TEXT NOT NULL,
		before_hash TEXT NOT NULL DEFAULT '',
		after_hash  TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_actor   ON audit_logs(actor)`,

	// --- skills: enterprise-only, NEVER synced to Hive ---
	`CREATE TABLE IF NOT EXISTS skills (
		id         TEXT PRIMARY KEY,
		team_id    TEXT NOT NULL DEFAULT '',
		name       TEXT NOT NULL,
		body       TEXT NOT NULL,
		tags       TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_skills_team ON skills(team_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_skills_team_name ON skills(team_id, name)`,

	// --- member_invites: one-time tokens admin sends to invite a member ---
	// token_hash = SHA256 of the plaintext token (never stored in clear)
	// used_at NULL = pending; non-null = already redeemed
	`CREATE TABLE IF NOT EXISTS member_invites (
		id          TEXT PRIMARY KEY,
		team_id     TEXT NOT NULL,
		email       TEXT NOT NULL,
		token_hash  TEXT NOT NULL UNIQUE,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		expires_at  TEXT NOT NULL,
		used_at     TEXT,
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_invites_team    ON member_invites(team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_invites_email   ON member_invites(email)`,
	`CREATE INDEX IF NOT EXISTS idx_invites_token   ON member_invites(token_hash)`,

	// --- member_sessions: active CLI sessions per member ---
	// Exactly one active session per member (email+team_id UNIQUE enforced in code).
	// token_hash = SHA256 of the plaintext session token.
	`CREATE TABLE IF NOT EXISTS member_sessions (
		id          TEXT PRIMARY KEY,
		team_id     TEXT NOT NULL,
		member_id   TEXT NOT NULL,
		email       TEXT NOT NULL,
		token_hash  TEXT NOT NULL UNIQUE,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		last_seen   TEXT NOT NULL DEFAULT (datetime('now')),
		expires_at  TEXT NOT NULL,
		FOREIGN KEY (team_id)   REFERENCES teams(id),
		FOREIGN KEY (member_id) REFERENCES team_members(id)
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_member ON member_sessions(email, team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_token   ON member_sessions(token_hash)`,

	// --- private_scrolls: team-managed knowledge documents served via Beacon ---
	// Scoped per-team via team_id ('' = global/admin-managed).
	// name is unique within a (team_id, name) pair.
	// NEVER synced to Hive (same isolation policy as skills).
	`CREATE TABLE IF NOT EXISTS private_scrolls (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		content    TEXT NOT NULL,
		team_id    TEXT NOT NULL DEFAULT '',
		created_by TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_private_scrolls_name ON private_scrolls(name)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_private_scrolls_team_name ON private_scrolls(team_id, name)`,

	// --- private_scrolls migration: add team_id for per-team scoping ---
	// For installs where the table was created before team_id existed.
	// Migrate() ignores "duplicate column name" so this is safe to apply repeatedly.
	`ALTER TABLE private_scrolls ADD COLUMN team_id TEXT NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_private_scrolls_team ON private_scrolls(team_id)`,

	// ── claude-mem integration: content-hash deduplication ───────────────────
	// Prevents storing the exact same observation twice (e.g. when the agent
	// re-processes the same tool output). hash = SHA256(title|content|project).
	`ALTER TABLE observations ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_observations_hash ON observations(content_hash)`,

	// ── SDD phase state — Spec-Driven Development workflow ──────────────────────
	// Tracks the current Spec-Driven Development phase per project.
	// Phases: explore → propose → spec → design → tasks → apply → verify → archive → onboard
	`CREATE TABLE IF NOT EXISTS sdd_state (
		project    TEXT PRIMARY KEY,
		phase      TEXT NOT NULL DEFAULT 'explore',
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// ── Project conventions table — per-project specification metadata ──────────
	// Stores per-project conventions (stack, architecture rules, testing standards)
	// injected automatically into every MCP session for that project.
	`CREATE TABLE IF NOT EXISTS openspec (
		project    TEXT PRIMARY KEY,
		content    TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// ── Quality gate checkpoints ───────────────────────────────────────────────
	// Records the result of a QA assessment at a specific SDD phase.
	// The AI performs the review; Korva stores the outcome for tracking and gating.
	//
	// status: pass | fail | partial | skip
	// score:  0-100 (AI-assigned quality score for that checkpoint)
	// findings: JSON array of {rule, status, notes} per quality criterion
	// gate_passed: 1 when this checkpoint satisfies the phase transition requirement
	`CREATE TABLE IF NOT EXISTS quality_checkpoints (
		id           TEXT PRIMARY KEY,
		project      TEXT NOT NULL,
		session_id   TEXT NOT NULL DEFAULT '',
		phase        TEXT NOT NULL,
		language     TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'partial',
		score        INTEGER NOT NULL DEFAULT 0,
		findings     TEXT NOT NULL DEFAULT '[]',
		notes        TEXT NOT NULL DEFAULT '',
		gate_passed  INTEGER NOT NULL DEFAULT 0,
		created_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_qc_project   ON quality_checkpoints(project)`,
	`CREATE INDEX IF NOT EXISTS idx_qc_phase     ON quality_checkpoints(project, phase)`,
	`CREATE INDEX IF NOT EXISTS idx_qc_created   ON quality_checkpoints(created_at)`,

	// ── Skill Hub: versioning + audit trail ──────────────────────────────────────
	// Tracks who changed each skill, at what version, and when.
	// scope: 'team' (default) | 'org' (visible to all org members).
	// These columns are always added via ALTER to keep the migration idempotent
	// for existing installs — Migrate() ignores "duplicate column name" errors.
	`ALTER TABLE skills ADD COLUMN version    INTEGER NOT NULL DEFAULT 1`,
	`ALTER TABLE skills ADD COLUMN updated_by TEXT    NOT NULL DEFAULT ''`,
	`ALTER TABLE skills ADD COLUMN scope      TEXT    NOT NULL DEFAULT 'team'`,

	// skill_history: immutable append-only record of every save.
	// Each row captures the exact body at that version so diffs are possible.
	// ON DELETE CASCADE keeps history consistent when a skill is deleted.
	`CREATE TABLE IF NOT EXISTS skill_history (
		id         TEXT PRIMARY KEY,
		skill_id   TEXT NOT NULL,
		version    INTEGER NOT NULL,
		body       TEXT NOT NULL,
		changed_by TEXT NOT NULL DEFAULT '',
		summary    TEXT NOT NULL DEFAULT '',
		changed_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_skill_history_skill   ON skill_history(skill_id)`,
	`CREATE INDEX IF NOT EXISTS idx_skill_history_changed ON skill_history(changed_at)`,

	// ── Skill Hub F5: sync telemetry ─────────────────────────────────────────────
	// Records every successful CLI sync so the admin can see who is up-to-date.
	// One row per sync event; the admin query groups by (user_email, target) to get
	// the latest sync per developer per machine.
	`CREATE TABLE IF NOT EXISTS skill_sync_log (
		id           TEXT PRIMARY KEY,
		team_id      TEXT NOT NULL,
		user_email   TEXT NOT NULL DEFAULT '',
		synced_at    TEXT NOT NULL DEFAULT (datetime('now')),
		skills_count INTEGER NOT NULL DEFAULT 0,
		target       TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sync_log_team  ON skill_sync_log(team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sync_log_email ON skill_sync_log(team_id, user_email)`,

	// ── Project sync controls ─────────────────────────────────────────────────────
	// Per-project pause/resume for Hive sync. Absent row = sync enabled (safe default).
	`CREATE TABLE IF NOT EXISTS project_sync_controls (
		project      TEXT PRIMARY KEY,
		sync_enabled INTEGER NOT NULL DEFAULT 1,
		paused_by    TEXT NOT NULL DEFAULT '',
		paused_at    TEXT,
		reason       TEXT NOT NULL DEFAULT '',
		updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// ── Smart Skill Auto-Loader: triggers + auto_load ─────────────────────────────
	// triggers — JSON object: {keywords, projects, file_patterns, priority}
	// auto_load — 1 = eligible for automatic injection in vault_context
	`ALTER TABLE skills ADD COLUMN triggers  TEXT    NOT NULL DEFAULT '{}'`,
	`ALTER TABLE skills ADD COLUMN auto_load INTEGER NOT NULL DEFAULT 0`,

	// ── Skill activation telemetry ────────────────────────────────────────────────
	// Records every auto-skill invocation for analytics and heuristic tuning.
	`CREATE TABLE IF NOT EXISTS skill_activations (
		id           TEXT PRIMARY KEY,
		skill_id     TEXT NOT NULL,
		team_id      TEXT NOT NULL DEFAULT '',
		project      TEXT NOT NULL DEFAULT '',
		prompt_hash  TEXT NOT NULL DEFAULT '',
		match_score  REAL NOT NULL DEFAULT 0,
		match_reason TEXT NOT NULL DEFAULT '',
		activated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_skill_activations_skill   ON skill_activations(skill_id)`,
	`CREATE INDEX IF NOT EXISTS idx_skill_activations_team    ON skill_activations(team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_skill_activations_project ON skill_activations(project)`,

	// ── Performance indexes ───────────────────────────────────────────────────────
	// Composite index for the most frequent query pattern: project + recency sort.
	// Speeds up Context(), Timeline(), and project-scoped search significantly.
	`CREATE INDEX IF NOT EXISTS idx_observations_project_created ON observations(project, created_at DESC)`,
	// Sessions ordered by start time — used by ListSessions, ListSessionsWithStats, Stats.
	`CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at DESC)`,

	// ── topic_key: stable upsert key for observations ─────────────────────────────
	// Enables vault_save to update an existing observation instead of creating a new
	// one when topic_key matches — ideal for evolving knowledge over multiple sessions.
	// NULL means no key (most observations). Non-null values are unique per project.
	`ALTER TABLE observations ADD COLUMN topic_key TEXT`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_observations_topic_key ON observations(project, topic_key) WHERE topic_key IS NOT NULL`,

	// ── working_dir: cwd where the observation was recorded ──────────────────────
	// Stored for project detection audit trail and future analytics.
	// Populated when vault_save receives a working_dir argument.
	`ALTER TABLE observations ADD COLUMN working_dir TEXT NOT NULL DEFAULT ''`,

	// ── observation_relations: semantic links between observations ─────────────────
	// Records directed relationships: source → target with a typed verdict.
	// relation values: supersedes | conflicts_with | related | compatible | scoped
	// status: pending | confirmed
	// Unique per (source_id, target_id) pair — only one relation per pair allowed.
	`CREATE TABLE IF NOT EXISTS observation_relations (
		id         TEXT PRIMARY KEY,
		source_id  TEXT NOT NULL,
		target_id  TEXT NOT NULL,
		relation   TEXT NOT NULL,
		status     TEXT NOT NULL DEFAULT 'confirmed',
		reason     TEXT NOT NULL DEFAULT '',
		author     TEXT NOT NULL DEFAULT '',
		project    TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (source_id) REFERENCES observations(id) ON DELETE CASCADE,
		FOREIGN KEY (target_id) REFERENCES observations(id) ON DELETE CASCADE
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_obs_relation ON observation_relations(source_id, target_id)`,
	`CREATE INDEX IF NOT EXISTS idx_obs_relation_source  ON observation_relations(source_id)`,
	`CREATE INDEX IF NOT EXISTS idx_obs_relation_target  ON observation_relations(target_id)`,
	`CREATE INDEX IF NOT EXISTS idx_obs_relation_project ON observation_relations(project)`,

	// ── mcp_calls: log every MCP tool invocation for analytics and audit ──────────
	// Captures tool name, author (session email or ''), project, outcome, and latency.
	// status: 'ok' | 'error'. Append-only — rows are never updated or deleted.
	`CREATE TABLE IF NOT EXISTS mcp_calls (
		id          TEXT PRIMARY KEY,
		tool        TEXT NOT NULL,
		project     TEXT NOT NULL DEFAULT '',
		author      TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'ok',
		latency_ms  INTEGER NOT NULL DEFAULT 0,
		error_msg   TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mcp_calls_tool    ON mcp_calls(tool)`,
	`CREATE INDEX IF NOT EXISTS idx_mcp_calls_created ON mcp_calls(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mcp_calls_project ON mcp_calls(project)`,

	// ── interactions: prompt-level activity log for Observatory dashboard ─────────
	// Distinct from mcp_calls (which is per-tool). One row per prompt round-trip.
	// Tokens fields are real when the client reports `usage` from the Anthropic SDK,
	// otherwise estimated server-side (estimated=1) from prompt+response length / 4.
	// prompt_excerpt and response_excerpt are privacy-filtered and capped at 8 KiB.
	`CREATE TABLE IF NOT EXISTS interactions (
		id                TEXT PRIMARY KEY,
		session_id        TEXT,
		project           TEXT NOT NULL DEFAULT '',
		team              TEXT NOT NULL DEFAULT '',
		agent             TEXT NOT NULL DEFAULT '',
		model             TEXT NOT NULL DEFAULT '',
		prompt_excerpt    TEXT NOT NULL DEFAULT '',
		response_excerpt  TEXT NOT NULL DEFAULT '',
		input_tokens      INTEGER NOT NULL DEFAULT 0,
		output_tokens     INTEGER NOT NULL DEFAULT 0,
		cache_read        INTEGER NOT NULL DEFAULT 0,
		cache_creation    INTEGER NOT NULL DEFAULT 0,
		duration_ms       INTEGER NOT NULL DEFAULT 0,
		tool_calls        TEXT NOT NULL DEFAULT '[]',
		status            TEXT NOT NULL DEFAULT 'ok',
		error_msg         TEXT NOT NULL DEFAULT '',
		estimated         INTEGER NOT NULL DEFAULT 0,
		created_at        TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_interactions_created_at ON interactions(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_interactions_project   ON interactions(project, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_interactions_model     ON interactions(model)`,
	`CREATE INDEX IF NOT EXISTS idx_interactions_agent     ON interactions(agent)`,
	`CREATE INDEX IF NOT EXISTS idx_interactions_status    ON interactions(status)`,

	// ── interactions FTS5 virtual table — full-text search on prompts/responses ──
	`CREATE VIRTUAL TABLE IF NOT EXISTS interactions_fts USING fts5(
		prompt_excerpt,
		response_excerpt,
		content='interactions',
		content_rowid='rowid',
		tokenize='porter unicode61'
	)`,

	// ── interactions FTS5 sync triggers ──────────────────────────────────────────
	`CREATE TRIGGER IF NOT EXISTS interactions_ai
		AFTER INSERT ON interactions BEGIN
			INSERT INTO interactions_fts(rowid, prompt_excerpt, response_excerpt)
			VALUES (new.rowid, new.prompt_excerpt, new.response_excerpt);
		END`,

	`CREATE TRIGGER IF NOT EXISTS interactions_ad
		AFTER DELETE ON interactions BEGIN
			INSERT INTO interactions_fts(interactions_fts, rowid, prompt_excerpt, response_excerpt)
			VALUES ('delete', old.rowid, old.prompt_excerpt, old.response_excerpt);
		END`,

	`CREATE TRIGGER IF NOT EXISTS interactions_au
		AFTER UPDATE ON interactions BEGIN
			INSERT INTO interactions_fts(interactions_fts, rowid, prompt_excerpt, response_excerpt)
			VALUES ('delete', old.rowid, old.prompt_excerpt, old.response_excerpt);
			INSERT INTO interactions_fts(rowid, prompt_excerpt, response_excerpt)
			VALUES (new.rowid, new.prompt_excerpt, new.response_excerpt);
		END`,

	// ── config_snapshots: full-content snapshot of korva.config.json before each PUT
	// Coexists with audit_logs: the audit row records the EVENT, the snapshot stores
	// the CONTENT for rollback. before_json is the file contents pre-mutation; after_json
	// is the post-mutation contents. Hashes are SHA-256 over the serialized JSON.
	`CREATE TABLE IF NOT EXISTS config_snapshots (
		id           TEXT PRIMARY KEY,
		actor        TEXT NOT NULL DEFAULT '',
		scope        TEXT NOT NULL,
		file_path    TEXT NOT NULL,
		before_hash  TEXT NOT NULL DEFAULT '',
		after_hash   TEXT NOT NULL DEFAULT '',
		before_json  TEXT NOT NULL DEFAULT '',
		after_json   TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_config_snapshots_created_at ON config_snapshots(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_config_snapshots_scope      ON config_snapshots(scope, created_at DESC)`,

	// ── normalized dedup columns on observations ────────────────────────────────
	// Complements content_hash (which is session-scoped exact match) with a
	// project-scoped, case+whitespace-normalized fingerprint so AI agents that
	// re-emit a known fact with slight formatting differences upsert into the
	// original row instead of cluttering the timeline.
	//
	// normalized_hash: SHA-256 of (lowercased, whitespace-collapsed)
	//                  title+'|'+content+'|'+project, first 32 hex chars.
	// duplicate_count: incremented every time the same normalized_hash arrives
	//                  within the dedup window (default 15 minutes).
	// last_seen_at:    refreshed on every duplicate hit so callers can tell
	//                  "we saw this knowledge again 2 hours ago" even when the
	//                  original row is days old.
	`ALTER TABLE observations ADD COLUMN normalized_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE observations ADD COLUMN duplicate_count INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE observations ADD COLUMN last_seen_at    TEXT`,
	// Index optimizes the project-scoped, recency-aware lookup performed by Save().
	`CREATE INDEX IF NOT EXISTS idx_observations_normhash ON observations(project, normalized_hash, created_at DESC)`,
}
