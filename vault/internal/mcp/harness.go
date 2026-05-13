package mcp

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alcandev/korva/internal/harness"
)

// Phase 11 — Harness Engineering MCP tools.
//
// These let an AI agent talking to the Vault drive the state machine
// in any repo's `feature_list.json` without shelling out. Every tool
// resolves the target directory in this order:
//
//   1. the `root` argument when present
//   2. $KORVA_HARNESS_ROOT
//   3. the server's working directory
//
// Read tools are exposed in every profile; write tools (start/done/block/
// add/init) are gated to `agent` and `admin` profiles. Wired in
// server.go's dispatch + profiles.go's profile maps.

// resolveHarnessRoot picks the directory the tool will operate on.
// Always returns a non-empty string — `os.Getwd()` is the final fallback
// so we never hand the harness package an empty path.
func resolveHarnessRoot(args map[string]any) string {
	if r := stringArg(args, "root"); r != "" {
		return r
	}
	if env := os.Getenv("KORVA_HARNESS_ROOT"); env != "" {
		return env
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// agentName mirrors the CLI's defaultAgentName logic: tools may pass an
// explicit `agent` arg, otherwise we record the MCP session's email
// (when authenticated), otherwise "mcp".
func (s *Server) agentName(args map[string]any) string {
	if a := stringArg(args, "agent"); a != "" {
		return a
	}
	if s.session != nil && s.session.email != "" {
		return s.session.email
	}
	return "mcp"
}

// ── vault_harness_init ───────────────────────────────────────────────────────

func (s *Server) toolHarnessInit(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	project := stringArg(args, "project")
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	stack := harness.Stack(stringArg(args, "stack"))
	if stack == "" {
		stack = harness.StackGeneric
	}
	written, err := harness.Generate(harness.InitOptions{
		Root:          root,
		Project:       project,
		Description:   stringArg(args, "description"),
		Stack:         stack,
		WithSubagents: boolArg(args, "with_subagents"),
		Overwrite:     boolArg(args, "overwrite"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"root":          root,
		"project":       project,
		"stack":         string(stack),
		"files_written": written,
	}, nil
}

// ── vault_harness_status ─────────────────────────────────────────────────────

func (s *Server) toolHarnessStatus(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	resp := map[string]any{
		"project":     fl.Project,
		"description": fl.Description,
		"root":        root,
		"counts":      fl.CountByStatus(),
	}
	if cur := fl.CurrentInProgress(); cur != nil {
		resp["in_progress"] = featureToMap(*cur)
	}
	if next := fl.NextPending(); next != nil {
		resp["next_pending"] = featureToMap(*next)
	}
	return resp, nil
}

// ── vault_harness_list ───────────────────────────────────────────────────────

func (s *Server) toolHarnessList(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	statusFilter := stringArg(args, "status")
	out := make([]map[string]any, 0, len(fl.Features))
	for _, f := range fl.Features {
		if statusFilter != "" && string(f.Status) != statusFilter {
			continue
		}
		out = append(out, featureToMap(f))
	}
	return map[string]any{
		"project":  fl.Project,
		"root":     root,
		"features": out,
	}, nil
}

// ── vault_harness_next ───────────────────────────────────────────────────────

func (s *Server) toolHarnessNext(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	next := fl.NextPending()
	if next == nil {
		return map[string]any{
			"root":         root,
			"next_pending": nil,
			"message":      "Backlog is clear — no pending features.",
		}, nil
	}
	return map[string]any{
		"root":         root,
		"next_pending": featureToMap(*next),
	}, nil
}

// ── vault_harness_start / done / block / reopen (transitions) ────────────────

func (s *Server) toolHarnessTransition(target harness.FeatureStatus) func(map[string]any) (any, error) {
	return func(args map[string]any) (any, error) {
		root := resolveHarnessRoot(args)
		id, err := readIDArg(args)
		if err != nil {
			return nil, err
		}
		fl, err := harness.LoadFeatureList(root)
		if err != nil {
			return nil, err
		}
		owner := s.agentName(args)
		now := time.Now().UTC().Format(time.RFC3339)
		if err := fl.SetStatus(id, target, owner, now); err != nil {
			return nil, err
		}
		if err := harness.SaveFeatureList(root, fl); err != nil {
			return nil, err
		}
		f := fl.FindByID(id)
		return map[string]any{
			"root":    root,
			"id":      id,
			"name":    f.Name,
			"status":  string(target),
			"owner":   owner,
			"updated": now,
		}, nil
	}
}

// ── vault_harness_add ────────────────────────────────────────────────────────

func (s *Server) toolHarnessAdd(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	name := stringArg(args, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	nextID := 1
	for _, f := range fl.Features {
		if f.ID >= nextID {
			nextID = f.ID + 1
		}
	}
	title := stringArg(args, "title")
	if title == "" {
		title = name
	}
	var acceptance []string
	if v, ok := args["acceptance"]; ok {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					acceptance = append(acceptance, s)
				}
			}
		}
	}
	feature := harness.Feature{
		ID:          nextID,
		Name:        name,
		Title:       title,
		Description: stringArg(args, "description"),
		Acceptance:  acceptance,
		Status:      harness.StatusPending,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	fl.Features = append(fl.Features, feature)
	if err := harness.SaveFeatureList(root, fl); err != nil {
		return nil, err
	}
	return map[string]any{
		"root":    root,
		"feature": featureToMap(feature),
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// featureToMap converts a harness.Feature into the JSON envelope every
// vault_harness_* tool returns. Kept in one place so the wire shape is
// consistent across tools (and easy to assert in tests).
func featureToMap(f harness.Feature) map[string]any {
	out := map[string]any{
		"id":     f.ID,
		"name":   f.Name,
		"title":  f.Title,
		"status": string(f.Status),
	}
	if f.Description != "" {
		out["description"] = f.Description
	}
	if len(f.Acceptance) > 0 {
		out["acceptance"] = f.Acceptance
	}
	if f.OwnerAgent != "" {
		out["owner_agent"] = f.OwnerAgent
	}
	if f.UpdatedAt != "" {
		out["updated_at"] = f.UpdatedAt
	}
	return out
}

// readIDArg accepts the feature id as either a JSON number (the common
// case for MCP clients) or a string the agent typed in by hand.
func readIDArg(args map[string]any) (int, error) {
	if v, ok := args["id"]; ok {
		switch n := v.(type) {
		case float64:
			return int(n), nil
		case int:
			return n, nil
		case string:
			parsed, err := strconv.Atoi(n)
			if err != nil {
				return 0, fmt.Errorf("id %q is not an integer", n)
			}
			return parsed, nil
		}
	}
	return 0, fmt.Errorf("id is required")
}
