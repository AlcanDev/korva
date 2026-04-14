package mcp

// tools returns the list of all 10 MCP tools exposed by Vault.
func tools() []Tool {
	return []Tool{
		{
			Name:        "vault_save",
			Description: "Save a knowledge observation to the Vault. Use after completing tasks, discovering patterns, fixing bugs, or making architectural decisions.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"title":   {Type: "string", Description: "Short descriptive title (max 100 chars)"},
					"content": {Type: "string", Description: "Full content of the observation"},
					"type": {Type: "string", Description: "Observation type",
						Enum: []string{"decision", "pattern", "bugfix", "learning", "context", "antipattern", "task"}},
					"tags":      {Type: "array", Description: "List of relevant tags"},
					"project":   {Type: "string", Description: "Project name (e.g., 'home-api')"},
					"team":      {Type: "string", Description: "Team name (e.g., 'backend-seguros')"},
					"country":   {Type: "string", Description: "Country code: CL, PE, CO, or ALL"},
					"author":    {Type: "string", Description: "Author (developer username or AI agent name)"},
					"session_id": {Type: "string", Description: "Active session ID (optional)"},
				},
				Required: []string{"title", "content", "type"},
			},
		},
		{
			Name:        "vault_search",
			Description: "Search the Vault using full-text search. Use BEFORE proposing solutions to check for prior decisions and patterns.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"query":   {Type: "string", Description: "Full-text search query"},
					"project": {Type: "string", Description: "Filter by project name"},
					"team":    {Type: "string", Description: "Filter by team name"},
					"country": {Type: "string", Description: "Filter by country code"},
					"type":    {Type: "string", Description: "Filter by observation type"},
					"limit":   {Type: "number", Description: "Max results (default: 20)"},
				},
			},
		},
		{
			Name:        "vault_context",
			Description: "Retrieve recent observations for the current project. Call at the START of each session to restore context.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Project name"},
					"limit":   {Type: "number", Description: "Max observations (default: 10)"},
				},
			},
		},
		{
			Name:        "vault_timeline",
			Description: "Get observations within a date range to understand recent project history.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Project name"},
					"from":    {Type: "string", Description: "Start date (RFC3339, default: 7 days ago)"},
					"to":      {Type: "string", Description: "End date (RFC3339, default: now)"},
				},
			},
		},
		{
			Name:        "vault_get",
			Description: "Retrieve a specific observation by its ID.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"id": {Type: "string", Description: "Observation ULID"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_session_start",
			Description: "Start a new work session. Call when beginning a development task to enable session-scoped observations.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Project name"},
					"team":    {Type: "string", Description: "Team name"},
					"country": {Type: "string", Description: "Country code"},
					"agent":   {Type: "string", Description: "AI agent: copilot, claude, cursor"},
					"goal":    {Type: "string", Description: "Brief description of the session goal"},
				},
			},
		},
		{
			Name:        "vault_session_end",
			Description: "End the current session with a summary of what was accomplished.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from vault_session_start"},
					"summary":    {Type: "string", Description: "Summary of what was done and learned"},
				},
				Required: []string{"session_id"},
			},
		},
		{
			Name:        "vault_summary",
			Description: "Get a high-level summary of a project's stored knowledge: recent decisions, patterns, and statistics.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Project name"},
				},
			},
		},
		{
			Name:        "vault_save_prompt",
			Description: "Save a reusable prompt template for future AI sessions.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"name":    {Type: "string", Description: "Unique prompt name"},
					"content": {Type: "string", Description: "Prompt content"},
					"tags":    {Type: "array", Description: "Tags for categorization"},
				},
				Required: []string{"name", "content"},
			},
		},
		{
			Name:        "vault_stats",
			Description: "Get global Vault statistics: total observations, sessions, breakdown by type/project/team.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{},
			},
		},
	}
}
