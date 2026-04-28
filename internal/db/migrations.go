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

	// ── gentle-ai integration: SDD phase state ───────────────────────────────
	// Tracks the current Spec-Driven Development phase per project.
	// Phases: explore → propose → spec → design → tasks → apply → verify → archive → onboard
	`CREATE TABLE IF NOT EXISTS sdd_state (
		project    TEXT PRIMARY KEY,
		phase      TEXT NOT NULL DEFAULT 'explore',
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// ── gentle-ai integration: OpenSpec project conventions ──────────────────
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
}
