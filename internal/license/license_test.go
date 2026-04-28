package license

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testPrivKeyPEM is the dev-only RSA-2048 private key used to sign JWS tokens
// in tests. It matches the public key embedded in keys/pubkey-dev.pem.
//
// This is a TEST-ONLY key. It was generated with:
//
//	openssl genrsa 2048 | openssl pkcs8 -topk8 -nocrypt
//
// The corresponding public key lives in internal/license/keys/pubkey-dev.pem.
// Neither key has any production value — the production private key is never
// stored in any repository.
var testPrivKeyPEM = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDilk1dTzBk2vmS
0vJCHDKrT1FIRXW5HZV2ZR6ZMIyZvtvDNFZazr8lrim04L1fPclgOlEONTSrfgjf
pXvhID/ODzyRo9nkIpxcNkECkIMKNrOclAVliVYSybrIkiPRCw9oV4RAnOleD+Pg
sh9UrxFbqAd/QERkUbvnfTEfJYg+EH6+I7Y4LYTw4ra1tQSXKbuSyUyKjW49pazI
rvyK/3CSJnSx0IZ18CguZMjg/62Rdou6KNldlTPHOk6Ohyr8AA8LBEBxt6zgLsJ/
WrLkdKRIpzRi+0nVb+5AWkipEzvFikH/IVfZWeFQGZlKkq694Ly30CW+uPW+t2LA
h2/pftZJAgMBAAECggEAUtV8oeR9vhkZTPYmD0oMsfjUr7WI5GwuxDISXhFUDS6r
W3DqMtdLJMTHRXM1d7h4Ql//WtDTmPAB4XS3VWU7PiLu0xVR5idK/yDsYjofVaAK
yG6KjISI/WRXDtTyGA1RjCUWWaKjY7ouZenoL0ay8015tCjz97KznVx0lTzc0kb4
FznOhqqxZyNa4Ad8Q/LjPSbBiTdXjYWkPctU2Z1kW4vKEemssju+92yygtyYgV8N
ZYSeZKfSrvBe/E3Pj0f6yfMuWm7GIW/i1HFCmegBRSfrF0bhHvYa1vVx4uscU0a4
36ovXx5fPwT5pg/ucM0H33r0+caxEXSCC60vQ6lWmwKBgQD+wFIdOY++pxCYmJvj
fOvhCQIgIrId8QxefbiifmaMOtunZT2CnaltFct1SwzOhcFknGM68xSRhbLLiwgv
8aZHyrQxMOyAxT7Ny0d79tF6AotlZBLNQ+u+7Ut61IxkkPpbR5YHgBlB4Vi+xBkp
9P+AC5YN1a+B378sLmNBHBUWkwKBgQDjsqOgoSEwvavzPaLLJYYDhqcVfz4QGVcT
lYBHhZEFy/0Iw98MoQYnLglzpVhJDwkSME4fWciDta598nB6HM2zYKrsPJZyHgvp
I3o2qio2eB+vRPtMg4F2V7KONPtGJ9zApaKg4nlJnFgvfODp16ApobhLzNpi3+t0
gVz69w6tMwKBgAOOhdb4ncQoqvemcc68SMLMkGYIdforCmQrVy+VmjLtA3IT3Mb9
Eod+XWfW02fywB96e3wwNqJNfpCO8V9R/WNVNizVpQerOVRAOVBGwuf0LyQMQKLz
BtCUmZAudYNV7tjlZ/fU1wVvcwC+1icaz5JnFwI8cIXcrNueDi6ziKvXAoGBAN2C
kARYPH26R2le8NxIKNONT0ZufuYSgM+ghScPHUJSbFr2kisrC11aP/+tPvH0GpMD
Qzzkj1jyikokbJ+fHc3/oMgpOQLTkCrCRMahTGeo/Mn5ha+tz2hdcGs/x6M8bFlN
yaRSLkQaQQARsIxNJJbbqPq000+VHu48W0QazMBZAoGBAPT9DwTDjoX/q1YI7SeK
oeAhJEwSlheO7zWukx9hjz8wYdj6RfMXo/JRELdhXHnduilURGq2/Xr6D/3gIWff
lES9Q7+YBNYOCsAJOzEzt6MEoNNVaHozaFa98OS31Twpj/Zf/CWWHSshiDOK02xU
axTJs4z9p5IatBWXh4Z6+3K/
-----END PRIVATE KEY-----
`)

func mustParsePrivKey(pemBytes []byte) *rsa.PrivateKey {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		panic("license_test: pem decode failed")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		k, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err2 != nil {
			panic("license_test: parse private key: " + err.Error())
		}
		return k
	}
	return key.(*rsa.PrivateKey)
}

func signTestJWS(privKey *rsa.PrivateKey, kid string, payload map[string]interface{}) string {
	headerBytes, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid})
	payloadBytes, _ := json.Marshal(payload)
	h64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	p64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sigInput := h64 + "." + p64
	hash := sha256.Sum256([]byte(sigInput))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	return strings.Join([]string{h64, p64, base64.RawURLEncoding.EncodeToString(sig)}, ".")
}

func validPayload(overrides map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{
		"license_id": "lic-test-001",
		"sub":        "team-test",
		"tier":       "teams",
		"features":   []string{"admin_skills", "audit_log"},
		"iat":        time.Now().UTC(),
		"exp":        time.Now().Add(365 * 24 * time.Hour).UTC(),
		"grace_days": 7,
		"seats":      5,
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

func TestVerifyJWS_Valid(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))

	lic, err := verifyJWS(jws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lic.LicenseID != "lic-test-001" {
		t.Errorf("license_id mismatch: %s", lic.LicenseID)
	}
	if lic.Tier != TierTeams {
		t.Errorf("tier mismatch: %s", lic.Tier)
	}
}

func TestVerifyJWS_TamperedSignature(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))
	// flip last byte of signature
	parts := strings.Split(jws, ".")
	sig := []byte(parts[2])
	sig[len(sig)-1] ^= 0xFF
	tampered := strings.Join([]string{parts[0], parts[1], string(sig)}, ".")

	if _, err := verifyJWS(tampered); err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestVerifyJWS_UnknownKid(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-future-v2", validPayload(nil))

	if _, err := verifyJWS(jws); err == nil {
		t.Fatal("expected unknown kid error, got nil")
	}
}

func TestVerifyJWS_MalformedSegments(t *testing.T) {
	cases := []string{"", "a.b", "a.b.c.d", "not-base64!.b.c"}
	for _, tc := range cases {
		if _, err := verifyJWS(tc); err == nil {
			t.Errorf("expected error for %q", tc)
		}
	}
}

func TestValidate_Expired(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	payload := validPayload(map[string]interface{}{
		"exp": time.Now().Add(-24 * time.Hour).UTC(),
	})
	jws := signTestJWS(priv, "korva-license-dev", payload)

	lic, err := verifyJWS(jws)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := lic.Validate(); err == nil {
		t.Fatal("expected expired error, got nil")
	}
}

func TestValidate_UnknownTier(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	payload := validPayload(map[string]interface{}{"tier": "enterprise-plus"})
	jws := signTestJWS(priv, "korva-license-dev", payload)

	lic, _ := verifyJWS(jws)
	if err := lic.Validate(); err == nil {
		t.Fatal("expected unknown tier error, got nil")
	}
}

func TestHasFeature(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))
	lic, _ := verifyJWS(jws)

	if !lic.HasFeature(FeatureAdminSkills) {
		t.Error("expected HasFeature(admin_skills)=true")
	}
	if lic.HasFeature(FeaturePrivateScrolls) {
		t.Error("expected HasFeature(private_scrolls)=false")
	}
	if ((*License)(nil)).HasFeature(FeatureAdminSkills) {
		t.Error("nil license should return false")
	}
}

func TestCurrentTier_GraceLapsed(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))
	lic, _ := verifyJWS(jws)

	state := &State{
		LastHeartbeat: time.Now().Add(-10 * 24 * time.Hour), // 10 days ago; grace=7
		LicenseID:     "lic-test-001",
	}
	if tier := lic.CurrentTier(state); tier != TierCommunity {
		t.Errorf("expected community after grace lapse, got %s", tier)
	}
}

func TestCurrentTier_WithinGrace(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))
	lic, _ := verifyJWS(jws)

	state := &State{
		LastHeartbeat: time.Now().Add(-3 * 24 * time.Hour), // 3 days ago; grace=7
		LicenseID:     "lic-test-001",
	}
	if tier := lic.CurrentTier(state); tier != TierTeams {
		t.Errorf("expected teams within grace, got %s", tier)
	}
}

func TestLoadState_Missing(t *testing.T) {
	s, err := LoadState(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.LastHeartbeat.IsZero() {
		t.Error("expected zero heartbeat for missing file")
	}
}

func TestSaveLoadState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	now := time.Now().UTC().Truncate(time.Second)
	in := &State{LastHeartbeat: now, LicenseID: "lic-abc"}

	if err := SaveState(path, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !out.LastHeartbeat.Equal(in.LastHeartbeat) {
		t.Errorf("heartbeat mismatch: got %v want %v", out.LastHeartbeat, in.LastHeartbeat)
	}
	if out.LicenseID != in.LicenseID {
		t.Errorf("license_id mismatch: %s", out.LicenseID)
	}
}

// --- Load ---

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "no-license.key"))
	if !errors.Is(err, ErrMissing) {
		t.Errorf("expected ErrMissing, got %v", err)
	}
}

func TestLoad_ValidJWS(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))

	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	if err := os.WriteFile(path, []byte(jws), 0600); err != nil {
		t.Fatal(err)
	}
	lic, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lic.LicenseID != "lic-test-001" {
		t.Errorf("license_id = %q", lic.LicenseID)
	}
}

func TestLoad_BadJWS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	if err := os.WriteFile(path, []byte("not-a-jws"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for bad JWS, got nil")
	}
}

// --- Activate ---

func TestActivate_Success(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/activate" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"jws": jws})
	}))
	defer srv.Close()

	dir := t.TempDir()
	licensePath := filepath.Join(dir, "license.key")
	statePath := filepath.Join(dir, "license.state.json")

	lic, err := Activate(context.Background(), srv.URL+"/v1/activate", "lic-key-dev", "install-xyz", licensePath, statePath)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if lic.LicenseID != "lic-test-001" {
		t.Errorf("license_id = %q", lic.LicenseID)
	}
	// Files must be written
	if _, err := os.Stat(licensePath); err != nil {
		t.Error("license file not written")
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Error("state file not written")
	}
}

func TestActivate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := Activate(context.Background(), srv.URL+"/v1/activate", "k", "i",
		filepath.Join(dir, "l.key"), filepath.Join(dir, "s.json"))
	if err == nil {
		t.Fatal("expected error for server 500, got nil")
	}
}

func TestActivate_BadJWSResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"jws": "bad.token.here"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := Activate(context.Background(), srv.URL+"/v1/activate", "k", "i",
		filepath.Join(dir, "l.key"), filepath.Join(dir, "s.json"))
	if err == nil {
		t.Fatal("expected error for bad JWS, got nil")
	}
}

// --- Deactivate ---

func TestDeactivate_RemovesBothFiles(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "license.key")
	sp := filepath.Join(dir, "state.json")
	os.WriteFile(lp, []byte("jws"), 0600)
	os.WriteFile(sp, []byte("{}"), 0600)

	if err := Deactivate(lp, sp); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Error("license file not removed")
	}
	if _, err := os.Stat(sp); !os.IsNotExist(err) {
		t.Error("state file not removed")
	}
}

func TestDeactivate_MissingFilesOK(t *testing.T) {
	dir := t.TempDir()
	if err := Deactivate(filepath.Join(dir, "x"), filepath.Join(dir, "y")); err != nil {
		t.Fatalf("Deactivate with missing files: %v", err)
	}
}

// --- sendHeartbeat ---

func TestSendHeartbeat_Success(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	jws := signTestJWS(priv, "korva-license-dev", validPayload(nil))
	lic, _ := verifyJWS(jws)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	if err := sendHeartbeat(context.Background(), srv.URL+"/v1/heartbeat", "install-abc", statePath, lic); err != nil {
		t.Fatalf("sendHeartbeat: %v", err)
	}
	state, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.LastHeartbeat.IsZero() {
		t.Error("expected heartbeat timestamp to be set")
	}
}

func TestSendHeartbeat_ServerError(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	lic, _ := verifyJWS(signTestJWS(priv, "korva-license-dev", validPayload(nil)))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	dir := t.TempDir()
	err := sendHeartbeat(context.Background(), srv.URL+"/v1/heartbeat", "id", filepath.Join(dir, "s.json"), lic)
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

// --- RunHeartbeat ---

func TestRunHeartbeat_NilLicNoOp(t *testing.T) {
	// Should return without panic when lic is nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	RunHeartbeat(ctx, "http://unused", "id", filepath.Join(t.TempDir(), "s.json"), nil)
}

func TestRunHeartbeat_CancelStops(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	lic, _ := verifyJWS(signTestJWS(priv, "korva-license-dev", validPayload(nil)))

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	RunHeartbeat(ctx, srv.URL+"/v1/heartbeat", "id", filepath.Join(dir, "s.json"), lic)
	// Give the immediate fire time to complete
	time.Sleep(50 * time.Millisecond)
	cancel()
	// At least the immediate fire should have happened
	if calls == 0 {
		t.Error("expected at least 1 heartbeat call")
	}
}

// --- parseRSAPubKey ---

func TestParseRSAPubKey_NilPEM(t *testing.T) {
	_, err := parseRSAPubKey([]byte("not-pem"))
	if err == nil {
		t.Fatal("expected error for non-PEM input")
	}
}

// --- GraceRemaining edge cases ---

func TestGraceRemaining_NilLicense(t *testing.T) {
	if d := ((*License)(nil)).GraceRemaining(&State{}); d != 0 {
		t.Errorf("nil license GraceRemaining = %v, want 0", d)
	}
}

func TestGraceRemaining_NilState(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	lic, _ := verifyJWS(signTestJWS(priv, "korva-license-dev", validPayload(nil)))
	if d := lic.GraceRemaining(nil); d != 0 {
		t.Errorf("nil state GraceRemaining = %v, want 0", d)
	}
}

func TestGraceRemaining_Positive(t *testing.T) {
	priv := mustParsePrivKey(testPrivKeyPEM)
	lic, _ := verifyJWS(signTestJWS(priv, "korva-license-dev", validPayload(nil)))
	state := &State{LastHeartbeat: time.Now().Add(-2 * 24 * time.Hour)}
	rem := lic.GraceRemaining(state)
	if rem <= 0 {
		t.Errorf("expected positive grace remaining, got %v", rem)
	}
}

// --- CurrentTier nil lic ---

func TestCurrentTier_NilLicense(t *testing.T) {
	if tier := ((*License)(nil)).CurrentTier(nil); tier != TierCommunity {
		t.Errorf("nil license tier = %s, want community", tier)
	}
}
