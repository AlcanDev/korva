// Package main — JWS RS256 signing for the Korva licensing server.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// loadPrivateKey reads an RSA private key from the environment or a file.
//
// Lookup order:
//  1. KORVA_LICENSING_PRIVATE_KEY_PEM  — PEM content as env var
//  2. KORVA_LICENSING_PRIVATE_KEY_FILE — path to a PEM file
//
// Both PKCS#8 and PKCS#1 formats are supported.
func loadPrivateKey() (*rsa.PrivateKey, error) {
	if pem := strings.TrimSpace(os.Getenv("KORVA_LICENSING_PRIVATE_KEY_PEM")); pem != "" {
		return parseRSAPrivKey([]byte(pem))
	}
	if path := strings.TrimSpace(os.Getenv("KORVA_LICENSING_PRIVATE_KEY_FILE")); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		return parseRSAPrivKey(raw)
	}
	return nil, fmt.Errorf(
		"no private key configured — set KORVA_LICENSING_PRIVATE_KEY_PEM or KORVA_LICENSING_PRIVATE_KEY_FILE")
}

// signJWS creates a compact JWS (RS256) from a license payload map.
// kid identifies which public key in the client binary should be used to verify.
func signJWS(privKey *rsa.PrivateKey, kid string, payload map[string]any) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{"alg": "RS256", "kid": kid})
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	h64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	p64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := h64 + "." + p64

	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return strings.Join([]string{h64, p64, base64.RawURLEncoding.EncodeToString(sig)}, "."), nil
}

func parseRSAPrivKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("pem: no key block found")
	}
	// Try PKCS#8 first (openssl genpkey / go generate output).
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("pem: parsed key is not RSA")
	}
	// Fall back to PKCS#1 (older openssl genrsa output).
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
