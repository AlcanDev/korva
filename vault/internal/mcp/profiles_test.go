package mcp

import (
	"os"
	"testing"
)

func TestActiveProfile_Default(t *testing.T) {
	os.Unsetenv("KORVA_MCP_PROFILE")
	if got := activeProfile(); got != ProfileAgent {
		t.Errorf("expected agent, got %q", got)
	}
}

func TestActiveProfile_Readonly(t *testing.T) {
	t.Setenv("KORVA_MCP_PROFILE", "readonly")
	if got := activeProfile(); got != ProfileReadonly {
		t.Errorf("expected readonly, got %q", got)
	}
}

func TestActiveProfile_Admin(t *testing.T) {
	t.Setenv("KORVA_MCP_PROFILE", "admin")
	if got := activeProfile(); got != ProfileAdmin {
		t.Errorf("expected admin, got %q", got)
	}
}

func TestActiveProfile_Unknown_FallsBackToAgent(t *testing.T) {
	t.Setenv("KORVA_MCP_PROFILE", "superadmin")
	if got := activeProfile(); got != ProfileAgent {
		t.Errorf("unknown profile should fall back to agent, got %q", got)
	}
}

func TestToolsForProfile_AgentExcludesDestructive(t *testing.T) {
	tools := toolsForProfile(ProfileAgent)
	for _, tool := range tools {
		if tool.Name == "vault_delete" || tool.Name == "vault_bulk_save" || tool.Name == "vault_query" {
			t.Errorf("agent profile should not include %q", tool.Name)
		}
	}
}

func TestToolsForProfile_ReadonlyOnlyReads(t *testing.T) {
	tools := toolsForProfile(ProfileReadonly)
	for _, tool := range tools {
		if tool.Name == "vault_save" || tool.Name == "vault_delete" || tool.Name == "vault_session_start" {
			t.Errorf("readonly profile should not include %q", tool.Name)
		}
	}
	// readonly must have vault_search
	found := false
	for _, tool := range tools {
		if tool.Name == "vault_search" {
			found = true
		}
	}
	if !found {
		t.Error("readonly profile must include vault_search")
	}
}

func TestToolsForProfile_AdminHasAll(t *testing.T) {
	adminTools := toolsForProfile(ProfileAdmin)
	all := tools()
	if len(adminTools) != len(all) {
		t.Errorf("admin profile should expose all %d tools, got %d", len(all), len(adminTools))
	}
}

func TestToolsForProfile_ExportLoreInAllProfiles(t *testing.T) {
	for _, p := range []Profile{ProfileAgent, ProfileReadonly, ProfileAdmin} {
		found := false
		for _, tool := range toolsForProfile(p) {
			if tool.Name == "vault_export_lore" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("profile %q should include vault_export_lore", p)
		}
	}
}

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		profile  Profile
		tool     string
		expected bool
	}{
		{ProfileAgent, "vault_save", true},
		{ProfileAgent, "vault_delete", false},
		{ProfileAgent, "vault_bulk_save", false},
		{ProfileReadonly, "vault_search", true},
		{ProfileReadonly, "vault_save", false},
		{ProfileAdmin, "vault_delete", true},
		{ProfileAdmin, "vault_bulk_save", true},
		{"unknown", "vault_save", true}, // unknown → agent fallback
		{"unknown", "vault_delete", false},
	}
	for _, tt := range tests {
		got := isAllowed(tt.profile, tt.tool)
		if got != tt.expected {
			t.Errorf("isAllowed(%q, %q) = %v, want %v", tt.profile, tt.tool, got, tt.expected)
		}
	}
}
