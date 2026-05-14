package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func newAPITestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("NewMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIngestInteraction_HappyPath(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{
		"project": "korva",
		"agent":   "claude",
		"model":   "claude-opus-4-7",
		"prompt":  "implement /admin/system-status",
		"response": "done",
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 200,
			"cache_read_input_tokens": 800,
			"cache_creation_input_tokens": 100
		},
		"duration_ms": 1234,
		"tool_calls": [{"name":"Read"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID        string `json:"id"`
		Estimated bool   `json:"estimated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response error = %v", err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty id")
	}
	if resp.Estimated {
		t.Error("expected estimated=false when usage is provided")
	}

	got, _ := s.GetInteraction(resp.ID)
	if got == nil || got.InputTokens != 1000 || got.CacheRead != 800 {
		t.Errorf("stored interaction mismatch: %+v", got)
	}
}

// ───────────────────────── Phase 18.C — editor telemetry header ─────────────

func TestIngestInteraction_RecordsEditorHeader(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{"project":"p","agent":"a","model":"m","prompt":"x","response":"y","duration_ms":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	req.Header.Set("X-Korva-Editor", "cursor")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	got, _ := s.GetInteraction(resp.ID)
	if got == nil {
		t.Fatal("not stored")
	}
	if got.Editor != "cursor" {
		t.Errorf("editor = %q, want cursor", got.Editor)
	}
}

func TestIngestInteraction_NormalizesEditorCase(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{"project":"p","agent":"a","model":"m","prompt":"x","response":"y","duration_ms":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	req.Header.Set("X-Korva-Editor", "  CURSOR  ")
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	got, _ := s.GetInteraction(resp.ID)
	if got.Editor != "cursor" {
		t.Errorf("editor = %q, want lowercased+trimmed", got.Editor)
	}
}

func TestIngestInteraction_RejectsUnknownEditor(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	// "neovim" isn't in harness.AllEditors → header should be ignored
	// (empty string stored). We do NOT 400 — being strict here would
	// break ingest for future editors that haven't been added yet.
	body := `{"project":"p","agent":"a","model":"m","prompt":"x","response":"y","duration_ms":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	req.Header.Set("X-Korva-Editor", "neovim")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	got, _ := s.GetInteraction(resp.ID)
	if got.Editor != "" {
		t.Errorf("editor = %q, want empty (unknown should be ignored)", got.Editor)
	}
}

func TestIngestInteraction_OmittedEditorIsAnonymous(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{"project":"p","agent":"a","model":"m","prompt":"x","response":"y","duration_ms":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	got, _ := s.GetInteraction(resp.ID)
	if got.Editor != "" {
		t.Errorf("editor = %q, want empty when header absent", got.Editor)
	}
}

func TestIngestInteraction_DisabledEnvIgnoresHeader(t *testing.T) {
	t.Setenv(editorTelemetryDisabledEnv, "1")
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{"project":"p","agent":"a","model":"m","prompt":"x","response":"y","duration_ms":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	req.Header.Set("X-Korva-Editor", "cursor")
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	got, _ := s.GetInteraction(resp.ID)
	if got.Editor != "" {
		t.Errorf("editor = %q, want empty when telemetry is disabled", got.Editor)
	}
}

func TestIngestInteraction_RequiresProjectAndAgent(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	tests := []struct {
		name string
		body string
	}{
		{"missing project", `{"agent":"claude"}`},
		{"missing agent", `{"project":"korva"}`},
		{"invalid json", `not-json`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions",
				bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestIngestInteraction_FallbackEstimated(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{
		"project": "korva",
		"agent":   "claude",
		"prompt":  "` + strings.Repeat("a", 400) + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Estimated bool `json:"estimated"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Estimated {
		t.Error("expected estimated=true when usage is omitted")
	}
}

func TestIngestInteraction_PrivacyFilterApplied(t *testing.T) {
	s := newAPITestStore(t)
	h := ingestInteraction(s)

	body := `{"project":"korva","agent":"claude","prompt":"password=hunter2 token=abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interactions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	got, _ := s.GetInteraction(resp.ID)
	if strings.Contains(got.PromptExcerpt, "hunter2") || strings.Contains(got.PromptExcerpt, "abc123") {
		t.Errorf("privacy filter not applied: %q", got.PromptExcerpt)
	}
}

func TestAdminListActivity_FiltersAndDetail(t *testing.T) {
	s := newAPITestStore(t)
	for _, in := range []store.Interaction{
		{Project: "korva", Agent: "claude", Model: "opus", PromptExcerpt: "build system status endpoint"},
		{Project: "other", Agent: "cursor", Model: "sonnet", PromptExcerpt: "fix bug in adapter"},
	} {
		if _, err := s.SaveInteraction(in); err != nil {
			t.Fatalf("seed error = %v", err)
		}
	}

	list := adminListActivity(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/activity?project=korva", nil)
	rec := httptest.NewRecorder()
	list(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Interactions []map[string]any `json:"interactions"`
		Total        int              `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Interactions) != 1 {
		t.Errorf("expected 1 interaction for project=korva, got %d", len(listResp.Interactions))
	}
	if listResp.Total != 1 {
		t.Errorf("total = %d, want 1", listResp.Total)
	}

	// FTS search omits total (-1 → not in response)
	req2 := httptest.NewRequest(http.MethodGet, "/admin/activity?q=adapter", nil)
	rec2 := httptest.NewRecorder()
	list(rec2, req2)
	var ftsResp struct {
		Interactions []map[string]any `json:"interactions"`
		Total        *int             `json:"total"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &ftsResp)
	if len(ftsResp.Interactions) != 1 {
		t.Errorf("FTS search len = %d, want 1", len(ftsResp.Interactions))
	}
	if ftsResp.Total != nil {
		t.Errorf("FTS response should omit total, got %v", ftsResp.Total)
	}
}

func TestAdminGetActivity_Detail(t *testing.T) {
	s := newAPITestStore(t)
	id, _ := s.SaveInteraction(store.Interaction{
		Project: "korva", Agent: "claude", PromptExcerpt: "hello",
	})

	h := adminGetActivity(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/activity/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	// missing id
	req2 := httptest.NewRequest(http.MethodGet, "/admin/activity/nope", nil)
	req2.SetPathValue("id", "01J0NOTEXIST")
	rec2 := httptest.NewRecorder()
	h(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec2.Code)
	}
}

func TestAdminTokenStats(t *testing.T) {
	s := newAPITestStore(t)
	for _, in := range []store.Interaction{
		{Project: "korva", Agent: "claude", Model: "opus", InputTokens: 1000, OutputTokens: 200, CacheRead: 500},
		{Project: "korva", Agent: "claude", Model: "opus", InputTokens: 2000, OutputTokens: 400, CacheRead: 1000},
	} {
		if _, err := s.SaveInteraction(in); err != nil {
			t.Fatalf("seed error = %v", err)
		}
	}

	t.Setenv(repoBaselineEnvVar, t.TempDir())
	h := adminTokenStats(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/tokens/stats", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Totals      map[string]any `json:"totals"`
		CacheHitPct float64        `json:"cache_hit_pct"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if v, _ := resp.Totals["input_tokens"].(float64); v != 3000 {
		t.Errorf("input_tokens = %v, want 3000", v)
	}
	if resp.CacheHitPct <= 0 {
		t.Errorf("cache_hit_pct should be > 0, got %f", resp.CacheHitPct)
	}
}

func TestEstimateBaselineTokens_SkipsNoise(t *testing.T) {
	dir := t.TempDir()

	mustWrite := func(path, content string) {
		full := dir + "/" + path
		// Ensure parent exists.
		_ = mkdirAll(full)
		writeFile(t, full, content)
	}

	mustWrite("src/main.go", strings.Repeat("a", 400))
	mustWrite("src/handler.go", strings.Repeat("b", 400))
	mustWrite("node_modules/foo.js", strings.Repeat("z", 9999))
	mustWrite("dist/bundle.js", strings.Repeat("z", 9999))
	mustWrite(".git/HEAD", "ref")

	got := estimateBaselineTokens(dir)
	// Only 800 bytes of source counted; 800 / 4 = 200 tokens.
	if got != 200 {
		t.Errorf("baseline = %d, want 200 (only src/ counted)", got)
	}
}
