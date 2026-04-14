package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	cfg, err := Generate(keyPath, "test@alcandev", false)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(cfg.Key) != 64 {
		t.Errorf("Key should be 64 hex chars (32 bytes), got %d", len(cfg.Key))
	}
	if cfg.Owner != "test@alcandev" {
		t.Errorf("Owner = %q, want %q", cfg.Owner, "test@alcandev")
	}
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}

	// Verify file permissions are 0600
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Stat(admin.key) error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("File permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestGenerateErrKeyExists(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	if _, err := Generate(keyPath, "owner", false); err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}

	_, err := Generate(keyPath, "owner", false)
	if err != ErrKeyExists {
		t.Errorf("second Generate() error = %v, want ErrKeyExists", err)
	}
}

func TestGenerateForce(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	first, _ := Generate(keyPath, "owner", false)
	second, err := Generate(keyPath, "owner", true)
	if err != nil {
		t.Fatalf("Generate(force=true) error = %v", err)
	}

	if first.Key == second.Key {
		t.Error("force regeneration should produce a different key")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	original, _ := Generate(keyPath, "owner@test", false)

	loaded, err := Load(keyPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Key != original.Key {
		t.Error("Loaded key does not match generated key")
	}
}

func TestLoadNoFile(t *testing.T) {
	_, err := Load("/nonexistent/path/admin.key")
	if err != ErrNoAdminKey {
		t.Errorf("Load() error = %v, want ErrNoAdminKey", err)
	}
}

func TestRotate(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	original, _ := Generate(keyPath, "owner", false)

	rotated, err := Rotate(keyPath, original.Key)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}

	if rotated.Key == original.Key {
		t.Error("Rotated key should differ from original")
	}
	if rotated.Version != 2 {
		t.Errorf("Version after rotation = %d, want 2", rotated.Version)
	}
}

func TestRotateWrongKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	Generate(keyPath, "owner", false)

	_, err := Rotate(keyPath, "wrong-key")
	if err == nil {
		t.Error("Rotate() with wrong key should return error")
	}
}

func TestSecureEqual(t *testing.T) {
	if !secureEqual("abc", "abc") {
		t.Error("secureEqual('abc', 'abc') should be true")
	}
	if secureEqual("abc", "def") {
		t.Error("secureEqual('abc', 'def') should be false")
	}
	if secureEqual("", "abc") {
		t.Error("secureEqual('', 'abc') should be false")
	}
}
