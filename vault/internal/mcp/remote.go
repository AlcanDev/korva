// RemoteDispatcher implements the Dispatcher interface by proxying every
// JSON-RPC request to a remote vault's Streamable HTTP MCP endpoint
// (typically https://api.korva.dev/mcp or any self-hosted equivalent).
//
// This is what makes "team memory" real: when a developer's editor spawns
// `korva-vault --mode mcp` and KORVA_VAULT_ENDPOINT is set, the local
// process becomes a thin stdio↔HTTP transcoder instead of touching local
// SQLite. Every member of the team writes to the same central store.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultRemoteTimeout caps a single proxied tool call. Long enough for
// vault_export_lore on a large team, short enough to surface a hang in
// the editor instead of looking like the MCP froze. Override with
// KORVA_VAULT_TIMEOUT (Go duration string, e.g. "60s").
const defaultRemoteTimeout = 30 * time.Second

// RemoteDispatcher forwards JSON-RPC requests over HTTPS to a vault that
// exposes the MCP Streamable HTTP transport at <endpoint>/mcp.
//
// Concurrency: the dispatcher holds a single *http.Client. The stdio
// runloop dispatches at most one request at a time, so no per-call
// locking is needed. The HTTP client itself is safe for concurrent use.
type RemoteDispatcher struct {
	endpoint string        // base URL, e.g. "https://api.korva.dev". No trailing /mcp.
	token    string        // bearer for upstream auth. May be empty (anonymous).
	client   *http.Client  // reused across calls; keep-alive amortizes TLS.
	timeout  time.Duration // per-request budget.
}

// NewRemoteDispatcher constructs a dispatcher that posts to endpoint/mcp.
// A trailing slash on endpoint is harmless; the constructor normalizes it.
// Pass empty token to operate without auth (only useful when the upstream
// has KORVA_MCP_ALLOW_ANONYMOUS=true, i.e. local development).
func NewRemoteDispatcher(endpoint, token string) *RemoteDispatcher {
	return &RemoteDispatcher{
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		token:    strings.TrimSpace(token),
		client: &http.Client{
			// One persistent connection per host keeps p50 around one RTT.
			// Read/write deadlines come from the per-request context below.
			Transport: &http.Transport{
				MaxIdleConns:        4,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		timeout: defaultRemoteTimeout,
	}
}

// WithTimeout returns a new dispatcher with the per-request budget overridden.
// Useful for tools that legitimately take longer than 30s.
func (d *RemoteDispatcher) WithTimeout(t time.Duration) *RemoteDispatcher {
	cp := *d
	cp.timeout = t
	return &cp
}

// HandleRequest implements the Dispatcher interface. Marshals the request
// envelope, POSTs to <endpoint>/mcp with Authorization: Bearer, decodes the
// response envelope. Network and decode failures are translated to JSON-RPC
// error responses so the stdio loop keeps running — an editor must never
// see the MCP "crash" because of a transient upstream blip.
func (d *RemoteDispatcher) HandleRequest(req Request) Response {
	body, err := json.Marshal(req)
	if err != nil {
		return makeError(req.ID, -32603, "internal error", fmt.Sprintf("marshal request: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint+"/mcp", bytes.NewReader(body))
	if err != nil {
		return makeError(req.ID, -32010, "vault unreachable", fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if d.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+d.token)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return makeError(req.ID, -32010, "vault unreachable", err.Error())
	}
	defer resp.Body.Close()

	// Limit the upstream body to 4 MiB — defensive; the upstream's own
	// limit is 1 MiB but a misbehaving proxy could send anything.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return makeError(req.ID, -32010, "vault unreachable", fmt.Sprintf("read response: %v", err))
	}

	// 401 / 403 — propagate the upstream's JSON-RPC error verbatim if it
	// followed the spec (mcp.korva.dev returns {"jsonrpc":"2.0","error":...}
	// with code -32001). If the body is non-JSON, synthesize an envelope so
	// the editor still sees a structured error.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		var passthrough Response
		if json.Unmarshal(raw, &passthrough) == nil && passthrough.Error != nil {
			return passthrough
		}
		return makeError(req.ID, -32001, "unauthorized",
			fmt.Sprintf("upstream %d: %s", resp.StatusCode, truncate(string(raw), 200)))
	}

	if resp.StatusCode >= 400 {
		return makeError(req.ID, -32603, "internal error",
			fmt.Sprintf("upstream %d: %s", resp.StatusCode, truncate(string(raw), 200)))
	}

	var out Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return makeError(req.ID, -32700, "parse error",
			fmt.Sprintf("upstream response not JSON-RPC: %v", err))
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
