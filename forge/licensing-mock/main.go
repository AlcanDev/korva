// Mock licensing server for local development only.
// Uses the dev RSA private key (dev_priv.pem) — NEVER ship to production.
//
// Endpoints:
//   POST /v1/activate   { license_key, install_id } → { jws }
//   POST /v1/heartbeat  { license_id, install_id }  → 200 OK
//   GET  /v1/health                                 → 200 OK
//
// Usage:  go run github.com/alcandev/korva/forge/licensing-mock
//         listens on :7439
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed dev_priv.pem
var devPrivKeyPEM []byte

var privKey *rsa.PrivateKey

func main() {
	var err error
	privKey, err = parseRSAPrivKey(devPrivKeyPEM)
	if err != nil {
		log.Fatalf("licensing-mock: load private key: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handleHealth)
	mux.HandleFunc("POST /v1/activate", handleActivate)
	mux.HandleFunc("POST /v1/heartbeat", handleHeartbeat)

	addr := ":7439"
	log.Printf("licensing-mock listening on %s (kid=korva-license-dev)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("licensing-mock: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","mock":true}`))
}

func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func handleActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseKey string `json:"license_key"`
		InstallID  string `json:"install_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	if req.LicenseKey == "" || req.InstallID == "" {
		http.Error(w, `{"error":"license_key and install_id required"}`, http.StatusBadRequest)
		return
	}

	idSnippet := req.InstallID
	if len(idSnippet) > 8 {
		idSnippet = idSnippet[:8]
	}
	payload := map[string]interface{}{
		"license_id": "lic-dev-" + idSnippet,
		"sub":        "team-dev",
		"tier":       "teams",
		"features": []string{
			"admin_skills",
			"custom_whitelist",
			"audit_log",
			"private_scrolls",
			"multi_profile",
			"cloud_private",
		},
		"iat":        time.Now().UTC(),
		"exp":        time.Now().AddDate(1, 0, 0).UTC(),
		"grace_days": 7,
		"seats":      10,
	}

	jws, err := signJWS(payload)
	if err != nil {
		log.Printf("licensing-mock: sign: %v", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"jws": jws})
}

func signJWS(payload map[string]interface{}) (string, error) {
	headerBytes, _ := json.Marshal(map[string]string{
		"alg": "RS256",
		"kid": "korva-license-dev",
	})
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	header64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header64 + "." + payload64

	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	sig64 := base64.RawURLEncoding.EncodeToString(sig)

	return strings.Join([]string{header64, payload64, sig64}, "."), nil
}

func parseRSAPrivKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("pem: no key block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("pem: not an RSA private key")
	}
	return rsaKey, nil
}
