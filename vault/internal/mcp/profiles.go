package mcp

import "os"

// Profile controls which MCP tools are visible and callable by the connected AI client.
//
// Select via KORVA_MCP_PROFILE environment variable:
//   - agent    (default) — all workflow tools; excludes destructive admin operations
//   - readonly            — search and read tools only; safe for untrusted clients
//   - admin              — all tools including vault_delete and vault_bulk_save
//
// Choosing a narrower profile reduces the tool list surfaced to the LLM, cutting
// tokens consumed per session and preventing accidental destructive calls.
type Profile string

const (
	ProfileAgent    Profile = "agent"
	ProfileReadonly Profile = "readonly"
	ProfileAdmin    Profile = "admin"
)

// profileTools maps each profile to the set of tool names it exposes.
var profileTools = map[Profile]map[string]bool{
	ProfileAgent: {
		"vault_save":          true,
		"vault_search":        true,
		"vault_context":       true,
		"vault_timeline":      true,
		"vault_get":           true,
		"vault_hint":          true,
		"vault_code_health":   true,
		"vault_pattern_mine":  true,
		"vault_skill_match":   true,
		"vault_compress":      true,
		"vault_session_start": true,
		"vault_session_end":   true,
		"vault_summary":       true,
		"vault_save_prompt":   true,
		"vault_sdd_phase":     true,
		"vault_qa_checklist":  true,
		"vault_qa_checkpoint": true,
		"vault_team_context":  true,
		"vault_export_lore":   true,
	},
	ProfileReadonly: {
		"vault_search":        true,
		"vault_context":       true,
		"vault_timeline":      true,
		"vault_get":           true,
		"vault_hint":          true,
		"vault_code_health":   true,
		"vault_pattern_mine":  true,
		"vault_skill_match":   true,
		"vault_compress":      true,
		"vault_summary":       true,
		"vault_stats":         true,
		"vault_export_lore":   true,
	},
	ProfileAdmin: {
		"vault_save":          true,
		"vault_search":        true,
		"vault_context":       true,
		"vault_timeline":      true,
		"vault_get":           true,
		"vault_hint":          true,
		"vault_code_health":   true,
		"vault_pattern_mine":  true,
		"vault_skill_match":   true,
		"vault_compress":      true,
		"vault_session_start": true,
		"vault_session_end":   true,
		"vault_summary":       true,
		"vault_save_prompt":   true,
		"vault_stats":         true,
		"vault_delete":        true,
		"vault_bulk_save":     true,
		"vault_query":         true,
		"vault_sdd_phase":     true,
		"vault_qa_checklist":  true,
		"vault_qa_checkpoint": true,
		"vault_team_context":  true,
		"vault_export_lore":   true,
	},
}

// activeProfile reads KORVA_MCP_PROFILE and returns the matching Profile.
// Falls back to ProfileAgent when the variable is absent or unrecognised.
func activeProfile() Profile {
	switch os.Getenv("KORVA_MCP_PROFILE") {
	case string(ProfileReadonly):
		return ProfileReadonly
	case string(ProfileAdmin):
		return ProfileAdmin
	default:
		return ProfileAgent
	}
}

// toolsForProfile returns the subset of all registered tools allowed by p.
func toolsForProfile(p Profile) []Tool {
	allowed, ok := profileTools[p]
	if !ok {
		allowed = profileTools[ProfileAgent]
	}
	all := tools()
	result := make([]Tool, 0, len(all))
	for _, t := range all {
		if allowed[t.Name] {
			result = append(result, t)
		}
	}
	return result
}

// isAllowed reports whether the given tool name is permitted under profile p.
func isAllowed(p Profile, toolName string) bool {
	allowed, ok := profileTools[p]
	if !ok {
		allowed = profileTools[ProfileAgent]
	}
	return allowed[toolName]
}
