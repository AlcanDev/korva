// Package identity manages per-installation identifiers used by Korva
// to talk to cloud services (Hive, licensing) without revealing user data.
//
// Files (under ~/.korva/):
//   - install.id  — stable, opaque per-installation ID (used as HMAC salt for anonymization)
//   - hive.key    — auth token for the community Hive cloud
//
// Both are generated once on `korva init` and never rotated automatically.
// Both are stored with permissions 0600 and excluded from sync/manifests.
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// EnsureInstallID returns the install ID at path, creating it if missing.
// The ID is 16 random bytes encoded as 32 hex chars — opaque, not reversible.
func EnsureInstallID(path string) (string, error) {
	return ensureHexFile(path, 16)
}

// EnsureHiveKey returns the Hive API key at path, creating it if missing.
// The key is 32 random bytes encoded as 64 hex chars.
func EnsureHiveKey(path string) (string, error) {
	return ensureHexFile(path, 32)
}

// LoadInstallID reads the install ID from path. Returns an error if missing.
func LoadInstallID(path string) (string, error) {
	return readHexFile(path)
}

// LoadHiveKey reads the Hive key from path. Returns an error if missing.
func LoadHiveKey(path string) (string, error) {
	return readHexFile(path)
}

// RotateHiveKey overwrites the Hive key at path with new random bytes.
// The caller must coordinate revocation server-side.
func RotateHiveKey(path string) (string, error) {
	return writeHexFile(path, 32)
}

func ensureHexFile(path string, n int) (string, error) {
	if existing, err := readHexFile(path); err == nil {
		return existing, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return writeHexFile(path, n)
}

func writeHexFile(path string, n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	encoded := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(encoded+"\n"), 0600); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return encoded, nil
}

func readHexFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
