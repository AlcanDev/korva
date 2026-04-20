// Package mcp implements a Model Context Protocol (MCP) stdio server
// for the Vault. It speaks JSON-RPC 2.0 over stdin/stdout so any
// MCP-compatible AI assistant (Copilot, Claude Code, Cursor) can use it.
package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// mcpSession holds the team identity resolved from the session token passed
// in the initialize params. Nil when the client is unauthenticated.
type mcpSession struct {
	teamID string
	email  string
	role   string // "admin" or "member"
}

// Server is the MCP stdio server.
type Server struct {
	store   *store.Store
	reader  *bufio.Reader
	writer  io.Writer
	logger  *log.Logger
	session *mcpSession // nil = anonymous; set during handleInitialize if valid token
}

// New creates an MCP server reading from stdin and writing to stdout.
//
// On startup it auto-loads a session token (team identity) from:
//  1. KORVA_SESSION_TOKEN environment variable
//  2. ~/.korva/session.token file (written by `korva auth redeem`)
//
// This is editor-agnostic: Claude Code, Cursor, Copilot, and any other
// MCP host automatically get team context without extra configuration.
// The session can also be overridden via initialize.params.session_token.
func New(s *store.Store) *Server {
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		logger: log.New(os.Stderr, "[vault-mcp] ", log.LstdFlags),
	}
	if token := loadSessionToken(); token != "" {
		srv.resolveSession(token)
	}
	return srv
}

// loadSessionToken reads the session token from the environment variable
// KORVA_SESSION_TOKEN or, if unset, from ~/.korva/session.token.
// Returns an empty string when neither source is available.
func loadSessionToken() string {
	if t := os.Getenv("KORVA_SESSION_TOKEN"); t != "" {
		return strings.TrimSpace(t)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".korva", "session.token"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Run starts the MCP server loop. It blocks until stdin is closed or an
// unrecoverable error occurs.
func (s *Server) Run() error {
	s.logger.Printf("Korva Vault MCP server starting (%s)", version.String())

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(nil, -32700, "parse error", err.Error())
			continue
		}

		s.handleRequest(req)
	}
}

func (s *Server) handleRequest(req Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "ping":
		s.writeResult(req.ID, map[string]string{"pong": "pong"})
	default:
		s.writeError(req.ID, -32601, "method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req Request) {
	// Attempt to resolve an optional session token from the initialize params.
	// Clients that have a ~/.korva/session.token should pass it here so that
	// MCP tools automatically carry team context.
	if req.Params != nil {
		var params struct {
			SessionToken string `json:"session_token"`
		}
		if json.Unmarshal(req.Params, &params) == nil && params.SessionToken != "" {
			s.resolveSession(params.SessionToken)
		}
	}

	s.writeResult(req.ID, InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
		ServerInfo: ServerInfo{
			Name:    "korva-vault",
			Version: version.Version,
		},
	})
}

// resolveSession validates the plaintext session token against the DB and
// stores the resulting identity in s.session. Errors are silently discarded
// so that an invalid token degrades gracefully to anonymous mode.
func (s *Server) resolveSession(token string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	var sess mcpSession
	err := s.store.DB().QueryRowContext(context.Background(),
		`SELECT ms.team_id, ms.email, COALESCE(tm.role, 'member')
		   FROM member_sessions ms
		   LEFT JOIN team_members tm
		          ON tm.team_id = ms.team_id AND tm.email = ms.email
		  WHERE ms.token_hash = ? AND ms.expires_at > datetime('now')`, hash).
		Scan(&sess.teamID, &sess.email, &sess.role)
	if err == nil {
		s.session = &sess
		s.logger.Printf("MCP session: %s role=%s team=%s", sess.email, sess.role, sess.teamID)
	}
}

// fetchTeamContext queries the team's skills and private scrolls from the DB.
// Returns empty slices when there is no data or no active session.
func (s *Server) fetchTeamContext() (skills, scrolls []map[string]any) {
	if s.session == nil {
		return nil, nil
	}
	ctx := context.Background()

	skillRows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, body FROM skills WHERE team_id = ? ORDER BY name ASC`,
		s.session.teamID)
	if err == nil {
		defer skillRows.Close()
		for skillRows.Next() {
			var name, body string
			if skillRows.Scan(&name, &body) == nil {
				skills = append(skills, map[string]any{"name": name, "body": body})
			}
		}
	}

	scrollRows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, content FROM private_scrolls WHERE team_id = ? ORDER BY name ASC`,
		s.session.teamID)
	if err == nil {
		defer scrollRows.Close()
		for scrollRows.Next() {
			var name, content string
			if scrollRows.Scan(&name, &content) == nil {
				scrolls = append(scrolls, map[string]any{"name": name, "content": content})
			}
		}
	}
	return
}

