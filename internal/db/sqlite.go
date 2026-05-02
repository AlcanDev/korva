package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at dbPath and applies migrations.
func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configuring sqlite: %w", err)
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// OpenMemory opens an in-memory SQLite database.
// Intended for tests — data is not persisted.
func OpenMemory() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory sqlite: %w", err)
	}

	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configuring in-memory sqlite: %w", err)
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating in-memory sqlite: %w", err)
	}

	return db, nil
}

// configure sets SQLite pragmas for best performance and safety.
func configure(db *sql.DB) error {
	// One writer at a time — SQLite does not benefit from multiple open connections
	// because writes serialize at the OS level regardless. Keeping a single idle
	// connection also prevents the overhead of opening/closing on every request.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // never expire; keep the connection warm

	pragmas := []string{
		"PRAGMA journal_mode=WAL",      // write-ahead log: readers never block writers
		"PRAGMA foreign_keys=ON",       // enforce referential integrity
		"PRAGMA busy_timeout=5000",     // wait up to 5 s before returning SQLITE_BUSY
		"PRAGMA synchronous=NORMAL",    // safe with WAL; fsync only at checkpoints
		"PRAGMA cache_size=-64000",     // 64 MiB page cache
		"PRAGMA temp_store=MEMORY",     // temp tables and indexes in RAM
		"PRAGMA mmap_size=268435456",   // 256 MiB memory-mapped I/O (read path)
		"PRAGMA wal_autocheckpoint=1000", // checkpoint every 1 000 pages (default, explicit)
		"PRAGMA optimize",              // update query-planner statistics at open time
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("setting pragma %q: %w", pragma, err)
		}
	}

	return nil
}
