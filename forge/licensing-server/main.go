// Korva Licensing Server — production-grade license issuance and activation.
//
// # Environment variables
//
//	KORVA_LICENSING_PRIVATE_KEY_PEM   RSA private key (PEM content). Required.
//	KORVA_LICENSING_PRIVATE_KEY_FILE  Path to RSA private key file (alternative to above).
//	KORVA_LICENSING_ADMIN_SECRET      Programmatic Bearer token. Required.
//	KORVA_ADMIN_EMAIL                 Single admin email for web UI login. Required.
//	KORVA_ADMIN_PASSWORD              Web UI login password (defaults to admin secret if unset).
//	KORVA_SESSION_SECRET              HMAC key for session tokens (defaults to admin secret).
//	KORVA_ADMIN_CORS_ORIGIN           Allowed CORS origin for admin UI (default: *).
//	KORVA_LICENSING_KID               JWS key-id (default: korva-license-v1).
//	KORVA_LICENSING_DB                SQLite database path (default: ./licensing.db).
//	KORVA_LICENSING_ADDR              Listen address (default: :7440).
//
// # Quick start (development)
//
//	# 1. Generate a key pair (run once):
//	go run github.com/alcandev/korva/forge/licensing-server/keygen
//
//	# 2. Start the server:
//	KORVA_LICENSING_PRIVATE_KEY_FILE=./priv.pem \
//	KORVA_LICENSING_ADMIN_SECRET=change-me \
//	go run github.com/alcandev/korva/forge/licensing-server
//
// # Endpoints
//
//	GET  /v1/health                                              Health check
//	POST /v1/issue          (admin-only)                        Issue a new license key
//	POST /v1/activate       {license_key, install_id}           Activate → JWS
//	POST /v1/heartbeat      {license_id, install_id}            Refresh JWS every 24h
//	POST /v1/deactivate     {license_id, install_id}            Free a seat
//
//	GET  /v1/admin/licenses                                     List all licenses (paginated)
//	GET  /v1/admin/licenses/{id}                                Get license + activations
//	POST /v1/admin/licenses/{id}/revoke                         Revoke license
//	DELETE /v1/admin/licenses/{id}/revoke                       Un-revoke license
//	GET  /v1/admin/licenses/{id}/activations                    List active seats
//	DELETE /v1/admin/licenses/{id}/activations/{install_id}     Force-free a seat
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := envOr("KORVA_LICENSING_ADDR", ":7440")
	dbPath := envOr("KORVA_LICENSING_DB", "./licensing.db")
	kid := envOr("KORVA_LICENSING_KID", "korva-license-v1")
	secret := strings.TrimSpace(os.Getenv("KORVA_LICENSING_ADMIN_SECRET"))
	adminEmail := strings.TrimSpace(os.Getenv("KORVA_ADMIN_EMAIL"))
	adminPassword := strings.TrimSpace(os.Getenv("KORVA_ADMIN_PASSWORD"))
	sessionSecret := strings.TrimSpace(os.Getenv("KORVA_SESSION_SECRET"))
	corsOrigin := envOr("KORVA_ADMIN_CORS_ORIGIN", "*")

	if secret == "" {
		log.Fatal("KORVA_LICENSING_ADMIN_SECRET is required")
	}
	if adminEmail == "" {
		log.Fatal("KORVA_ADMIN_EMAIL is required")
	}
	// Fall back to admin secret if dedicated password not set.
	if adminPassword == "" {
		adminPassword = secret
	}
	// Fall back to admin secret as session signing key if not set.
	if sessionSecret == "" {
		sessionSecret = secret
	}

	privKey, err := loadPrivateKey()
	if err != nil {
		log.Fatalf("load private key: %v", err)
	}
	log.Printf("Private key loaded (kid=%s)", kid)

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	log.Printf("Database: %s", dbPath)
	log.Printf("Admin email: %s", adminEmail)

	srv := &server{
		db:            db,
		privKey:       privKey,
		kid:           kid,
		secret:        secret,
		adminEmail:    adminEmail,
		adminPassword: adminPassword,
		sessionSecret: sessionSecret,
		corsOrigin:    corsOrigin,
		limiter:       newRateLimiter(),
	}

	mux := http.NewServeMux()

	// Public endpoints — used by korva CLI clients.
	mux.HandleFunc("GET /v1/health", srv.withCORS(srv.handleHealth))
	mux.HandleFunc("POST /v1/issue", srv.withCORS(srv.handleIssue))
	mux.HandleFunc("POST /v1/activate", srv.withCORS(srv.handleActivate))
	mux.HandleFunc("POST /v1/heartbeat", srv.withCORS(srv.handleHeartbeat))
	mux.HandleFunc("POST /v1/deactivate", srv.withCORS(srv.handleDeactivate))

	// Web UI auth.
	mux.HandleFunc("POST /v1/admin/login", srv.withCORS(srv.handleAdminLogin))
	mux.HandleFunc("OPTIONS /v1/admin/login", srv.withCORS(func(w http.ResponseWriter, _ *http.Request) {}))

	// Admin endpoints — Bearer token (raw secret or session JWT).
	mux.HandleFunc("GET /v1/admin/licenses", srv.withCORS(srv.handleAdminListLicenses))
	mux.HandleFunc("GET /v1/admin/licenses/{id}", srv.withCORS(srv.handleAdminGetLicense))
	mux.HandleFunc("POST /v1/admin/licenses/{id}/revoke", srv.withCORS(srv.handleAdminRevokeLicense))
	mux.HandleFunc("DELETE /v1/admin/licenses/{id}/revoke", srv.withCORS(srv.handleAdminUnrevokeLicense))
	mux.HandleFunc("GET /v1/admin/licenses/{id}/activations", srv.withCORS(srv.handleAdminListActivations))
	mux.HandleFunc("DELETE /v1/admin/licenses/{id}/activations/{install_id}", srv.withCORS(srv.handleAdminForceDeactivate))

	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Korva Licensing Server listening on %s", addr)
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
