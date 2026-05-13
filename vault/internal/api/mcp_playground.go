package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/mcp"
	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 10.2 — MCP playground admin endpoints.
//
//   GET  /admin/mcp/tools    list of read-only MCP tools + their JSON schemas
//   POST /admin/mcp/invoke   body {tool, args} → tool output
//
// Hard-capped to the Readonly profile (search / context / stats / get /
// timeline / summary / hints). Mutations stay out — operators can prove the
// tools work without touching state, and the playground is safe to expose
// in shared admin dashboards.

func adminMCPTools() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"tools":   mcp.PlaygroundTools(),
			"profile": "readonly",
		})
	}
}

type mcpInvokeRequest struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

func adminMCPInvoke(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mcpInvokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Tool == "" {
			writeError(w, http.StatusBadRequest, "tool is required")
			return
		}
		if req.Args == nil {
			req.Args = map[string]any{}
		}
		result, err := mcp.Invoke(r.Context(), s, req.Tool, req.Args)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tool":   req.Tool,
			"result": result,
		})
	}
}