func (s *Server) handleToolsList(req Request) {
	s.writeResult(req.ID, map[string]any{
		"tools": tools(),
	})
}

func (s *Server) handleToolsCall(req Request) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, -32602, "invalid params", err.Error())
		return
	}

	result, err := s.dispatch(params.Name, params.Arguments)
	if err != nil {
		s.writeToolError(req.ID, err.Error())
		return
	}

	s.writeResult(req.ID, ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: toJSON(result)}},
	})
}

func (s *Server) dispatch(tool string, args map[string]any) (any, error) {
	switch tool {
	case "vault_save":
		return s.toolSave(args)
	case "vault_search":
		return s.toolSearch(args)
	case "vault_context":
		return s.toolContext(args)
	case "vault_timeline":
		return s.toolTimeline(args)
	case "vault_get":
		return s.toolGet(args)
	case "vault_session_start":
		return s.toolSessionStart(args)
	case "vault_session_end":
		return s.toolSessionEnd(args)
	case "vault_summary":
		return s.toolSummary(args)
	case "vault_save_prompt":
		return s.toolSavePrompt(args)
	case "vault_stats":
		return s.toolStats(args)
	case "vault_delete":
		return s.toolDelete(args)
	case "vault_bulk_save":
		return s.toolBulkSave(args)
	case "vault_query":
		return s.toolQuery(args)
	case "vault_team_context":
		return s.toolTeamContext(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

// --- tool implementations ---

func (s *Server) toolSave(args map[string]any) (any, error) {
	obs := store.Observation{
		Project: stringArg(args, "project"),
		Team:    stringArg(args, "team"),
		Country: stringArg(args, "country"),
		Type:    store.ObservationType(stringArg(args, "type")),
		Title:   stringArg(args, "title"),
		Content: stringArg(args, "content"),
		Author:  stringArg(args, "author"),
		Tags:    stringSliceArg(args, "tags"),
	}
	// Auto-fill team from the active session so members don't have to pass it explicitly.
	if obs.Team == "" && s.session != nil {
		obs.Team = s.session.teamID
	}
	if obs.Type == "" {
		obs.Type = store.TypeContext
	}

	id, err := s.store.Save(obs)
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id, "status": "saved"}, nil
}

func (s *Server) toolBulkSave(args map[string]any) (any, error) {
	rawItems, ok := args["observations"]
	if !ok {
		return nil, fmt.Errorf("observations is required")
	}
	items, ok := rawItems.([]any)
	if !ok {
		return nil, fmt.Errorf("observations must be an array")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("observations array is empty")
	}
	const maxBulk = 50
	if len(items) > maxBulk {
		return nil, fmt.Errorf("too many observations: max %d per call, got %d", maxBulk, len(items))
	}

	ids := make([]string, 0, len(items))
	var errs []string

	for i, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("item[%d]: not an object", i))
			continue
		}
		obs := store.Observation{
			Project: stringArg(m, "project"),
			Team:    stringArg(m, "team"),
			Country: stringArg(m, "country"),
			Type:    store.ObservationType(stringArg(m, "type")),
			Title:   stringArg(m, "title"),
			Content: stringArg(m, "content"),
			Author:  stringArg(m, "author"),
			Tags:    stringSliceArg(m, "tags"),
		}
		if obs.Team == "" && s.session != nil {
			obs.Team = s.session.teamID
		}
		if obs.Type == "" {
			obs.Type = store.TypeContext
		}
		id, err := s.store.Save(obs)
		if err != nil {
			errs = append(errs, fmt.Sprintf("item[%d]: %v", i, err))
			continue
		}
		ids = append(ids, id)
	}

	result := map[string]any{
		"saved": len(ids),
		"ids":   ids,
	}
	if len(errs) > 0 {
		result["errors"] = errs
	}
	return result, nil
}

func (s *Server) toolSearch(args map[string]any) (any, error) {
	results, err := s.store.Search(
		stringArg(args, "query"),
		store.SearchFilters{
			Project: stringArg(args, "project"),
			Team:    stringArg(args, "team"),
			Country: stringArg(args, "country"),
			Type:    store.ObservationType(stringArg(args, "type")),
			Limit:   intArg(args, "limit", 20),
		},
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "count": len(results)}, nil
}

func (s *Server) toolContext(args map[string]any) (any, error) {
	project := stringArg(args, "project")
	limit := intArg(args, "limit", 10)

	results, err := s.store.Context(project, nil, limit)
	if err != nil {
		return nil, err
	}

	resp := map[string]any{"context": results, "project": project}

	// When a session is active, enrich the context with the team's custom
	// skills and private scrolls so the AI carries all team knowledge.
	if s.session != nil {
		skills, scrolls := s.fetchTeamContext()
		if len(skills) > 0 {
			resp["team_skills"] = skills
		}
		if len(scrolls) > 0 {
			resp["team_scrolls"] = scrolls
		}
		resp["team_id"] = s.session.teamID
	}

	return resp, nil
}

