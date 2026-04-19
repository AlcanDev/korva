package db

import (
	"database/sql"
	"fmt"
)

// Migrate applies all schema migrations to the database.
// It is idempotent — safe to call on every startup.
func Migrate(db *sql.DB) error {
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
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
	// Scoped globally (admin-managed); name is the unique human identifier.
	// NEVER synced to Hive (same isolation policy as skills).
	`CREATE TABLE IF NOT EXISTS private_scrolls (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL UNIQUE,
		content    TEXT NOT NULL,
		created_by TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_private_scrolls_name ON private_scrolls(name)`,
}
