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
}