// toolTeamContext returns the team's custom skills and private scrolls.
// It works with or without a session: without a session it returns an empty
// result with a hint about how to authenticate.
func (s *Server) toolTeamContext(_ map[string]any) (any, error) {
	if s.session == nil {
		return map[string]any{
			"team_id": "",
			"skills":  []any{},
			"scrolls": []any{},
			"note":    "no active session — pass session_token in initialize params to load team context",
		}, nil
	}

	skills, scrolls := s.fetchTeamContext()

	// Return empty slices, not null, for consistent JSON handling.
	if skills == nil {
		skills = []map[string]any{}
	}
	if scrolls == nil {
		scrolls = []map[string]any{}
	}

	return map[string]any{
		"team_id": s.session.teamID,
		"email":   s.session.email,
		"role":    s.session.role,
		"skills":  skills,
		"scrolls": scrolls,
	}, nil
}

func (s *Server) toolTimeline(args map[string]any) (any, error) {
	project := stringArg(args, "project")

	from := time.Now().Add(-7 * 24 * time.Hour)
	to := time.Now()

	if fromStr := stringArg(args, "from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr := stringArg(args, "to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	results, err := s.store.Timeline(project, from, to)
	if err != nil {
		return nil, err
	}
	return map[string]any{"timeline": results, "project": project}, nil
}

func (s *Server) toolGet(args map[string]any) (any, error) {
	id := stringArg(args, "id")
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	obs, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}
	if obs == nil {
		return map[string]any{"found": false}, nil
	}
	return map[string]any{"found": true, "observation": obs}, nil
}

func (s *Server) toolSessionStart(args map[string]any) (any, error) {
	id, err := s.store.SessionStart(
		stringArg(args, "project"),
		stringArg(args, "team"),
		stringArg(args, "country"),
		stringArg(args, "agent"),
		stringArg(args, "goal"),
	)
	if err != nil {
		return nil, err
	}
	return map[string]string{"session_id": id, "status": "started"}, nil
}

func (s *Server) toolSessionEnd(args map[string]any) (any, error) {
	id := stringArg(args, "session_id")
	if id == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if err := s.store.SessionEnd(id, stringArg(args, "summary")); err != nil {
		return nil, err
	}
	return map[string]string{"status": "ended"}, nil
}

func (s *Server) toolSummary(args map[string]any) (any, error) {
	return s.store.Summary(stringArg(args, "project"))
}

func (s *Server) toolSavePrompt(args map[string]any) (any, error) {
	if err := s.store.SavePrompt(
		stringArg(args, "name"),
		stringArg(args, "content"),
		stringSliceArg(args, "tags"),
	); err != nil {
		return nil, err
	}
	return map[string]string{"status": "saved"}, nil
}

func (s *Server) toolStats(args map[string]any) (any, error) {
	return s.store.Stats()
}

func (s *Server) toolDelete(args map[string]any) (any, error) {
	id := stringArg(args, "id")
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	deleted, err := s.store.Delete(id)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return map[string]any{"deleted": false, "message": "observation not found"}, nil
	}
	return map[string]any{"deleted": true, "id": id}, nil
}

func (s *Server) toolQuery(args map[string]any) (any, error) {
	filters := store.SearchFilters{
		Project: stringArg(args, "project"),
		Team:    stringArg(args, "team"),
		Type:    store.ObservationType(stringArg(args, "type")),
		Limit:   intArg(args, "limit", 20),
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	if sinceStr := stringArg(args, "since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filters.Since = t
		}
	}
	if untilStr := stringArg(args, "until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			filters.Until = t
		}
	}

	// vault_query uses the non-FTS path (empty query string = recent observations
	// sorted by date, filtered by the struct fields including Since/Until).
	results, err := s.store.Search("", filters)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "count": len(results)}, nil
}

// --- write helpers ---

func (s *Server) writeResult(id any, result any) {
	s.write(Response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeError(id any, code int, message, data string) {
	s.write(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message, Data: data},
	})
}

func (s *Server) writeToolError(id any, msg string) {
	s.writeResult(id, ToolCallResult{
		Content:  []ContentBlock{{Type: "text", Text: msg}},
		IsError:  true,
	})
}

func (s *Server) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		s.logger.Printf("marshal error: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}

// --- argument helpers ---

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func stringSliceArg(args map[string]any, key string) []string {
	if v, ok := args[key]; ok {
		if arr, ok := v.([]any); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return []string{}
}

func toJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
