package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformPaths(t *testing.T) {
	paths, err := PlatformPaths()
	if err != nil {
		t.Fatalf("PlatformPaths() error = %v", err)
	}

	if paths.HomeDir == "" {
		t.Error("HomeDir should not be empty")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(paths.HomeDir, "korva") {
			t.Errorf("Windows HomeDir should contain 'korva', got: %s", paths.HomeDir)
		}
	} else {
		if !strings.Contains(paths.HomeDir, ".korva") {
			t.Errorf("Unix HomeDir should contain '.korva', got: %s", paths.HomeDir)
		}
	}

	// All derived paths should be under HomeDir
	derived := []struct {
		name string
		val  string
	}{
		{"ConfigFile", paths.ConfigFile},
		{"AdminKey", paths.AdminKey},
		{"ProfilesDir", paths.ProfilesDir},
		{"LoreDir", paths.LoreDir},
		{"VaultDir", paths.VaultDir},
		{"LogsDir", paths.LogsDir},
	}

	for _, d := range derived {
		if !strings.HasPrefix(d.val, paths.HomeDir) {
			t.Errorf("%s (%s) should be under HomeDir (%s)", d.name, d.val, paths.HomeDir)
		}
	}
}

func TestProfileDirSanitization(t *testing.T) {
	paths := &Paths{ProfilesDir: "/base/profiles"}

	tests := []struct {
		id   string
		safe bool
	}{
		{"acme-corp", true},
		{"../../../etc", false},
		{"my_team", true},
		{"team/with/slash", false},
	}

	for _, tt := range tests {
		result := paths.ProfileDir(tt.id)
		if strings.Contains(result, "..") {
			t.Errorf("ProfileDir(%q) contains path traversal: %s", tt.id, result)
		}
		if strings.Contains(result, "/etc") {
			t.Errorf("ProfileDir(%q) escapes base dir: %s", tt.id, result)
		}
	}
}

func TestVaultDB(t *testing.T) {
	paths := &Paths{VaultDir: "/home/user/.korva/vault"}
	db := paths.VaultDB()
	if !strings.HasSuffix(db, "observations.db") {
		t.Errorf("VaultDB() = %s, want suffix 'observations.db'", db)
	}
}

// loreDirBase returns a platform-appropriate base path for testing PrivateLoreDir
// and PublicLoreDir. Hard-coded /home/user paths break on Windows because
// filepath.Join normalises separators (\ vs /).
func loreDirBase() string {
	if runtime.GOOS == "windows" {
		return `C:\Users\test\.korva\lore`
	}
	return "/home/user/.korva/lore"
}

func TestPrivateLoreDir(t *testing.T) {
	paths := &Paths{LoreDir: loreDirBase()}
	dir := paths.PrivateLoreDir()
	if filepath.Base(dir) != "private" {
		t.Errorf("PrivateLoreDir() = %s, want last component 'private'", dir)
	}
	if !strings.HasPrefix(dir, paths.LoreDir) {
		t.Errorf("PrivateLoreDir() = %s, should be under LoreDir %s", dir, paths.LoreDir)
	}
}

func TestPublicLoreDir(t *testing.T) {
	paths := &Paths{LoreDir: loreDirBase()}
	dir := paths.PublicLoreDir()
	if filepath.Base(dir) != "public" {
		t.Errorf("PublicLoreDir() = %s, want last component 'public'", dir)
	}
	if !strings.HasPrefix(dir, paths.LoreDir) {
		t.Errorf("PublicLoreDir() = %s, should be under LoreDir %s", dir, paths.LoreDir)
	}
}

func TestEnsureAll_CreatesDirectories(t *testing.T) {
	base := t.TempDir()
	paths := &Paths{
		HomeDir:     base,
		ConfigFile:  base + "/config.json",
		AdminKey:    base + "/admin.key",
		ProfilesDir: base + "/profiles",
		LoreDir:     base + "/lore",
		VaultDir:    base + "/vault",
		LogsDir:     base + "/logs",
	}

	if err := paths.EnsureAll(); err != nil {
		t.Fatalf("EnsureAll() error = %v", err)
	}

	dirs := []string{
		paths.ProfilesDir,
		paths.LoreDir,
		paths.PrivateLoreDir(),
		paths.PublicLoreDir(),
		paths.VaultDir,
		paths.LogsDir,
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("EnsureAll() did not create directory %s", dir)
		}
	}
}

func TestEnsureAll_Idempotent(t *testing.T) {
	base := t.TempDir()
	paths := &Paths{
		HomeDir:     base,
		ConfigFile:  base + "/config.json",
		AdminKey:    base + "/admin.key",
		ProfilesDir: base + "/profiles",
		LoreDir:     base + "/lore",
		VaultDir:    base + "/vault",
		LogsDir:     base + "/logs",
	}

	// Call twice — should not error
	if err := paths.EnsureAll(); err != nil {
		t.Fatalf("first EnsureAll() error = %v", err)
	}
	if err := paths.EnsureAll(); err != nil {
		t.Fatalf("second EnsureAll() error = %v", err)
	}
}

func TestSanitizeProfileID(t *testing.T) {
	tests := []struct {
		input    string
		wantSafe bool
	}{
		{"acme-corp", true},
		{"my_team_v2", true},
		{"../../../etc/passwd", false}, // must not contain ..
		{"team/slash", false},          // must not contain /
		{"team with spaces", false},    // spaces become _
	}

	for _, tt := range tests {
		result := sanitizeProfileID(tt.input)
		if strings.Contains(result, "..") {
			t.Errorf("sanitizeProfileID(%q) = %q still contains '..'", tt.input, result)
		}
		if strings.Contains(result, "/") {
			t.Errorf("sanitizeProfileID(%q) = %q still contains '/'", tt.input, result)
		}
	}
}
