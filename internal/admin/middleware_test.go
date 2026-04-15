package admin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMiddleware_MissingHeader(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")
	Generate(keyPath, "owner", false)

	handler := Middleware(keyPath)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", rec.Code)
	}
}

func TestMiddleware_CorrectKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")
	cfg, _ := Generate(keyPath, "owner", false)

	reached := false
	handler := Middleware(keyPath)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set(headerName, cfg.Key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with correct key, got %d", rec.Code)
	}
	if !reached {
		t.Error("next handler was not reached with correct key")
	}
}

func TestMiddleware_WrongKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")
	Generate(keyPath, "owner", false)

	handler := Middleware(keyPath)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set(headerName, "completely-wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 with wrong key, got %d", rec.Code)
	}
}

func TestMiddleware_NoKeyFile(t *testing.T) {
	handler := Middleware("/nonexistent/path/admin.key")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set(headerName, "any-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No admin.key on this machine → 403 Forbidden
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when no key file, got %d", rec.Code)
	}
}

func TestMiddleware_RotatedKey_OldKeyRejected(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	original, _ := Generate(keyPath, "owner", false)
	Rotate(keyPath, original.Key)

	handler := Middleware(keyPath)(okHandler())

	// Old key should now fail
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set(headerName, original.Key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for old key after rotation, got %d", rec.Code)
	}
}

func TestMiddleware_RotatedKey_NewKeyAccepted(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	original, _ := Generate(keyPath, "owner", false)
	rotated, _ := Rotate(keyPath, original.Key)

	handler := Middleware(keyPath)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set(headerName, rotated.Key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for new key after rotation, got %d", rec.Code)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "bad.key")
	os.WriteFile(keyPath, []byte("not-valid-json"), 0600)

	_, err := Load(keyPath)
	if err == nil {
		t.Error("expected error loading invalid JSON key file")
	}
}

func TestRotate_PreservesOwner(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	original, _ := Generate(keyPath, "felipe@alcandev", false)
	rotated, err := Rotate(keyPath, original.Key)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rotated.Owner != "felipe@alcandev" {
		t.Errorf("Rotate should preserve owner, got %q", rotated.Owner)
	}
}

func TestRotate_VersionIncrementsMultiple(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "admin.key")

	cfg, _ := Generate(keyPath, "owner", false)
	r1, _ := Rotate(keyPath, cfg.Key)
	r2, err := Rotate(keyPath, r1.Key)
	if err != nil {
		t.Fatalf("second Rotate: %v", err)
	}
	if r2.Version != 3 {
		t.Errorf("expected version 3 after two rotations, got %d", r2.Version)
	}
}

func TestRotate_NoKeyFile(t *testing.T) {
	_, err := Rotate("/nonexistent/admin.key", "any-key")
	if err != ErrNoAdminKey {
		t.Errorf("expected ErrNoAdminKey, got %v", err)
	}
}

// okHandler returns an HTTP handler that always responds 200.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
