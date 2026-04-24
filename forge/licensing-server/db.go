package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// openDB opens (or creates) the SQLite database and applies all migrations.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=on")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		// licenses — one row per license sold to a customer.
		`CREATE TABLE IF NOT EXISTS licenses (
			id              TEXT PRIMARY KEY,
			license_key     TEXT NOT NULL UNIQUE,
			customer_email  TEXT NOT NULL,
			tier            TEXT NOT NULL DEFAULT 'teams',
			seats           INTEGER NOT NULL DEFAULT 5,
			features        TEXT NOT NULL DEFAULT '[]',
			grace_days      INTEGER NOT NULL DEFAULT 7,
			expires_at      TEXT NOT NULL,
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			revoked_at      TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_licenses_key   ON licenses(license_key)`,
		`CREATE INDEX IF NOT EXISTS idx_licenses_email ON licenses(customer_email)`,

		// activations — one row per active installation per license.
		// A single license can be active on up to `seats` installations simultaneously.
		`CREATE TABLE IF NOT EXISTS activations (
			id          TEXT PRIMARY KEY,
			license_id  TEXT NOT NULL,
			install_id  TEXT NOT NULL,
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			last_seen   TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (license_id) REFERENCES licenses(id),
			UNIQUE (license_id, install_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_activations_license ON activations(license_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// ─── Models ──────────────────────────────────────────────────────────────────

type dbLicense struct {
	ID            string
	LicenseKey    string
	CustomerEmail string
	Tier          string
	Seats         int
	Features      []string
	GraceDays     int
	ExpiresAt     time.Time
	CreatedAt     time.Time
	RevokedAt     *time.Time
}

// ─── Queries ─────────────────────────────────────────────────────────────────

// createLicense inserts a new license record and returns it.
func createLicense(db *sql.DB, email, tier string, seats, graceDays, expireDays int, features []string) (*dbLicense, error) {
	id := newLicenseID()
	key := generateLicenseKey()
	featJSON, _ := json.Marshal(features)
	expiresAt := time.Now().UTC().AddDate(0, 0, expireDays)

	_, err := db.Exec(
		`INSERT INTO licenses(id, license_key, customer_email, tier, seats, features, grace_days, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, key, email, tier, seats, string(featJSON), graceDays, expiresAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return &dbLicense{
		ID:            id,
		LicenseKey:    key,
		CustomerEmail: email,
		Tier:          tier,
		Seats:         seats,
		Features:      features,
		GraceDays:     graceDays,
		ExpiresAt:     expiresAt,
	}, nil
}

// licenseByKey finds a license by its human-readable key.
// Returns sql.ErrNoRows when not found.
func licenseByKey(db *sql.DB, key string) (*dbLicense, error) {
	var lic dbLicense
	var featJSON, expiresStr string
	var revokedStr *string

	err := db.QueryRow(
		`SELECT id, license_key, customer_email, tier, seats, features, grace_days, expires_at, revoked_at
		   FROM licenses WHERE license_key = ?`, key).
		Scan(&lic.ID, &lic.LicenseKey, &lic.CustomerEmail, &lic.Tier,
			&lic.Seats, &featJSON, &lic.GraceDays, &expiresStr, &revokedStr)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(featJSON), &lic.Features) //nolint:errcheck
	lic.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	if revokedStr != nil {
		t, _ := time.Parse(time.RFC3339, *revokedStr)
		lic.RevokedAt = &t
	}
	return &lic, nil
}

// licenseByID finds a license by its internal ID.
func licenseByID(db *sql.DB, id string) (*dbLicense, error) {
	var lic dbLicense
	var featJSON, expiresStr string
	var revokedStr *string

	err := db.QueryRow(
		`SELECT id, license_key, customer_email, tier, seats, features, grace_days, expires_at, revoked_at
		   FROM licenses WHERE id = ?`, id).
		Scan(&lic.ID, &lic.LicenseKey, &lic.CustomerEmail, &lic.Tier,
			&lic.Seats, &featJSON, &lic.GraceDays, &expiresStr, &revokedStr)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(featJSON), &lic.Features) //nolint:errcheck
	lic.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	if revokedStr != nil {
		t, _ := time.Parse(time.RFC3339, *revokedStr)
		lic.RevokedAt = &t
	}
	return &lic, nil
}

