package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// GET /v1/admin/licenses?email=&limit=50&offset=0
//
// Lists all licenses with active seat counts. Optional ?email= filter.
func (s *server) handleAdminListLicenses(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	q := r.URL.Query()
	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(q.Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	licenses, total, err := listLicenses(s.db, q.Get("email"), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]map[string]any, 0, len(licenses))
	for _, lic := range licenses {
		active, _ := countActivations(s.db, lic.ID)
		item := map[string]any{
			"id":             lic.ID,
			"license_key":    lic.LicenseKey,
			"customer_email": lic.CustomerEmail,
			"tier":           lic.Tier,
			"seats":          lic.Seats,
			"features":       lic.Features,
			"grace_days":     lic.GraceDays,
			"expires_at":     lic.ExpiresAt.Format(time.RFC3339),
			"created_at":     lic.CreatedAt.Format(time.RFC3339),
			"active_seats":   active,
		}
		if lic.RevokedAt != nil {
			item["revoked_at"] = lic.RevokedAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"licenses": items,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GET /v1/admin/licenses/{id}
//
// Returns full license details including the list of active installations.
func (s *server) handleAdminGetLicense(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	lic, err := licenseByID(s.db, r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "license not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	activations, err := listActivations(s.db, lic.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := map[string]any{
		"id":             lic.ID,
		"license_key":    lic.LicenseKey,
		"customer_email": lic.CustomerEmail,
		"tier":           lic.Tier,
		"seats":          lic.Seats,
		"features":       lic.Features,
		"grace_days":     lic.GraceDays,
		"expires_at":     lic.ExpiresAt.Format(time.RFC3339),
		"created_at":     lic.CreatedAt.Format(time.RFC3339),
		"activations":    activations,
	}
	if lic.RevokedAt != nil {
		resp["revoked_at"] = lic.RevokedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /v1/admin/licenses/{id}/revoke
//
// Sets revoked_at to now. Subsequent activate and heartbeat calls will be
// rejected with 403. Existing active installations will degrade after their
// grace period expires.
func (s *server) handleAdminRevokeLicense(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	id := r.PathValue("id")
	if err := revokeLicense(s.db, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked", "id": id})
}

// DELETE /v1/admin/licenses/{id}/revoke
//
// Clears revoked_at — restores the license to active status.
func (s *server) handleAdminUnrevokeLicense(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	id := r.PathValue("id")
	if err := unrevokeLicense(s.db, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active", "id": id})
}

// GET /v1/admin/licenses/{id}/activations
//
// Returns all installations currently holding a seat for this license.
func (s *server) handleAdminListActivations(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	activations, err := listActivations(s.db, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activations": activations, "count": len(activations)})
}

// DELETE /v1/admin/licenses/{id}/activations/{install_id}
//
// Force-frees a specific seat. Useful when a developer leaves the org or a
// machine is decommissioned without running `korva license deactivate`.
func (s *server) handleAdminForceDeactivate(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth(r) {
		writeError(w, http.StatusUnauthorized, "invalid admin secret")
		return
	}
	if err := deleteActivation(s.db, r.PathValue("id"), r.PathValue("install_id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
