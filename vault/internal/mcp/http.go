// HTTP (Streamable HTTP) transport for the MCP server.
//
// Implements the simplest valid form of the MCP Streamable HTTP transport
// (spec 2025-03-26): a single endpoint that accepts POST with a JSON-RPC
// request body and returns the JSON-RPC response inline. No SSE streams are
// emitted because no tool produces server-initiated messages today; clients
// that open GET for a stream receive 405.
//
// Authentication: Bearer token in the standard Authorization header. The
// token is validated against member_sessions exactly as the stdio transport
// does, so a `korva auth redeem` token works identically over both wires.
//
// Concurrency: each HTTP request gets a shallow clone of the Server with
// isolated session-scoped fields, so two clients with different tokens never
// race on s.session.
package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

// maxMCPBodyBytes caps the JSON-RPC request body to defend against memory
// bombs. 1 MiB matches the limit used by the REST API.
const maxMCPBodyBytes = 1 << 20

// HTTPHandler returns an http.Handler that speaks MCP Streamable HTTP backed
// by this server. The handler is safe for concurrent use; per-request state
// (session identity, active session id, context cache) is isolated via
// forRequest().
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.servePOST(w, r)
	case http.MethodGet, http.MethodDelete:
		// Server-initiated streams and explicit session termination are not
		// implemented yet. Returning 405 keeps clients on the POST-only path.
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	case http.MethodOptions:
		writeMCPCORS(w)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) servePOST(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMCPBodyBytes))
	if err != nil {
		writeJSONRPC(w, http.StatusBadRequest, makeError(nil, -32700, "parse error", err.Error()))
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPC(w, http.StatusBadRequest, makeError(nil, -32700, "parse error", err.Error()))
		return
	}

	// Notifications (no id) get an empty 202 — the spec forbids returning a
	// JSON-RPC response for them.
	isNotification := req.ID == nil

	// Per-request server clone. Bearer token, when present, populates session.
	sess := s.forRequest()
	if token := bearerToken(r); token != "" {
		sess.resolveSession(token)
	}

	// Auth gate: default-deny. The remote MCP endpoint is private — every
	// request must carry a valid session token. The escape hatch
	// KORVA_MCP_ALLOW_ANONYMOUS=true is for local development only.
	if sess.session == nil && !mcpAllowAnonymous() {
		writeJSONRPC(w, http.StatusUnauthorized, makeError(req.ID, -32001, "unauthorized",
			"valid bearer token required — set Authorization: Bearer <session_token>"))
		return
	}

	resp := sess.HandleRequest(req)

	if isNotification {
		writeMCPCORS(w)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	writeJSONRPC(w, http.StatusOK, resp)
}

// mcpAllowAnonymous returns true when the operator has explicitly opted in to
// anonymous access via KORVA_MCP_ALLOW_ANONYMOUS=true. Default false.
func mcpAllowAnonymous() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KORVA_MCP_ALLOW_ANONYMOUS")))
	return v == "true" || v == "1" || v == "yes"
}

// forRequest returns a shallow Server copy that shares the heavy, immutable
// dependencies (store, logger, cloud, license, profile) but owns the
// session-scoped fields. Callers mutate the returned value freely without
// affecting other in-flight HTTP requests.
func (s *Server) forRequest() *Server {
	return &Server{
		store:        s.store,
		logger:       s.logger,
		cloud:        s.cloud,
		lic:          s.lic,
		profile:      s.profile,
		contextCache: make(map[string]*contextCacheEntry),
	}
}

// bearerToken extracts the token from a standard `Authorization: Bearer …`
// header. Returns "" when the header is missing or malformed.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func writeJSONRPC(w http.ResponseWriter, status int, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}
	writeMCPCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// writeMCPCORS sets the CORS headers needed for browser-based MCP clients.
// The MCP endpoint typically receives requests from native processes, but
// the headers cost nothing and unlock Beacon-driven debug tools.
func writeMCPCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
}
