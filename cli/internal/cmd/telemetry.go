package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/version"
)

// sendUsageEvent fires a best-effort telemetry event to the licensing server.
// Always runs in a goroutine — never blocks, never surfaces errors to the user.
// Only sends when a Teams license is active and install.id exists.
func sendUsageEvent(event string, metadata map[string]any) {
	go func() {
		paths, err := config.PlatformPaths()
		if err != nil {
			return
		}
		lic, err := license.Load(paths.LicenseFile)
		if err != nil || lic == nil {
			return // community tier — no telemetry
		}
		installID, err := identity.LoadInstallID(paths.InstallID)
		if err != nil || installID == "" {
			return
		}
		cfg, err := config.Load(paths.ConfigFile)
		if err != nil {
			return
		}

		usageURL := deriveUsageURL(cfg.License.ActivationURL)
		if usageURL == "" {
			return
		}

		payload := map[string]any{
			"license_id": lic.LicenseID,
			"install_id": installID,
			"event":      event,
			"version":    version.Version,
			"metadata":   metadata,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", usageURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()
}

// deriveUsageURL converts the activation URL to the usage URL.
// "https://licensing.korva.dev/v1/activate" → "https://licensing.korva.dev/v1/usage"
func deriveUsageURL(activationURL string) string {
	if idx := strings.LastIndex(activationURL, "/v1/"); idx != -1 {
		return activationURL[:idx] + "/v1/usage"
	}
	return "https://licensing.korva.dev/v1/usage"
}
