package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInstallID_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.id")

	id, err := EnsureInstallID(path)
	if err != nil {
		t.Fatalf("EnsureInstallID: %v", err)
	}
	if len(id) != 32 {
		t.Errorf("install ID length = %d, want 32", len(id))
	}
	// File must exist and be readable
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != id+"\n" {
		t.Errorf("file content %q, want %q", string(data), id+"\n")
	}
}

func TestEnsureInstallID_IdempotentOnExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.id")

	id1, err := EnsureInstallID(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	id2, err := EnsureInstallID(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if id1 != id2 {
		t.Errorf("EnsureInstallID not idempotent: %q != %q", id1, id2)
	}
}

func TestEnsureHiveKey_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hive.key")

	key, err := EnsureHiveKey(path)
	if err != nil {
		t.Fatalf("EnsureHiveKey: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("hive key length = %d, want 64", len(key))
	}
}

func TestEnsureHiveKey_IdempotentOnExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hive.key")

	k1, _ := EnsureHiveKey(path)
	k2, err := EnsureHiveKey(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if k1 != k2 {
		t.Errorf("EnsureHiveKey not idempotent: %q != %q", k1, k2)
	}
}

func TestLoadInstallID_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.id")

	created, _ := EnsureInstallID(path)
	loaded, err := LoadInstallID(path)
	if err != nil {
		t.Fatalf("LoadInstallID: %v", err)
	}
	if loaded != created {
		t.Errorf("loaded %q, want %q", loaded, created)
	}
}

func TestLoadInstallID_MissingFile(t *testing.T) {
	_, err := LoadInstallID(filepath.Join(t.TempDir(), "no-such-file"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadHiveKey_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hive.key")
	created, _ := EnsureHiveKey(path)

	loaded, err := LoadHiveKey(path)
	if err != nil {
		t.Fatalf("LoadHiveKey: %v", err)
	}
	if loaded != created {
		t.Errorf("loaded %q, want %q", loaded, created)
	}
}

func TestLoadHiveKey_MissingFile(t *testing.T) {
	_, err := LoadHiveKey(filepath.Join(t.TempDir(), "no-such-file"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestRotateHiveKey_ChangesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hive.key")

	original, _ := EnsureHiveKey(path)
	rotated, err := RotateHiveKey(path)
	if err != nil {
		t.Fatalf("RotateHiveKey: %v", err)
	}
	if rotated == original {
		t.Error("RotateHiveKey returned same key — expected a new random key")
	}
	// Verify the file was actually updated
	loaded, _ := LoadHiveKey(path)
	if loaded != rotated {
		t.Errorf("file contains %q after rotate, want %q", loaded, rotated)
	}
}

func TestRotateHiveKey_CreatesIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hive.key")
	key, err := RotateHiveKey(path)
	if err != nil {
		t.Fatalf("RotateHiveKey on missing file: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64", len(key))
	}
}

func TestReadHexFile_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.id")
	if err := os.WriteFile(path, []byte("  abc123\n  "), 0600); err != nil {
		t.Fatal(err)
	}
	val, err := readHexFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if val != "abc123" {
		t.Errorf("got %q, want %q", val, "abc123")
	}
}

func TestFilePermissions_0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.id")
	_, err := EnsureInstallID(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}
