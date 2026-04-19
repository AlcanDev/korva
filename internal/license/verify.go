package license

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

//go:embed keys/pubkey-dev.pem
var devPubKeyPEM []byte

// prodPubKeyPEM is the production RSA public key (kid: korva-license-v1).
// BEFORE GO-LIVE: replace keys/pubkey.pem with a freshly generated key using
//
//	go run github.com/alcandev/korva/forge/licensing-server/keygen
//
// The matching private key must NEVER be committed to the repository.
//
//go:embed keys/pubkey.pem
var prodPubKeyPEM []byte

// pubkeys maps a JWS `kid` (key id) to its parsed RSA public key.
// Multiple keys are supported so server-side key rotation does not require
// shipping a new client binary — add the new key here and keep the old one
// until all active licenses have been re-issued.
var pubkeys = map[string]*rsa.PublicKey{}

func init() {
	if k, err := parseRSAPubKey(devPubKeyPEM); err == nil {
		pubkeys["korva-license-dev"] = k
	}
	if k, err := parseRSAPubKey(prodPubKeyPEM); err == nil {
		pubkeys["korva-license-v1"] = k
	}
}

func parseRSAPubKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("pem: no key block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("pem: not an RSA public key")
	}
	return rsaPub, nil
}

type jwsHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// verifyJWS parses a compact JWS string ("h.p.s"), verifies the RS256
// signature against an embedded public key matching `kid`, and returns
// the decoded License payload.
func verifyJWS(token string) (*License, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("license: malformed JWS (expected 3 segments, got %d)", len(parts))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("license: header b64: %w", err)
	}
	var h jwsHeader
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, fmt.Errorf("license: header json: %w", err)
	}
	if h.Alg != "RS256" {
		return nil, fmt.Errorf("license: unsupported alg %q (only RS256)", h.Alg)
	}
	pub, ok := pubkeys[h.Kid]
	if !ok {
		return nil, fmt.Errorf("license: unknown kid %q (binary may be too old)", h.Kid)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("license: sig b64: %w", err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	hash := sha256.Sum256(signingInput)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil {
		return nil, fmt.Errorf("license: signature: %w", err)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("license: payload b64: %w", err)
	}
	var lic License
	if err := json.Unmarshal(payloadJSON, &lic); err != nil {
		return nil, fmt.Errorf("license: payload json: %w", err)
	}
	return &lic, nil
}
