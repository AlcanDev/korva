package config

import (
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
		{"falabella-financiero", true},
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
