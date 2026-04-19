package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// ─── Test setup ──────────────────────────────────────────────────────────────

const testSecret = "test-admin-secret-abc"

func newTestServer(t *testing.T) *server {
	t.Helper()
	db, err := openDB(":memory:")
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// 2048-bit key for test speed (production uses 4096).
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	return &server{
		db:      db,
		privKey: privKey,
		kid:     "korva-license-v1",
		secret:  testSecret,
	}
}

// do is a thin helper that dispatches a POST directly to a handler.
func do(t *testing.T, srv *server, path string, body any, extraHeaders map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	switch path {
	case "/v1/issue":
		srv.handleIssue(w, req)
	case "/v1/activate":
		srv.handleActivate(w, req)
	case "/v1/heartbeat":
		srv.handleHeartbeat(w, req)
	case "/v1/deactivate":
		srv.handleDeactivate(w, req)
	default:
		t.Fatalf("unknown path: %s", path)
	}
	return w
}

func authHeader() map[string]string {
	return map[string]string{"Authorization": "Bearer " + testSecret}
}

// issueLicense creates a license via the admin endpoint and returns its key + id.
func issueLicense(t *testing.T, srv *server, email string, seats int) (key, licID string) {
	t.Helper()
	w := do(t, srv, "/v1/issue", map[string]any{
		"customer_email": email, "seats": seats, "expire_days": 365,
	}, authHeader())
	if w.Code != http.StatusCreated {
		t.Fatalf("issueLicense: want 201, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	return resp["license_key"].(string), resp["license_id"].(string)
}

// ─── Health ──────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// ─── Issue ───────────────────────────────────────────────────────────────────

func TestIssue_Success(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/issue", map[string]any{
		"customer_email": "alice@corp.com",
		"seats":          5,
		"expire_days":    365,
	}, authHeader())

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp["license_key"] == "" {
		t.Error("response must contain license_key")
	}
	key, _ := resp["license_key"].(string)
	// Format: KORVA-XXXX-XXXX-XXXX-XXXX  →  6 + (4+1)*4 = 25 chars total
	if len(key) != 25 || key[:6] != "KORVA-" {
		t.Errorf("unexpected key format: %q (len=%d)", key, len(key))
	}
}

func TestIssue_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/issue", map[string]any{"customer_email": "x@y.com"},
		map[string]string{"Authorization": "Bearer wrong"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestIssue_MissingEmail(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/issue", map[string]any{"seats": 5}, authHeader())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestIssue_DefaultsApplied(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/issue", map[string]any{"customer_email": "b@c.com"}, authHeader())
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if int(resp["seats"].(float64)) != 5 {
		t.Errorf("default seats should be 5, got %v", resp["seats"])
	}
	feats, _ := resp["features"].([]any)
	if len(feats) == 0 {
		t.Error("default features should not be empty")
	}
}

// ─── Activate ────────────────────────────────────────────────────────────────

func TestActivate_Success(t *testing.T) {
	srv := newTestServer(t)
	key, _ := issueLicense(t, srv, "alice@corp.com", 3)

	w := do(t, srv, "/v1/activate", map[string]string{
		"license_key": key,
		"install_id":  "install-001",
	}, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp["jws"] == "" {
		t.Error("response must contain jws")
	}
	// JWS compact format: three base64url segments.
	parts := bytes.Split([]byte(resp["jws"]), []byte{'.'})
	if len(parts) != 3 {
		t.Errorf("JWS must have 3 segments, got %d", len(parts))
	}
}

func TestActivate_UnknownKey(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/activate", map[string]string{
		"license_key": "KORVA-ZZZZ-ZZZZ-ZZZZ-ZZZZ",
		"install_id":  "x",
	}, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestActivate_MissingInstallID(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/activate",
		map[string]string{"license_key": "KORVA-ABCD-EFGH-IJKL-MNOP"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestActivate_SeatLimit(t *testing.T) {
	srv := newTestServer(t)
	key, _ := issueLicense(t, srv, "bob@corp.com", 1) // only 1 seat

	w1 := do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "seat-A"}, nil)
	if w1.Code != http.StatusOK {
		t.Fatalf("first activation: want 200, got %d", w1.Code)
	}

	// Different install_id on the same 1-seat license should fail.
	w2 := do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "seat-B"}, nil)
	if w2.Code != http.StatusPaymentRequired {
		t.Fatalf("over-seat: want 402, got %d — %s", w2.Code, w2.Body.String())
	}
}

