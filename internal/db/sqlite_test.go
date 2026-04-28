package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenMemory(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("OpenMemory() returned nil db")
	}

	// Verify it's actually usable
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping() after OpenMemory() error = %v", err)
	}
}

func TestOpenMemory_TablesCreated(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer db.Close()

	tables := []string{"observations", "sessions", "prompts"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Open() did not create the database file")
	}
}

func TestOpen_IdempotentMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First open
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	db1.Close()

	// Second open on same file — migrations should be idempotent
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db2.Close()

	if err := db2.Ping(); err != nil {
		t.Fatalf("Ping() on reopened db error = %v", err)
	}
}

func TestMigrate_FTS5Available(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer db.Close()

	// FTS5 virtual table should exist
	var name string
	err = db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='observations_fts'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("observations_fts virtual table not found: %v", err)
	}
}

func TestMigrate_IndexesCreated(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer db.Close()

	indexes := []string{
		"idx_observations_project",
		"idx_observations_type",
		"idx_observations_created",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// Pick a path that's guaranteed to be uncreatable on each OS:
	//   - Unix: /dev/null/<x> — /dev/null is a char device, can't be a parent dir.
	//   - Windows: a path under NUL\ — NUL is a reserved device name.
	// Both produce a "not a directory" / "invalid path" error from the OS.
	var invalidPath string
	if runtime.GOOS == "windows" {
		invalidPath = `NUL\korva\test.db`
	} else {
		invalidPath = "/dev/null/korva/test.db"
	}
	_, err := Open(invalidPath)
	if err == nil {
		t.Errorf("Open(%q) on invalid path should return an error", invalidPath)
	}
}
