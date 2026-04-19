package main

import (
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// server holds shared dependencies injected at startup.
type server struct {
	db      *sql.DB
	privKey *rsa.PrivateKey
	kid     string   // JWS key id embedded in client binaries
	secret  string   // KORVA_LICENSING_ADMIN_SECRET — protects /v1/issue
}

// ─── Health ──────────────────────────────────────────────────────────────────

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "korva-licensing"})
}

// ─── Issue (admin-only) ───────────────────────────────────────────────────────

// POST /v1/issue — creates a new license record.
// Protected by Authorization: Bearer {KORVA_LICENSING_ADMIN_SECRET}.
//
// Request body:
//
//	{
//	  "customer_email": "alice@corp.com",
//	  "seats":          10,
//	  "features":       ["admin_skills", "audit_log", "private_scrolls", "multi_profile"],
//	  "expire_days":    365,
//	  "tier":           "teams",       // optional, default "teams"
//	  "grace_days":     7              // optional, default 7
//	}
func (s *server) handleIssue(w http.ResponseWriter, r *http.Request) {
	// Admin secret auth.
	if s.secret == "" || r.Header.Get("Authorization") != "Bearer "+s.secret {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}

	var req struct {
		CustomerEmail string   `json:"customer_email"`
		Seats         int      `json:"seats"`
		Features      []string `json:"features"`
		ExpireDays    int      `json:"expire_days"`
		Tier          string   `json:"tier"`
		GraceDays     int      `json:"grace_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CustomerEmail == "" {
		writeError(w, http.StatusBadRequest, "customer_email is required")
		return
	}
	if req.Seats <= 0 {
		req.Seats = 5
	}
	if req.ExpireDays <= 0 {
		req.ExpireDays = 365
	}
	if req.Tier == "" {
		req.Tier = "teams"
	}
	if req.GraceDays <= 0 {
		req.GraceDays = 7
	}
	if len(req.Features) == 0 {
		req.Features = defaultTeamsFeatures()
	}

	lic, err := createLicense(s.db, req.CustomerEmail, req.Tier, req.Seats, req.GraceDays, req.ExpireDays, req.Features)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"license_id":     lic.ID,
		"license_key":    lic.LicenseKey,
		"customer_email": lic.CustomerEmail,
		"tier":           lic.Tier,
		"seats":          lic.Seats,
		"features":       lic.Features,
		"expires_at":     lic.ExpiresAt.Format(time.RFC3339),
		"note":           "send the license_key to the customer — they run: korva license activate <key>",
	})
}

// ─── Activate ────────────────────────────────────────────────────────────────

// POST /v1/activate — client redeems their license key, receives a signed JWS.
//
// Request: {"license_key": "KORVA-ABCD-...", "install_id": "01JXX..."}
// Response: {"jws": "..."}
func (s *server) handleActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseKey string `json:"license_key"`
		InstallID  string `json:"install_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.LicenseKey = strings.ToUpper(strings.TrimSpace(req.LicenseKey))
	req.InstallID = strings.TrimSpace(req.InstallID)
	if req.LicenseKey == "" || req.InstallID == "" {
		writeError(w, http.StatusBadRequest, "license_key and install_id are required")
		return
	}

	lic, err := licenseByKey(s.db, req.LicenseKey)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "license key not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lic.RevokedAt != nil {
		writeError(w, http.StatusForbidden, "this license has been revoked")
		return
	}
	if time.Now().UTC().After(lic.ExpiresAt) {
		writeError(w, http.StatusPaymentRequired, "license has expired — please renew")
		return
	}

	// Seat enforcement: check if this install_id is already counted; only
	// reject new installs over the seat limit.
	existing, err := countActivations(s.db, lic.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Check if this install_id already has an activation (renewal is always allowed).
	var alreadyActive int
	s.db.QueryRow( //nolint:errcheck
		`SELECT COUNT(*) FROM activations WHERE license_id = ? AND install_id = ?`,
		lic.ID, req.InstallID).Scan(&alreadyActive)

	if alreadyActive == 0 && existing >= lic.Seats {
		writeError(w, http.StatusPaymentRequired,
			"seat limit reached — deactivate another installation or upgrade your plan")
		return
	}

	if err := upsertActivation(s.db, lic.ID, req.InstallID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jws, err := s.buildJWS(lic)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "jws signing failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"jws": jws})
}

// ─── Heartbeat ───────────────────────────────────────────────────────────────

// POST /v1/heartbeat — refreshes a JWS every 24 h.
//
// Request: {"license_id": "lic_...", "install_id": "01JXX..."}
// Response: {"jws": "...", "ok": true}
func (s *server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseID string `json:"license_id"`
		InstallID string `json:"install_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.LicenseID == "" || req.InstallID == "" {
		writeError(w, http.StatusBadRequest, "license_id and install_id are required")
		return
	}

	lic, err := licenseByID(s.db, req.LicenseID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "license not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lic.RevokedAt != nil {
		writeError(w, http.StatusForbidden, "license has been revoked")
		return
	}

	// Touch last_seen so admins can see activity.
	upsertActivation(s.db, lic.ID, req.InstallID) //nolint:errcheck

	jws, err := s.buildJWS(lic)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "jws signing failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jws": jws, "ok": true})
}

// ─── Deactivate ──────────────────────────────────────────────────────────────

// POST /v1/deactivate — removes an activation from the seat count.
//
// Request: {"license_id": "lic_...", "install_id": "01JXX..."}
func (s *server) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseID string `json:"license_id"`
		InstallID string `json:"install_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.LicenseID == "" || req.InstallID == "" {
		writeError(w, http.StatusBadRequest, "license_id and install_id are required")
		return
	}

	if err := deleteActivation(s.db, req.LicenseID, req.InstallID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// buildJWS creates the signed token from a dbLicense.
func (s *server) buildJWS(lic *dbLicense) (string, error) {
	payload := map[string]any{
		"license_id": lic.ID,
		"sub":        lic.ID,
		"tier":       lic.Tier,
		"features":   lic.Features,
		"iat":        time.Now().UTC(),
		"exp":        lic.ExpiresAt.UTC(),
		"grace_days": lic.GraceDays,
		"seats":      lic.Seats,
	}
	return signJWS(s.privKey, s.kid, payload)
}

// defaultTeamsFeatures returns the standard Teams feature set.
func defaultTeamsFeatures() []string {
	return []string{
		"admin_skills",
		"custom_whitelist",
		"audit_log",
		"private_scrolls",
		"multi_profile",
		"cloud_private",
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
