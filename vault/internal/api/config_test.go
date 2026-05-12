package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminGetConfig_Local(t *testing.T) {
	s := newAPITestStore(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, path, map[string]any{
		"version": "1",
		"project": "korva",
		"agent":   "claude",
		"vault":   map[string]any{"port": 7437, "auto_start": true},
	})

	c := &configEndpoint{store: s, pathLocal: path}
	h := adminGetConfig(c)

	req := httptest.NewRequest(http.MethodGet, "/admin/config", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Scope  string             `json:"scope"`
		Path   string             `json:"path"`
		Hash   string             `json:"hash"`
		Config config.KorvaConfig `json:"config"`
		Exists bool               `json:"exists"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if resp.Scope != "local" {
		t.Errorf("scope = %q, want local", resp.Scope)
	}
	if resp.Path != path {
		t.Errorf("path = %q, want %q", resp.Path, path)
	}
	if resp.Hash == "" {
		t.Error("hash should not be empty")
	}
	if resp.Config.Project != "korva" {
		t.Errorf("project = %q, want korva", resp.Config.Project)
	}
	if !resp.Exists {
		t.Error("exists should be true")
	}
}

func TestAdminGetConfig_UnknownScope(t *testing.T) {
	s := newAPITestStore(t)
	c := &configEndpoint{store: s, pathLocal: "/tmp/x.json"}
	h := adminGetConfig(c)

	req := httptest.NewRequest(http.MethodGet, "/admin/config?scope=alien", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminPutConfig_HappyPath(t *testing.T) {
	s := newAPITestStore(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, path, map[string]any{
		"version": "1",
		"agent":   "claude",
		"country": "CL",
		"vault":   map[string]any{"port": 7437, "auto_start": true},
	})

	c := &configEndpoint{store: s, pathLocal: path}
	h := adminPutConfig(c)

	body := putConfigRequest{
		Scope: "local",
		Config: config.KorvaConfig{
			Version: "1",
			Agent:   "claude",
			Country: "CL",
			Vault:   config.VaultConfig{Port: 7437, AutoStart: false},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "saved" {
		t.Errorf("status = %v, want saved", resp["status"])
	}
	if resp["snapshot_id"] == "" || resp["snapshot_id"] == nil {
		t.Errorf("expected snapshot_id, got %v", resp["snapshot_id"])
	}

	// Disk content updated to new auto_start=false.
	got, _ := config.Load(path)
	if got.Vault.AutoStart {
		t.Errorf("expected auto_start=false on disk, got %+v", got.Vault)
	}

	// Snapshot row created.
	snaps, _ := s.ListConfigSnapshots("local", 10)
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
	// BeforeJSON keeps whatever formatting was on disk before the PUT — match
	// loosely to allow both indented and compact serializations.
	if !strings.Contains(snaps[0].BeforeJSON, `"auto_start":true`) &&
		!strings.Contains(snaps[0].BeforeJSON, `"auto_start": true`) {
		t.Errorf("snapshot BeforeJSON missing previous state: %q", snaps[0].BeforeJSON)
	}
}

func TestAdminPutConfig_ValidationError(t *testing.T) {
	s := newAPITestStore(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")
	c := &configEndpoint{store: s, pathLocal: path}
	h := adminPutConfig(c)

	body := putConfigRequest{
		Scope: "local",
		Config: config.KorvaConfig{
			Vault: config.VaultConfig{Port: 80}, // out of range
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["field"] != "vault.port" {
		t.Errorf("field = %v, want vault.port", resp["field"])
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not exist after validation failure")
	}
}

func TestAdminPutConfig_HashConflict(t *testing.T) {
	s := newAPITestStore(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	// Initial write so the file exists with a known hash.
	if _, err := config.WriteAtomic(path, config.KorvaConfig{
		Agent: "claude", Country: "CL", Vault: config.VaultConfig{Port: 7437},
	}, config.WriteOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := &configEndpoint{store: s, pathLocal: path}
	h := adminPutConfig(c)

	body := putConfigRequest{
		Scope:        "local",
		ExpectedHash: "deadbeefdeadbeefdeadbeefdeadbeef",
		Config: config.KorvaConfig{
			Agent: "claude", Country: "CL", Vault: config.VaultConfig{Port: 7437},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestAdminPutConfig_RestartRequired(t *testing.T) {
	s := newAPITestStore(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "korva.config.json")

	// Seed initial config.
	if _, err := config.WriteAtomic(path, config.KorvaConfig{
		Agent: "claude", Country: "CL", Vault: config.VaultConfig{Port: 7437},
	}, config.WriteOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := &configEndpoint{store: s, pathLocal: path}
	h := adminPutConfig(c)

	body := putConfigRequest{
		Scope: "local",
		Config: config.KorvaConfig{
			Agent: "claude", Country: "CL",
			Vault: config.VaultConfig{Port: 8080}, // restart-sensitive
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	restartList, _ := resp["restart_required"].([]any)
	if len(restartList) == 0 {
		t.Errorf("restart_required should not be empty, got %v", resp["restart_required"])
	}
}

func TestAdminListConfigSnapshots(t *testing.T) {
	s := newAPITestStore(t)
	for i := 0; i < 2; i++ {
		if _, err := s.SaveConfigSnapshot(store.ConfigSnapshot{
			Actor:      "admin",
			Scope:      "local",
			FilePath:   "/tmp/x.json",
			BeforeJSON: `{"v":0}`,
			AfterJSON:  `{"v":1}`,
		}); err != nil {
			t.Fatalf("seed Save() error = %v", err)
		}
	}

	h := adminListConfigSnapshots(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/config/snapshots?scope=local", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Count != 2 {
		t.Errorf("count = %d, want 2", resp.Count)
	}
}
