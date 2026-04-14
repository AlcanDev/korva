package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alcandev/korva/internal/config"
)

// Manager handles team profile lifecycle: clone, validate, apply, sync.
type Manager struct {
	paths *config.Paths
}

// NewManager creates a ProfileManager using the given platform paths.
func NewManager(paths *config.Paths) *Manager {
	return &Manager{paths: paths}
}

// Clone clones a remote profile repository to the local profiles directory.
// It uses the profile ID from team-profile.json to name the local directory.
func (m *Manager) Clone(repoURL string) (string, error) {
	// First, clone to a temp dir to read the profile ID
	tmp, err := os.MkdirTemp("", "korva-profile-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := gitClone(repoURL, tmp); err != nil {
		return "", fmt.Errorf("cloning profile repo: %w", err)
	}

	profilePath := filepath.Join(tmp, "team-profile.json")
	profile, err := config.LoadTeamProfile(profilePath)
	if err != nil {
		return "", fmt.Errorf("reading team-profile.json from cloned repo: %w", err)
	}

	if err := Validate(profile); err != nil {
		return "", fmt.Errorf("invalid team profile: %w", err)
	}

	targetDir := m.paths.ProfileDir(profile.Profile.ID)

	// If already exists, remove it before re-cloning
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return "", fmt.Errorf("removing existing profile dir: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return "", fmt.Errorf("creating profiles directory: %w", err)
	}

	if err := gitClone(repoURL, targetDir); err != nil {
		return "", fmt.Errorf("cloning to target dir: %w", err)
	}

	return targetDir, nil
}

// Apply loads a profile from profileDir, validates it, merges overrides onto
// baseCfg, and writes the result to ~/.korva/config.json.
func (m *Manager) Apply(profileDir string, baseCfg config.KorvaConfig) (config.KorvaConfig, error) {
	profile, err := config.LoadTeamProfile(filepath.Join(profileDir, "team-profile.json"))
	if err != nil {
		return config.KorvaConfig{}, fmt.Errorf("loading team profile: %w", err)
	}

	if err := Validate(profile); err != nil {
		return config.KorvaConfig{}, fmt.Errorf("validating team profile: %w", err)
	}

	merged := ApplyOverrides(baseCfg, profile)

	if err := config.Save(merged, m.paths.ConfigFile); err != nil {
		return config.KorvaConfig{}, fmt.Errorf("saving merged config: %w", err)
	}

	return merged, nil
}

// InstallScrolls copies private scrolls from profileDir/scrolls/ to
// ~/.korva/lore/private/.
func (m *Manager) InstallScrolls(profileDir string) error {
	scrollsSource := filepath.Join(profileDir, "scrolls")
	scrollsDest := m.paths.PrivateLoreDir()

	if _, err := os.Stat(scrollsSource); os.IsNotExist(err) {
		// No private scrolls in this profile — that's fine
		return nil
	}

	if err := os.MkdirAll(scrollsDest, 0755); err != nil {
		return fmt.Errorf("creating private lore dir: %w", err)
	}

	return copyDir(scrollsSource, scrollsDest)
}

// MergeInstructions appends team-specific AI instructions to the project's
// instruction files. It uses idempotent markers so running it multiple times
// is safe — it replaces the existing block if present.
func (m *Manager) MergeInstructions(profileDir, projectDir string) error {
	profile, err := config.LoadTeamProfile(filepath.Join(profileDir, "team-profile.json"))
	if err != nil {
		return fmt.Errorf("loading team profile: %w", err)
	}

	overrides := profile.Overrides.Instructions
	if overrides == nil {
		return nil
	}

	profileID := profile.Profile.ID
	strategy := overrides.MergeStrategy
	if strategy == "" {
		strategy = "append"
	}

	// Merge copilot extensions
	if overrides.CopilotExtensions != "" {
		src := filepath.Join(profileDir, overrides.CopilotExtensions)
		dst := filepath.Join(projectDir, ".github", "copilot-instructions.md")
		if err := mergeInstructionFile(src, dst, profileID, strategy); err != nil {
			return fmt.Errorf("merging copilot instructions: %w", err)
		}
	}

	// Merge claude extensions
	if overrides.ClaudeExtensions != "" {
		src := filepath.Join(profileDir, overrides.ClaudeExtensions)
		dst := filepath.Join(projectDir, "CLAUDE.md")
		if err := mergeInstructionFile(src, dst, profileID, strategy); err != nil {
			return fmt.Errorf("merging claude instructions: %w", err)
		}
	}

	return nil
}

// Sync pulls the latest changes from the remote profile and re-applies.
func (m *Manager) Sync(profileID string, baseCfg config.KorvaConfig) (config.KorvaConfig, error) {
	profileDir := m.paths.ProfileDir(profileID)

	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return config.KorvaConfig{}, fmt.Errorf("profile %q not found: run 'korva init --profile <url>' first", profileID)
	}

	if err := gitPull(profileDir); err != nil {
		return config.KorvaConfig{}, fmt.Errorf("pulling profile updates: %w", err)
	}

	return m.Apply(profileDir, baseCfg)
}

// ActiveProfileID returns the profile ID from ~/.korva/config.json, if set.
func (m *Manager) ActiveProfileID() (string, error) {
	type activeProfile struct {
		ProfileID string `json:"_active_profile_id"`
	}
	data, err := os.ReadFile(m.paths.ConfigFile)
	if err != nil {
		return "", nil
	}
	var ap activeProfile
	json.Unmarshal(data, &ap)
	return ap.ProfileID, nil
}

// mergeInstructionFile appends/replaces a block in dst using idempotent markers.
func mergeInstructionFile(src, dst, profileID, strategy string) error {
	extension, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading extension file %s: %w", src, err)
	}

	beginMarker := fmt.Sprintf("<!-- korva:team-extensions:%s:begin -->", profileID)
	endMarker := fmt.Sprintf("<!-- korva:team-extensions:%s:end -->", profileID)
	block := "\n" + beginMarker + "\n" + string(extension) + "\n" + endMarker + "\n"

	// Read existing destination file
	var existing string
	if data, err := os.ReadFile(dst); err == nil {
		existing = string(data)
	}

	var result string
	if strings.Contains(existing, beginMarker) {
		// Replace existing block
		startIdx := strings.Index(existing, beginMarker)
		endIdx := strings.Index(existing, endMarker)
		if endIdx == -1 {
			// Malformed: just append
			result = existing + block
		} else {
			result = existing[:startIdx] + strings.TrimLeft(block, "\n") + existing[endIdx+len(endMarker):]
		}
	} else {
		// Append new block
		result = existing + block
	}

	return os.WriteFile(dst, []byte(result), 0644)
}

// gitClone runs git clone <url> <dir>.
func gitClone(url, dir string) error {
	cmd := exec.Command("git", "clone", "--depth=1", url, dir)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitPull runs git pull in the given directory.
func gitPull(dir string) error {
	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// copyDir recursively copies src directory contents into dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
