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
)

func TestAdminGetSentinelRules_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, configPath, map[string]any{
		"version":  "1",
		"sentinel": map[string]any{"rules_path": ".korva/sentinel-rules.yaml"},
	})

	h := adminGetSentinelRules(configPath)
	req := httptest.NewRequest(http.MethodGet, "/admin/sentinel/rules", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Profile   string         `json:"profile"`
		RulesPath string         `json:"rules_path"`
		Builtin   []map[string]any `json:"builtin"`
		Custom    []sentinelRule `json:"custom"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.HasSuffix(resp.RulesPath, "sentinel-rules.yaml") {
		t.Errorf("rules_path = %q, expected ending in sentinel-rules.yaml", resp.RulesPath)
	}
	if len(resp.Builtin) != 10 {
		t.Errorf("builtin len = %d, want 10", len(resp.Builtin))
	}
	if len(resp.Custom) != 0 {
		t.Errorf("custom len = %d, want 0 (file does not exist)", len(resp.Custom))
	}
}

func TestAdminPutSentinelRules_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, configPath, map[string]any{
		"version":  "1",
		"sentinel": map[string]any{"rules_path": ".korva/sentinel-rules.yaml"},
	})

	h := adminPutSentinelRules(configPath)
	body := putSentinelRulesRequest{
		Profile: "custom",
		CustomRules: []*sentinelRule{
			{
				ID:           "CUSTOM-001",
				Description:  "no console.log",
				Severity:     "error",
				Pattern:      `console\.log`,
				PathsInclude: []string{"src/**/*.ts"},
				Message:      "no console.log",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/sentinel/rules", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Round-trip via GET.
	getH := adminGetSentinelRules(configPath)
	req2 := httptest.NewRequest(http.MethodGet, "/admin/sentinel/rules", nil)
	rec2 := httptest.NewRecorder()
	getH(rec2, req2)

	var resp struct {
		Custom []sentinelRule `json:"custom"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if len(resp.Custom) != 1 {
		t.Fatalf("expected 1 custom rule after PUT, got %d", len(resp.Custom))
	}
	if resp.Custom[0].ID != "CUSTOM-001" || resp.Custom[0].Pattern != `console\.log` {
		t.Errorf("rule mismatch after round-trip: %+v", resp.Custom[0])
	}
}

func TestAdminPutSentinelRules_RejectsInvalidRegex(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, configPath, map[string]any{"version": "1"})

	h := adminPutSentinelRules(configPath)
	body := putSentinelRulesRequest{
		CustomRules: []*sentinelRule{
			{ID: "BAD-001", Pattern: "[unclosed"},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/sentinel/rules", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	// File should NOT have been created on disk.
	matches, _ := filepath.Glob(filepath.Join(tmp, ".korva", "*.yaml"))
	if len(matches) > 0 {
		t.Errorf("expected no yaml file written, got %v", matches)
	}
}

func TestAdminPutSentinelRules_RejectsDuplicateID(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "korva.config.json")
	mustWriteJSON(t, configPath, map[string]any{"version": "1"})

	h := adminPutSentinelRules(configPath)
	body := putSentinelRulesRequest{
		CustomRules: []*sentinelRule{
			{ID: "DUP-001", Pattern: "x"},
			{ID: "DUP-001", Pattern: "y"},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/sentinel/rules", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminTestSentinelRule_Match(t *testing.T) {
	h := adminTestSentinelRule()
	body := testSentinelRuleRequest{
		Rule: sentinelRule{
			ID:           "TEST-001",
			Pattern:      `console\.log`,
			PathsInclude: []string{"src/**/*.ts"},
			Message:      "no console.log",
		},
		Code: `const x = 1
console.log("hello")
const y = 2`,
		FilePath: "src/app.ts",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/sentinel/test", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Matches []testSentinelRuleMatch `json:"matches"`
		Applies bool                    `json:"applies"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Applies {
		t.Error("applies should be true for src/app.ts")
	}
	if len(resp.Matches) != 1 {
		t.Errorf("matches len = %d, want 1", len(resp.Matches))
	}
	if resp.Matches[0].Line != 2 {
		t.Errorf("line = %d, want 2", resp.Matches[0].Line)
	}
}

func TestAdminTestSentinelRule_DoesNotApplyToFile(t *testing.T) {
	h := adminTestSentinelRule()
	body := testSentinelRuleRequest{
		Rule: sentinelRule{
			ID:           "TEST-002",
			Pattern:      `console\.log`,
			PathsInclude: []string{"src/**/*.ts"},
		},
		Code:     `console.log("hi")`,
		FilePath: "README.md",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/sentinel/test", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct {
		Applies bool `json:"applies"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Applies {
		t.Error("applies should be false for README.md")
	}
}

func TestSentinelRule_ValidateRejectsUnsafe(t *testing.T) {
	tests := []struct {
		name string
		r    sentinelRule
	}{
		{"lowercase id", sentinelRule{ID: "lower", Pattern: "x"}},
		{"empty pattern", sentinelRule{ID: "TEST-1", Pattern: ""}},
		{"bad regex", sentinelRule{ID: "TEST-2", Pattern: "["}},
		{"bad severity", sentinelRule{ID: "TEST-3", Pattern: "x", Severity: "fatal"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.Validate(); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestRulesFilePath_RespectsConfigOverride(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "korva.config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"sentinel":{"rules_path":"custom/path/rules.yaml"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfigOrEmpty(cfgPath)
	got := rulesFilePath(&cfg, tmp)
	want := filepath.Join(tmp, "custom/path/rules.yaml")
	if got != want {
		t.Errorf("rulesFilePath = %q, want %q", got, want)
	}
}
