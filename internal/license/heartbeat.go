package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const heartbeatInterval = 24 * time.Hour

// HeartbeatRequest is the payload sent to the licensing server.
type HeartbeatRequest struct {
	LicenseID string `json:"license_id"`
	InstallID string `json:"install_id"`
}

// RunHeartbeat starts the background goroutine that re-validates the license
// every 24h. It fires once immediately on start then on the ticker.
// Returns immediately; call cancel() to stop.
func RunHeartbeat(ctx context.Context, heartbeatURL, installID, statePath string, lic *License) {
	if lic == nil {
		return
	}
	go func() {
		tick := time.NewTicker(heartbeatInterval)
		defer tick.Stop()
		// fire once immediately
		if err := sendHeartbeat(ctx, heartbeatURL, installID, statePath, lic); err != nil {
			log.Printf("license heartbeat: %v", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := sendHeartbeat(ctx, heartbeatURL, installID, statePath, lic); err != nil {
					log.Printf("license heartbeat: %v", err)
				}
			}
		}
	}()
}

func sendHeartbeat(ctx context.Context, heartbeatURL, installID, statePath string, lic *License) error {
	body, _ := json.Marshal(HeartbeatRequest{
		LicenseID: lic.LicenseID,
		InstallID: installID,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", heartbeatURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	state, err := LoadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	state.LastHeartbeat = time.Now().UTC()
	return SaveState(statePath, state)
}
