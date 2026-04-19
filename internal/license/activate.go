package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Activate exchanges a customer license key for a signed JWS by calling
// the licensing server, verifies the result, and writes both the license
// file and the heartbeat state to disk.
func Activate(ctx context.Context, activationURL, licenseKey, installID, licensePath, statePath string) (*License, error) {
	body, err := json.Marshal(map[string]string{
		"license_key": licenseKey,
		"install_id":  installID,
	})
	if err != nil {
		return nil, fmt.Errorf("license activate: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", activationURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("license activate: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("license activate: status %d: %s", resp.StatusCode, string(respBody))
	}

	var ar struct {
		JWS string `json:"jws"`
	}
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return nil, fmt.Errorf("license activate: parse: %w", err)
	}
	lic, err := verifyJWS(ar.JWS)
	if err != nil {
		return nil, fmt.Errorf("license activate: verify: %w", err)
	}
	if err := lic.Validate(); err != nil {
		return nil, fmt.Errorf("license activate: %w", err)
	}

	if err := os.WriteFile(licensePath, []byte(ar.JWS), 0600); err != nil {
		return nil, fmt.Errorf("license: write license: %w", err)
	}
	if err := SaveState(statePath, &State{LastHeartbeat: time.Now().UTC(), LicenseID: lic.LicenseID}); err != nil {
		return nil, fmt.Errorf("license: write state: %w", err)
	}
	return lic, nil
}

// Deactivate removes the local license and state files.
// Server-side seat release is the caller's responsibility.
func Deactivate(licensePath, statePath string) error {
	for _, p := range []string{licensePath, statePath} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("license deactivate: %w", err)
		}
	}
	return nil
}

// LoadState reads the heartbeat state from disk. Returns a zero-value State
// (not an error) if the file is missing — the caller can treat that as
// "never heartbeated yet".
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveState writes the heartbeat state.
func SaveState(path string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
