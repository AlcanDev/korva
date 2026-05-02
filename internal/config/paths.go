package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// Paths holds all filesystem paths used by Korva.
// Always obtain via PlatformPaths() — never hardcode separators.
type Paths struct {
	HomeDir          string
	ConfigFile       string
	AdminKey         string
	HiveKey          string
	InstallID        string
	LicenseFile      string
	LicenseStateFile string
	SessionTokenFile string // ~/.korva/session.token — member auth session
	SkillsSyncFile   string // ~/.korva/skills_sync.json — last successful sync timestamp
	ProfilesDir      string
	LoreDir          string
	VaultDir         string
	LogsDir          string
}

// PlatformPaths returns the correct base paths for the current OS.
//
//   - macOS/Linux: ~/.korva/
//   - Windows:     %APPDATA%\korva\
func PlatformPaths() (*Paths, error) {
	base, err := baseDir()
	if err != nil {
		return nil, err
	}

	return &Paths{
		HomeDir:          base,
		ConfigFile:       filepath.Join(base, "config.json"),
		AdminKey:         filepath.Join(base, "admin.key"),
		HiveKey:          filepath.Join(base, "hive.key"),
		InstallID:        filepath.Join(base, "install.id"),
		LicenseFile:      filepath.Join(base, "license.key"),
		LicenseStateFile: filepath.Join(base, "license.state.json"),
		SessionTokenFile: filepath.Join(base, "session.token"),
		SkillsSyncFile:   filepath.Join(base, "skills_sync.json"),
		ProfilesDir:      filepath.Join(base, "profiles"),
		LoreDir:          filepath.Join(base, "lore"),
		VaultDir:         filepath.Join(base, "vault"),
		LogsDir:          filepath.Join(base, "logs"),
	}, nil
}

// VaultDB returns the path to the main SQLite database.
func (p *Paths) VaultDB() string {
	return filepath.Join(p.VaultDir, "observations.db")
}

// VaultPIDFile returns the path to the vault server PID file.
// Used by `korva vault start/stop/status` to track the running process.
func (p *Paths) VaultPIDFile() string {
	return filepath.Join(p.VaultDir, "vault.pid")
}

// VaultLogFile returns the path to the vault server log file.
func (p *Paths) VaultLogFile() string {
	return filepath.Join(p.LogsDir, "vault.log")
}

// ProfileDir returns the local clone path for a given profile ID.
// The profile ID is sanitized to prevent path traversal.
func (p *Paths) ProfileDir(profileID string) string {
	safe := sanitizeProfileID(profileID)
	return filepath.Join(p.ProfilesDir, safe)
}

// PrivateLoreDir returns the directory for private/team scrolls.
func (p *Paths) PrivateLoreDir() string {
	return filepath.Join(p.LoreDir, "private")
}

// PublicLoreDir returns the directory for public curated scrolls.
func (p *Paths) PublicLoreDir() string {
	return filepath.Join(p.LoreDir, "public")
}

// EnsureAll creates all required directories if they don't exist.
func (p *Paths) EnsureAll() error {
	dirs := []string{
		p.HomeDir,
		p.ProfilesDir,
		p.LoreDir,
		p.PrivateLoreDir(),
		p.PublicLoreDir(),
		p.VaultDir,
		p.LogsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", dir, err)
		}
	}

	return nil
}

// baseDir returns the platform-specific base directory for korva data.
// KORVA_HOME overrides the default when set — useful for container deployments
// where the home directory is ephemeral but a persistent volume is available.
func baseDir() (string, error) {
	if override := os.Getenv("KORVA_HOME"); override != "" {
		return override, nil
	}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}
		return filepath.Join(appData, "korva"), nil
	default:
		// macOS and Linux
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, ".korva"), nil
	}
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

// sanitizeProfileID removes characters that could cause path traversal.
func sanitizeProfileID(id string) string {
	return unsafeChars.ReplaceAllString(id, "_")
}