// countActivations returns the number of active seats for a license.
func countActivations(db *sql.DB, licenseID string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM activations WHERE license_id = ?`, licenseID).Scan(&n)
	return n, err
}

// upsertActivation inserts or updates (last_seen) the activation record.
func upsertActivation(db *sql.DB, licenseID, installID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	id := newLicenseID()
	_, err := db.Exec(
		`INSERT INTO activations(id, license_id, install_id, created_at, last_seen) VALUES(?,?,?,?,?)
		 ON CONFLICT(license_id, install_id) DO UPDATE SET last_seen = excluded.last_seen`,
		id, licenseID, installID, now, now)
	return err
}

// deleteActivation removes an activation (deactivate).
func deleteActivation(db *sql.DB, licenseID, installID string) error {
	_, err := db.Exec(
		`DELETE FROM activations WHERE license_id = ? AND install_id = ?`,
		licenseID, installID)
	return err
}

// listLicenses returns a paginated slice of licenses with optional email filter.
func listLicenses(db *sql.DB, email string, limit, offset int) ([]dbLicense, int, error) {
	var (
		where string
		args  []any
	)
	if email != "" {
		where = " WHERE customer_email LIKE ?"
		args = append(args, "%"+email+"%")
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM licenses"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(
		"SELECT id, license_key, customer_email, tier, seats, features, grace_days, expires_at, created_at, revoked_at"+
			" FROM licenses"+where+
			" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var licenses []dbLicense
	for rows.Next() {
		var lic dbLicense
		var featJSON, expiresStr, createdStr string
		var revokedStr *string
		if err := rows.Scan(&lic.ID, &lic.LicenseKey, &lic.CustomerEmail, &lic.Tier,
			&lic.Seats, &featJSON, &lic.GraceDays, &expiresStr, &createdStr, &revokedStr); err != nil {
			return nil, 0, err
		}
		json.Unmarshal([]byte(featJSON), &lic.Features) //nolint:errcheck
		lic.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
		lic.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		if revokedStr != nil {
			t, _ := time.Parse(time.RFC3339, *revokedStr)
			lic.RevokedAt = &t
		}
		licenses = append(licenses, lic)
	}
	return licenses, total, rows.Err()
}

// revokeLicense sets revoked_at to now for the given license ID.
func revokeLicense(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE licenses SET revoked_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// unrevokeLicense clears revoked_at, restoring the license to active status.
func unrevokeLicense(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE licenses SET revoked_at = NULL WHERE id = ?`, id)
	return err
}

type dbActivation struct {
	ID        string    `json:"id"`
	LicenseID string    `json:"license_id"`
	InstallID string    `json:"install_id"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// listActivations returns all active seats for a license.
func listActivations(db *sql.DB, licenseID string) ([]dbActivation, error) {
	rows, err := db.Query(
		`SELECT id, license_id, install_id, created_at, last_seen
		   FROM activations WHERE license_id = ? ORDER BY last_seen DESC`, licenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dbActivation
	for rows.Next() {
		var a dbActivation
		var createdStr, lastSeenStr string
		if err := rows.Scan(&a.ID, &a.LicenseID, &a.InstallID, &createdStr, &lastSeenStr); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		a.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newLicenseID generates a short random hex ID.
func newLicenseID() string {
	b := make([]byte, 12)
	rand.Read(b) //nolint:errcheck
	return "lic_" + hex.EncodeToString(b)
}

// generateLicenseKey returns a human-readable key in the format
// KORVA-XXXX-XXXX-XXXX-XXXX using 8 random bytes encoded as hex uppercase.
func generateLicenseKey() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	s := strings.ToUpper(hex.EncodeToString(b)) // 16 hex chars
	return fmt.Sprintf("KORVA-%s-%s-%s-%s", s[0:4], s[4:8], s[8:12], s[12:16])
}
