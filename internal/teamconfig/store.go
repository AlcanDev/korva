package teamconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SyncState records the last successful sync so subsequent calls can skip
// unchanged files.
type SyncState struct {
	SyncedAt      time.Time `json:"synced_at"`
	BundleVersion string    `json:"bundle_version"` // latest updated_at from last bundle
	LicenseID     string    `json:"license_id"`
	ItemCount     int       `json:"item_count"`
}

// WriteResult summarizes what was written during a sync.
type WriteResult struct {
	Written int // files created or updated
	Skipped int // files whose hash matched — skipped
	Deleted int // files removed because they were not in the bundle
}

// WriteBundleToDisk persists bundle items under configDir.
// Layout: <configDir>/<section>/<name>
//
// Items with unchanged hash are skipped (idempotent).
// Files on disk that are no longer in the bundle are removed.
// Returns a summary of what changed.
func WriteBundleToDisk(configDir string, bundle *Bundle) (WriteResult, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return WriteResult{}, fmt.Errorf("teamconfig: ensure dir: %w", err)
	}

	// Index bundle items by "section/name" for O(1) lookup during cleanup.
	bundleIndex := make(map[string]BundleItem, len(bundle.Items))
	for _, item := range bundle.Items {
		bundleIndex[item.Section+"/"+item.Name] = item
	}

	var result WriteResult

	// Write new or changed items.
	for _, item := range bundle.Items {
		if err := writeItem(configDir, item, &result); err != nil {
			return result, err
		}
	}

	// Remove files that are no longer in the bundle.
	if err := pruneStale(configDir, bundleIndex, &result); err != nil {
		return result, err
	}

	return result, nil
}

// LoadSyncState reads the sync state from disk.
// Returns zero value + nil error when the file doesn't exist yet.
func LoadSyncState(stateFile string) (SyncState, error) {
	data, err := os.ReadFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return SyncState{}, nil
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("teamconfig: read sync state: %w", err)
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		return SyncState{}, fmt.Errorf("teamconfig: parse sync state: %w", err)
	}
	return s, nil
}

// SaveSyncState writes the sync state to disk (0600).
func SaveSyncState(stateFile string, s SyncState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("teamconfig: marshal sync state: %w", err)
	}
	if err := os.WriteFile(stateFile, data, 0600); err != nil {
		return fmt.Errorf("teamconfig: write sync state: %w", err)
	}
	return nil
}

// SaveTeamKey writes the raw license key to disk with 0600 permissions.
func SaveTeamKey(keyFile, licenseKey string) error {
	if err := os.WriteFile(keyFile, []byte(strings.TrimSpace(licenseKey)), 0600); err != nil {
		return fmt.Errorf("teamconfig: write team key: %w", err)
	}
	return nil
}

// LoadTeamKey reads the raw license key from disk.
// Returns empty string + nil when the file doesn't exist.
func LoadTeamKey(keyFile string) (string, error) {
	data, err := os.ReadFile(keyFile)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("teamconfig: read team key: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ─── Internal ────────────────────────────────────────────────────────────────

func writeItem(configDir string, item BundleItem, result *WriteResult) error {
	// Validate section and name to prevent path traversal.
	if !isSafeSegment(item.Section) || !isSafePath(item.Name) {
		return fmt.Errorf("teamconfig: unsafe item path: %q/%q", item.Section, item.Name)
	}

	dir := filepath.Join(configDir, item.Section, filepath.Dir(item.Name))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("teamconfig: mkdir %s: %w", dir, err)
	}

	dest := filepath.Join(configDir, item.Section, filepath.FromSlash(item.Name))

	// Skip if existing file content matches (compare via hash stored in sidecar).
	if existingHash, err := readHashSidecar(dest); err == nil && existingHash == item.Hash {
		result.Skipped++
		return nil
	}

	if err := os.WriteFile(dest, []byte(item.Content), 0600); err != nil {
		return fmt.Errorf("teamconfig: write %s: %w", dest, err)
	}
	if err := writeHashSidecar(dest, item.Hash); err != nil {
		return err
	}
	result.Written++
	return nil
}

// pruneStale removes files under configDir that are no longer in bundleIndex.
func pruneStale(configDir string, bundleIndex map[string]BundleItem, result *WriteResult) error {
	for _, section := range []string{"scrolls", "rules", "instructions", "skills", "settings"} {
		sectionDir := filepath.Join(configDir, section)
		if _, err := os.Stat(sectionDir); errors.Is(err, os.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(sectionDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(path, ".hash") {
				return err
			}
			rel, _ := filepath.Rel(configDir, path)
			rel = filepath.ToSlash(rel) // normalize to forward slashes
			if _, inBundle := bundleIndex[rel]; !inBundle {
				os.Remove(path)                                    //nolint:errcheck
				os.Remove(path + ".hash")                          //nolint:errcheck
				result.Deleted++
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("teamconfig: prune walk: %w", err)
		}
	}
	return nil
}

// Hash sidecars store the SHA-256 of the last-written content next to each
// file (e.g., "api-patterns.md.hash"). This lets WriteBundleToDisk skip
// unchanged files without re-reading and re-hashing the file itself.

func hashSidecarPath(filePath string) string { return filePath + ".hash" }

func readHashSidecar(filePath string) (string, error) {
	data, err := os.ReadFile(hashSidecarPath(filePath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeHashSidecar(filePath, hash string) error {
	if err := os.WriteFile(hashSidecarPath(filePath), []byte(hash), 0600); err != nil {
		return fmt.Errorf("teamconfig: write hash sidecar: %w", err)
	}
	return nil
}

// isSafeSegment checks that a path segment contains only safe characters.
func isSafeSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !isNameChar(c) {
			return false
		}
	}
	return true
}

// isSafePath checks an item name (may include forward slashes for nesting).
func isSafePath(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	parts := strings.Split(s, "/")
	for _, p := range parts {
		if !isSafeSegment(p) {
			return false
		}
	}
	return true
}

func isNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.'
}
