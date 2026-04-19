// Korva Licensing Server — production-grade license issuance and activation.
//
// # Environment variables
//
//	KORVA_LICENSING_PRIVATE_KEY_PEM   RSA private key (PEM content). Required.
//	KORVA_LICENSING_PRIVATE_KEY_FILE  Path to RSA private key file (alternative to above).
//	KORVA_LICENSING_ADMIN_SECRET      Secret for POST /v1/issue. Required.
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
//	GET  /v1/health                                    Health check
//	POST /v1/issue      (admin-only)                  Issue a new license key
//	POST /v1/activate   {license_key, install_id}     Activate → JWS
//	POST /v1/heartbeat  {license_id, install_id}      Refresh JWS every 24h
//	POST /v1/deactivate {license_id, install_id}      Free a seat
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

	if secret == "" {
		log.Fatal("KORVA_LICENSING_ADMIN_SECRET is required")
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

	srv := &server{
		db:      db,
		privKey: privKey,
		kid:     kid,
		secret:  secret,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", srv.handleHealth)
	mux.HandleFunc("POST /v1/issue", srv.handleIssue)
	mux.HandleFunc("POST /v1/activate", srv.handleActivate)
	mux.HandleFunc("POST /v1/heartbeat", srv.handleHeartbeat)
	mux.HandleFunc("POST /v1/deactivate", srv.handleDeactivate)

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
