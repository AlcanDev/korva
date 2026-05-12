package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_HappyPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Vault.Port = 7437
	cfg.Country = "CL"
	cfg.Agent = "claude"
	cfg.Lore.ScrollPriority = "private_first"
	cfg.Sentinel.Hooks = []string{"pre-commit"}

	if err := Validate(cfg); err != nil {
		t.Errorf("Validate(default) error = %v, want nil", err)
	}
}

func TestValidate_PortRange(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"too low", 80, true},
		{"too high", 70000, true},
		{"valid", 8080, false},
		{"zero (unset)", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Vault.Port = tc.port
			err := Validate(cfg)
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_AgentEnum(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent = "magic"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *ValidationError, got %T", err)
	}
	if ve.Field != "agent" {
		t.Errorf("Field = %q, want %q", ve.Field, "agent")
	}
}

func TestValidate_BlankPattern(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Vault.PrivatePatterns = []string{"password", "  ", "token"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	var ve *ValidationError
	errors.As(err, &ve)
	if !strings.HasPrefix(ve.Field, "vault.private_patterns[") {
		t.Errorf("expected vault.private_patterns[i], got %q", ve.Field)
	}
}

func TestValidate_UnknownHook(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Sentinel.Hooks = []string{"pre-commit", "yolo"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteAtomic_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	cfg := DefaultConfig()
	cfg.Project = "korva"
	cfg.Country = "CL"
	cfg.Agent = "claude"
	cfg.Vault.Port = 7437

	res, err := WriteAtomic(path, cfg, WriteOptions{})
	if err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	if res == nil {
		t.Fatal("expected result, got nil")
	}
	if res.AfterHash == "" || res.AfterHash == res.BeforeHash {
		t.Errorf("expected non-empty AfterHash distinct from BeforeHash")
	}
	if res.BeforeJSON != "" {
		t.Errorf("BeforeJSON should be empty on first write")
	}
	if !strings.Contains(res.AfterJSON, `"project": "korva"`) {
		t.Errorf("AfterJSON missing project: %s", res.AfterJSON)
	}

	// Verify on-disk content.
	got, _ := Load(path)
	if got.Project != "korva" || got.Vault.Port != 7437 {
		t.Errorf("on-disk config mismatch: %+v", got)
	}
}

func TestWriteAtomic_NoTmpFileLeftBehind(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	cfg := DefaultConfig()
	cfg.Country = "CL"
	cfg.Agent = "claude"

	if _, err := WriteAtomic(path, cfg, WriteOptions{}); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}

func TestWriteAtomic_HashConflict(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	// Initial write.
	cfg := DefaultConfig()
	cfg.Country = "CL"
	cfg.Agent = "claude"
	res, err := WriteAtomic(path, cfg, WriteOptions{})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	expectedHash := res.AfterHash

	// Someone else changes the file out-of-band.
	if err := os.WriteFile(path, []byte(`{"version":"99","project":"foo"}`), 0o644); err != nil {
		t.Fatalf("out-of-band write: %v", err)
	}

	// Second WriteAtomic with the previously-known hash should fail.
	cfg.Project = "korva"
	_, err = WriteAtomic(path, cfg, WriteOptions{ExpectedHash: expectedHash})
	if err == nil {
		t.Fatal("expected ConflictError")
	}
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Errorf("expected *ConflictError, got %T (%v)", err, err)
	}
}

func TestWriteAtomic_RestartRequiredDetection(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	cfg := DefaultConfig()
	cfg.Country = "CL"
	cfg.Agent = "claude"
	cfg.Vault.Port = 7437

	if _, err := WriteAtomic(path, cfg, WriteOptions{}); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Change vault.port — should trigger restart_required.
	cfg.Vault.Port = 8080
	res, err := WriteAtomic(path, cfg, WriteOptions{})
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !sliceContains(res.RestartRequired, "vault.port") {
		t.Errorf("RestartRequired = %v, want contains vault.port", res.RestartRequired)
	}
}

func TestWriteAtomic_RejectsInvalidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	cfg := DefaultConfig()
	cfg.Vault.Port = 80 // out of range
	cfg.Country = "CL"

	_, err := WriteAtomic(path, cfg, WriteOptions{})
	if err == nil {
		t.Fatal("expected ValidationError")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *ValidationError, got %T", err)
	}

	// File should NOT be written.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not exist after validation failure, stat err = %v", err)
	}
}

func TestHashFile_MissingReturnsEmpty(t *testing.T) {
	if got := HashFile("/path/that/does/not/exist"); got != "" {
		t.Errorf("HashFile(missing) = %q, want empty", got)
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
