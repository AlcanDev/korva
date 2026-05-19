package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// TestCORSPreflight covers the fix for the OPTIONS-405 regression: Go 1.22+
// ServeMux registers routes as "METHOD /path" and answers OPTIONS with 405
// before the per-route withCORS wrapper runs. The corsPreflight middleware
// intercepts preflight requests and replies 204 with the headers withCORS
// would have set on the actual response.
func TestCORSPreflight(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	t.Setenv("KORVA_CORS_ORIGIN", "https://app.korva.dev")

	h := Router(context.Background(), s, RouterConfig{})

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantOrigin string
	}{
		{"api v1 status", "/api/v1/status", http.StatusNoContent, "https://app.korva.dev"},
		{"api v1 metrics", "/api/v1/metrics", http.StatusNoContent, "https://app.korva.dev"},
		{"hive v1 search", "/v1/search", http.StatusNoContent, "https://app.korva.dev"},
		{"team scrolls", "/team/scrolls", http.StatusNoContent, "https://app.korva.dev"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			req.Header.Set("Origin", "https://app.korva.dev")
			req.Header.Set("Access-Control-Request-Method", "GET")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != tc.wantOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tc.wantOrigin)
			}
			if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
				t.Errorf("Allow-Headers = %q, missing Authorization", got)
			}
			if got := rr.Header().Get("Access-Control-Max-Age"); got == "" {
				t.Error("Max-Age missing — browsers will re-preflight every request")
			}
		})
	}
}

// TestCORSPreflightSkipsMCP verifies the carve-out for /mcp* paths. The MCP
// HTTP handler advertises a wider Allow-Origin ("*") for multi-editor support
// and must own its own preflight response.
func TestCORSPreflightSkipsMCP(t *testing.T) {
	t.Setenv("KORVA_CORS_ORIGIN", "https://app.korva.dev")

	hits := 0
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)
	})

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	h := Router(context.Background(), s, RouterConfig{MCPHandler: mcpHandler})

	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if hits != 1 {
		t.Fatalf("MCP handler hits = %d, want 1 — preflight middleware swallowed OPTIONS for /mcp", hits)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want * — middleware overrode MCP's policy", got)
	}
}

// TestCORSPreflightPreservesGET ensures normal GET requests are not affected.
func TestCORSPreflightPreservesGET(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	h := Router(context.Background(), s, RouterConfig{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want 200", rr.Code)
	}
}

// TestIsMCPPath unit-checks the path matcher used by the carve-out.
func TestIsMCPPath(t *testing.T) {
	cases := map[string]bool{
		"/mcp":          true,
		"/mcp/":         true,
		"/mcp/anything": true,
		"/mcpish":       false,
		"/api/v1/mcp":   false,
		"":              false,
		"/":             false,
	}
	for p, want := range cases {
		if got := isMCPPath(p); got != want {
			t.Errorf("isMCPPath(%q) = %v, want %v", p, got, want)
		}
	}
}
