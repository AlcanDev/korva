// Package mcp implements a Model Context Protocol (MCP) stdio server
// for the Vault. It speaks JSON-RPC 2.0 over stdin/stdout so any
// MCP-compatible AI assistant (Copilot, Claude Code, Cursor) can use it.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// Server is the MCP stdio server.
type Server struct {
	store  *store.Store
	reader *bufio.Reader
	writer io.Writer
	logger *log.Logger
}

// New creates an MCP server reading from stdin and writing to stdout.
func New(s *store.Store) *Server {
	return &Server{
		store:  s,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		logger: log.New(os.Stderr, "[vault-mcp] ", log.LstdFlags),
	}
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
	s.writeResult(req.ID, InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
		ServerInfo: ServerInfo{
			Name:    "korva-vault",
			Version: version.Version,
		},
	})
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
	if obs.Type == "" {
		obs.Type = store.TypeContext
	}

	id, err := s.store.Save(obs)
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id, "status": "saved"}, nil
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
	return map[string]any{"context": results, "project": project}, nil
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
