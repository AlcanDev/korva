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

func TestAdminMCPTools_ListsReadonlyTools(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/mcp/tools", nil)
	rec := httptest.NewRecorder()
	adminMCPTools()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Profile != "readonly" {
		t.Errorf("profile = %q, want readonly", resp.Profile)
	}
	if len(resp.Tools) == 0 {
		t.Fatal("expected at least one tool in the readonly profile")
	}
	// vault_search is the canonical readonly tool — it must be there.
	hasSearch := false
	for _, t := range resp.Tools {
		if t.Name == "vault_search" {
			hasSearch = true
		}
	}
	if !hasSearch {
		t.Errorf("vault_search not exposed in playground")
	}
}

func TestAdminMCPInvoke_RejectsUnknownTool(t *testing.T) {
	s := newAPITestStore(t)
	body, _ := json.Marshal(mcpInvokeRequest{Tool: "vault_definitely_not_real", Args: map[string]any{}})
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp/invoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	adminMCPInvoke(s)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminMCPInvoke_RejectsMutatingTools(t *testing.T) {
	// vault_save is NOT readonly — must be rejected even though it's a
	// real registered tool.
	s := newAPITestStore(t)
	body, _ := json.Marshal(mcpInvokeRequest{Tool: "vault_save", Args: map[string]any{"title": "x"}})
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp/invoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	adminMCPInvoke(s)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for mutating tool", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "readonly") {
		t.Errorf("error message should mention readonly profile, got: %s", rec.Body.String())
	}
}

func TestAdminMCPInvoke_RunsVaultSearch(t *testing.T) {
	s := newAPITestStore(t)
	// Seed one observation so search has something to return.
	if _, err := s.Save(store.Observation{
		Project: "korva", Type: store.TypeDecision,
		Title: "Use ULID", Content: "We picked ULID over UUID.",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	body, _ := json.Marshal(mcpInvokeRequest{
		Tool: "vault_search",
		Args: map[string]any{"q": "ULID", "project": "korva", "limit": float64(10)},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp/invoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	adminMCPInvoke(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Tool   string `json:"tool"`
		Result any    `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Tool != "vault_search" {
		t.Errorf("tool echo wrong: %q", resp.Tool)
	}
	if resp.Result == nil {
		t.Error("result should not be nil for a search that has a match")
	}
}

func TestAdminMCPInvoke_ValidatesBody(t *testing.T) {
	s := newAPITestStore(t)
	tests := []struct {
		name string
		body string
	}{
		{"empty tool", `{"args":{}}`},
		{"malformed", `not-json`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/mcp/invoke", bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			adminMCPInvoke(s)(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}
