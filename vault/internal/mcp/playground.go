package mcp

import (
	"context"
	"fmt"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 10.2 — MCP Playground: invoke read-only MCP tools from outside the
// MCP server's stdio loop. Used by the /admin/mcp/* HTTP endpoints to power
// the Beacon "MCP Playground" panel.
//
// Safety: we hard-cap to the Readonly profile. Operators experimenting in
// the UI never touch the database state — search / context / stats / get /
// timeline / summary / suggestion helpers only. Anything that mutates
// (vault_save, vault_delete, vault_judge, …) is rejected.

// PlaygroundTool is a wire-friendly subset of Tool — we drop the InputSchema
// type and inline it so callers don't have to depend on the protocol types.
type PlaygroundTool struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	InputSchema PlaygroundToolSchema `json:"input_schema"`
}

// PlaygroundToolSchema mirrors Schema/Property but with JSON-friendly types.
type PlaygroundToolSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]PlaygroundToolProp `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

// PlaygroundToolProp matches Property.
type PlaygroundToolProp struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// PlaygroundTools returns the subset of MCP tools allowed by the Readonly
// profile, in the wire shape expected by the dashboard.
func PlaygroundTools() []PlaygroundTool {
	src := toolsForProfile(ProfileReadonly)
	out := make([]PlaygroundTool, 0, len(src))
	for _, t := range src {
		// Property and PlaygroundToolProp are structurally identical, so a
		// direct type conversion is both correct and what staticcheck S1016
		// recommends. The named type still carries the public wire contract.
		props := map[string]PlaygroundToolProp{}
		for k, p := range t.InputSchema.Properties {
			props[k] = PlaygroundToolProp(p)
		}
		out = append(out, PlaygroundTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: PlaygroundToolSchema{
				Type:       t.InputSchema.Type,
				Properties: props,
				Required:   t.InputSchema.Required,
			},
		})
	}
	return out
}

// Invoke runs a single MCP tool with the given args under the Readonly
// profile. Returns the raw tool output (typically a struct) so callers can
// JSON-marshal it for the wire. Errors when the tool is unknown OR not
// allowed by the Readonly profile.
//
// `ctx` is reserved for future cancelation paths (the existing tools don't
// honor it yet; passing it now keeps the signature future-proof).
func Invoke(_ context.Context, s *store.Store, tool string, args map[string]any) (any, error) {
	srv := &Server{
		store:        s,
		profile:      ProfileReadonly,
		contextCache: make(map[string]*contextCacheEntry),
	}
	if !isAllowed(srv.profile, tool) {
		return nil, fmt.Errorf("tool %q is not allowed in the readonly profile (playground is read-only)", tool)
	}
	return srv.dispatchInner(tool, args)
}
