package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/license"
)

// licenseStatusHandler returns the current license tier, features, and grace window.
// Returns community tier JSON when no license is installed.
func licenseStatusHandler(lic *license.License, statePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if lic == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"tier":     "community",
				"features": []string{},
				"grace_ok": true,
			})
			return
		}

		state, err := license.LoadState(statePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load state: "+err.Error())
			return
		}

		tier := lic.CurrentTier(state)
		rem := lic.GraceRemaining(state)

		resp := map[string]any{
			"license_id": lic.LicenseID,
			"tier":       tier,
			"features":   lic.Features,
			"seats":      lic.Seats,
			"grace_ok":   tier == lic.Tier,
		}
		if !lic.ExpiresAt.IsZero() {
			resp["expires_at"] = lic.ExpiresAt.Format(time.RFC3339)
		}
		if !state.LastHeartbeat.IsZero() {
			resp["last_heartbeat"] = state.LastHeartbeat.Format(time.RFC3339)
		}
		if rem > 0 {
			resp["grace_remaining_hours"] = int(rem.Hours())
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// licenseActivateHandler exchanges a license key for a signed JWS via the
// licensing server, persists it to disk, and returns the new tier.
func licenseActivateHandler(activationURL, installID, licensePath, statePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			LicenseKey string `json:"license_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LicenseKey == "" {
			writeError(w, http.StatusBadRequest, "license_key is required")
			return
		}
		if activationURL == "" {
			writeError(w, http.StatusServiceUnavailable, "activation_url not configured")
			return
		}
		lic, err := license.Activate(r.Context(), activationURL, body.LicenseKey, installID, licensePath, statePath)
		if err != nil {
			writeError(w, http.StatusBadGateway, "activation failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "activated",
			"tier":   lic.Tier,
			"seats":  lic.Seats,
		})
	}
}

// licenseDeactivateHandler removes the local license and state files, freeing
// the installation. The caller is responsible for notifying the licensing
// server separately if a server-side seat release is needed.
func licenseDeactivateHandler(licensePath, statePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := license.Deactivate(licensePath, statePath); err != nil {
			writeError(w, http.StatusInternalServerError, "deactivate failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
	}
}

// requireFeature returns a middleware that rejects requests when the license
// does not include featureName. Returns 402 Payment Required.
func requireFeature(lic *license.License, featureName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !lic.HasFeature(featureName) {
				writeError(w, http.StatusPaymentRequired, "feature '"+featureName+"' requires Korva for Teams license")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
