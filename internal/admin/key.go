package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// AdminConfig holds the admin key and metadata.
// It is stored in ~/.korva/admin.key with permissions 0600.
// It is NEVER included in Git Sync manifests.
type AdminConfig struct {
	Key       string    `json:"key"`        // 32 bytes = 64 hex chars
	Owner     string    `json:"owner"`      // e.g., "felipe@alcandev"
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"version"` // increments on each rotation
}

var ErrNoAdminKey = errors.New("admin.key not found: this machine is not configured as admin")
var ErrKeyExists = errors.New("admin.key already exists: use 'korva admin rotate-key' to rotate")

// Generate creates a new admin key and writes it to keyPath.
// Returns ErrKeyExists if the file already exists (use force=true to overwrite).
func Generate(keyPath, owner string, force bool) (*AdminConfig, error) {
	if !force {
		if _, err := os.Stat(keyPath); err == nil {
			return nil, ErrKeyExists
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating secure random key: %w", err)
	}

	cfg := &AdminConfig{
		Key:       hex.EncodeToString(raw),
		Owner:     owner,
		CreatedAt: time.Now().UTC(),
		Version:   1,
	}

	if err := write(cfg, keyPath); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load reads the admin key from keyPath.
// Returns ErrNoAdminKey if the file does not exist.
func Load(keyPath string) (*AdminConfig, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoAdminKey
		}
		return nil, fmt.Errorf("reading admin key: %w", err)
	}

	var cfg AdminConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing admin key file: %w", err)
	}

	return &cfg, nil
}

// Rotate generates a new key, preserving the owner and incrementing the version.
// The old key must be provided for verification before rotation.
func Rotate(keyPath, currentKey string) (*AdminConfig, error) {
	existing, err := Load(keyPath)
	if err != nil {
		return nil, err
	}

	if !secureEqual(existing.Key, currentKey) {
		return nil, fmt.Errorf("current key verification failed")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating new key: %w", err)
	}

	cfg := &AdminConfig{
		Key:       hex.EncodeToString(raw),
		Owner:     existing.Owner,
		CreatedAt: time.Now().UTC(),
		Version:   existing.Version + 1,
	}

	if err := write(cfg, keyPath); err != nil {
		return nil, err
	}

	return cfg, nil
}

// write serializes AdminConfig to keyPath with 0600 permissions.
func write(cfg *AdminConfig, keyPath string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing admin config: %w", err)
	}

	// 0600: only the file owner can read and write
	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		return fmt.Errorf("writing admin key to %s: %w", keyPath, err)
	}

	return nil
}