func TestActivate_RenewalAlwaysAllowed(t *testing.T) {
	srv := newTestServer(t)
	key, _ := issueLicense(t, srv, "carol@corp.com", 1)

	// Same install_id can renew indefinitely regardless of seat count.
	for i := 0; i < 3; i++ {
		w := do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "my-laptop"}, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("renewal %d: want 200, got %d", i+1, w.Code)
		}
	}
}

// ─── Heartbeat ───────────────────────────────────────────────────────────────

func TestHeartbeat_Success(t *testing.T) {
	srv := newTestServer(t)
	key, licID := issueLicense(t, srv, "dave@corp.com", 2)

	// Must activate first.
	do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "hb-machine"}, nil)

	w := do(t, srv, "/v1/heartbeat", map[string]string{
		"license_id": licID,
		"install_id": "hb-machine",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp["jws"] == "" {
		t.Error("heartbeat must return jws")
	}
	if resp["ok"] != true {
		t.Error("heartbeat must return ok=true")
	}
}

func TestHeartbeat_UnknownLicense(t *testing.T) {
	srv := newTestServer(t)
	w := do(t, srv, "/v1/heartbeat", map[string]string{
		"license_id": "lic_nonexistent",
		"install_id": "x",
	}, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

// ─── Deactivate ──────────────────────────────────────────────────────────────

func TestDeactivate_FreesASeat(t *testing.T) {
	srv := newTestServer(t)
	key, licID := issueLicense(t, srv, "eve@corp.com", 1)

	// Occupy the single seat.
	do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "old-machine"}, nil)

	// Deactivate.
	wd := do(t, srv, "/v1/deactivate", map[string]string{
		"license_id": licID, "install_id": "old-machine",
	}, nil)
	if wd.Code != http.StatusOK {
		t.Fatalf("deactivate: want 200, got %d", wd.Code)
	}

	// Seat is now free — new machine should activate.
	w2 := do(t, srv, "/v1/activate", map[string]string{"license_key": key, "install_id": "new-machine"}, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("post-deactivate activation: want 200, got %d", w2.Code)
	}
}

// ─── Key generation ──────────────────────────────────────────────────────────

func TestGenerateLicenseKey_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		k := generateLicenseKey()
		if len(k) != 25 {
			t.Errorf("[%d] want len=25, got %d (%q)", i, len(k), k)
		}
		if k[:6] != "KORVA-" {
			t.Errorf("[%d] must start with KORVA-, got %q", i, k)
		}
		if seen[k] {
			t.Errorf("duplicate key generated: %q", k)
		}
		seen[k] = true
	}
}

// ─── loadPrivateKey ──────────────────────────────────────────────────────────

func TestLoadPrivateKey_FromFile(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(privKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	f, _ := os.CreateTemp(t.TempDir(), "priv-*.pem")
	f.Write(pemBytes)
	f.Close()

	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_PEM", "")
	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_FILE", f.Name())

	key, err := loadPrivateKey()
	if err != nil {
		t.Fatalf("loadPrivateKey from file: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestLoadPrivateKey_FromEnvVar(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(privKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_PEM", string(pemBytes))
	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_FILE", "")

	key, err := loadPrivateKey()
	if err != nil {
		t.Fatalf("loadPrivateKey from env: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestLoadPrivateKey_MissingConfig(t *testing.T) {
	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_PEM", "")
	t.Setenv("KORVA_LICENSING_PRIVATE_KEY_FILE", "")
	_, err := loadPrivateKey()
	if err == nil {
		t.Fatal("expected error when no key configured")
	}
}
