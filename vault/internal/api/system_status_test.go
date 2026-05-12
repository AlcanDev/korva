package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminSystemStatus_BasicShape(t *testing.T) {
	s := newAPITestStore(t)
	// Seed a few rows so counts are non-zero.
	for i := 0; i < 3; i++ {
		_, _ = s.SaveInteraction(store.Interaction{Project: "korva", Agent: "claude"})
	}

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, configPath, map[string]any{
		"version":  "1",
		"project":  "korva",
		"sentinel": map[string]any{"enabled": true, "hooks": []string{"pre-commit"}},
		"lore":     map[string]any{"active_scrolls": []string{"forge-sdd"}},
	})

	h := adminSystemStatus(systemStatusInputs{
		Store:           s,
		StartedAt:       time.Now().Add(-2 * time.Minute),
		Version:         "0.7.2",
		Port:            7437,
		ConfigPathLocal: configPath,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/system-status", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response error = %v", err)
	}

	for _, key := range []string{"ide", "vault", "hive", "sentinel", "lore", "skills", "license",
		"sessions", "observations", "prompts"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing top-level key %q in response", key)
		}
	}

	vault, ok := resp["vault"].(map[string]any)
	if !ok {
		t.Fatalf("vault field is not a map: %T", resp["vault"])
	}
	if vault["running"] != true {
		t.Errorf("vault.running = %v, want true", vault["running"])
	}
	if vault["version"] != "0.7.2" {
		t.Errorf("vault.version = %v, want 0.7.2", vault["version"])
	}
	if uptime, _ := vault["uptime_sec"].(float64); uptime <= 0 {
		t.Errorf("vault.uptime_sec = %v, want > 0", uptime)
	}

	sentinel, _ := resp["sentinel"].(map[string]any)
	if sentinel["enabled"] != true {
		t.Errorf("sentinel.enabled = %v, want true", sentinel["enabled"])
	}

	lore, _ := resp["lore"].(map[string]any)
	scrolls, _ := lore["active_scrolls"].([]any)
	if len(scrolls) != 1 || scrolls[0] != "forge-sdd" {
		t.Errorf("lore.active_scrolls = %v, want [forge-sdd]", scrolls)
	}

	license, _ := resp["license"].(map[string]any)
	if license["tier"] != "community" {
		t.Errorf("license.tier = %v, want community", license["tier"])
	}
}

func TestAdminSystemStatus_NoConfig(t *testing.T) {
	s := newAPITestStore(t)
	h := adminSystemStatus(systemStatusInputs{
		Store:           s,
		ConfigPathLocal: "", // no config file
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/system-status", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	// Just assert it doesn't crash and returns the standard shape.
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := resp["sentinel"]; !ok {
		t.Errorf("sentinel section should exist even without config")
	}
}

func TestAdminSystemStatus_WithLicense(t *testing.T) {
	s := newAPITestStore(t)
	lic := &license.License{
		LicenseID: "lic-1",
		TeamID:    "team-1",
		Tier:      license.TierTeams,
		Features:  []string{},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Seats:     5,
	}

	h := adminSystemStatus(systemStatusInputs{
		Store:   s,
		License: lic,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/system-status", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	licResp, _ := resp["license"].(map[string]any)
	if licResp["tier"] != "teams" {
		t.Errorf("license.tier = %v, want teams", licResp["tier"])
	}
	if seats, _ := licResp["seats_total"].(float64); seats != 5 {
		t.Errorf("license.seats_total = %v, want 5", seats)
	}
}

func TestSentinelBuiltinCount_Profiles(t *testing.T) {
	tests := []struct {
		profile string
		want    int
	}{
		{"minimal", 1},
		{"standard", 4},
		{"strict", 10},
		{"custom", 0},
		{"unknown", 4}, // falls back to standard
	}
	for _, tc := range tests {
		t.Run(tc.profile, func(t *testing.T) {
			if got := sentinelBuiltinCount(tc.profile); got != tc.want {
				t.Errorf("sentinelBuiltinCount(%q) = %d, want %d", tc.profile, got, tc.want)
			}
		})
	}
}

func TestCountCustomRules(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "rules.yaml")
	yaml := `version: 1
profile: custom
rules:
  - id: CUSTOM-001
    description: "rule 1"
  - id: CUSTOM-002
    description: "rule 2"
  - id: CUSTOM-003
    description: "rule 3"
`
	if err := os.WriteFile(yamlPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if got := countCustomRules(yamlPath, ""); got != 3 {
		t.Errorf("countCustomRules = %d, want 3", got)
	}
	if got := countCustomRules("", ""); got != 0 {
		t.Errorf("empty path should return 0, got %d", got)
	}
	if got := countCustomRules("/nonexistent", ""); got != 0 {
		t.Errorf("missing file should return 0, got %d", got)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func mustWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
