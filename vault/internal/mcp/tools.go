package mcp

// obsTypeEnum lists all valid observation types (used in tool schemas).
var obsTypeEnum = []string{
	"decision", "pattern", "bugfix", "learning", "context",
	"antipattern", "task", "feature", "refactor", "discovery",
}

// tools returns the list of all MCP tools exposed by Vault.
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
						Enum: obsTypeEnum},
					"tags":       {Type: "array", Description: "List of relevant tags"},
					"project":    {Type: "string", Description: "Project name (e.g., 'home-api')"},
					"team":       {Type: "string", Description: "Team name (e.g., 'backend-seguros')"},
					"country":    {Type: "string", Description: "Country code: CL, PE, CO, or ALL"},
					"author":     {Type: "string", Description: "Author (developer username or AI agent name)"},
					"session_id": {Type: "string", Description: "Active session ID (optional)"},
				},
				Required: []string{"title", "content", "type"},
			},
		},
		{
			Name: "vault_search",
			Description: "Search the Vault using full-text search. Use BEFORE proposing solutions to check for prior decisions and patterns. " +
				"Pass compact=true for a lightweight index (IDs + titles only) that costs far fewer tokens — " +
				"then call vault_get for the full text of specific entries.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"query":   {Type: "string", Description: "Full-text search query"},
					"project": {Type: "string", Description: "Filter by project name"},
					"team":    {Type: "string", Description: "Filter by team name"},
					"country": {Type: "string", Description: "Filter by country code"},
					"type":    {Type: "string", Description: "Filter by observation type"},
					"limit":   {Type: "number", Description: "Max results (default: 20)"},
					"compact": {Type: "boolean", Description: "Return compact index (id + type + title + project only). Default: false"},
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
		{
			Name:        "vault_delete",
			Description: "Delete a specific observation from the Vault by its ID. Use to remove incorrect, duplicate, or outdated entries.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"id": {Type: "string", Description: "Observation ULID to delete"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_bulk_save",
			Description: "Save multiple knowledge observations in a single call. Ideal for session-end dumps where an AI agent has 3-10 learnings to persist. Returns the list of created IDs in order. Max 50 items per call.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"observations": {
						Type:        "array",
						Description: "Array of observation objects. Each must have title, content, and type. Optional fields: project, team, country, author, tags, session_id.",
					},
				},
				Required: []string{"observations"},
			},
		},
		{
			Name: "vault_query",
			Description: "Structured query of the Vault with type, date-range, and project filters — no full-text search. " +
				"Ideal for 'show me all decisions from the last month' or 'list every antipattern in project X'.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Filter by project name"},
					"team":    {Type: "string", Description: "Filter by team name"},
					"type":    {Type: "string", Description: "Filter by observation type", Enum: obsTypeEnum},
					"since":   {Type: "string", Description: "Return observations created at or after this RFC3339 timestamp"},
					"until":   {Type: "string", Description: "Return observations created at or before this RFC3339 timestamp"},
					"limit":   {Type: "number", Description: "Maximum results (default: 20, max: 100)"},
				},
			},
		},
		{
			Name: "vault_sdd_phase",
			Description: "Read or update the Spec-Driven Development (SDD) phase for a project. " +
				"SDD phases: explore → propose → spec → design → tasks → apply → verify → archive → onboard. " +
				"Call with only 'project' to GET the current phase. Include 'phase' to SET a new phase. " +
				"The active phase is automatically included in vault_context responses so the AI always knows where development stands.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project": {Type: "string", Description: "Project name (required)"},
					"phase": {Type: "string", Description: "New phase to set (omit to just read)",
						Enum: []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive", "onboard"}},
				},
				Required: []string{"project"},
			},
		},
		{
			Name: "vault_qa_checklist",
			Description: "Get the quality checklist for a specific SDD phase and optional language. " +
				"Returns criteria with IDs, categories, severity levels, and guidance. " +
				"Call before starting an implementation or review to know exactly what to check. " +
				"Use the criterion IDs (e.g. APP-001, GO-APP-001) as 'rule' values in vault_qa_checkpoint findings.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"phase": {Type: "string", Description: "SDD phase to get checklist for",
						Enum: []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive", "onboard"}},
					"language": {Type: "string",
						Description: "Programming language for additional language-specific criteria: go, typescript, react. Omit for general criteria only."},
				},
				Required: []string{"phase"},
			},
		},
		{
			Name: "vault_qa_checkpoint",
			Description: "Record a QA assessment result for the current SDD phase. " +
				"The AI performs the assessment against vault_qa_checklist criteria and calls this to persist findings. " +
				"For gated transitions (apply→verify, verify→archive) a checkpoint with gate_passed=true is required before vault_sdd_phase will allow the advance. " +
				"Include one finding per criterion ID evaluated.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project":  {Type: "string", Description: "Project name (required)"},
					"phase":    {Type: "string", Description: "SDD phase that was assessed (required)"},
					"language": {Type: "string", Description: "Language evaluated: go, typescript, react (optional)"},
					"status": {Type: "string", Description: "Overall assessment status",
						Enum: []string{"pass", "fail", "partial", "skip"}},
					"score":      {Type: "number", Description: "Quality score 0–100. gate_passed=true requires score ≥ 70"},
					"findings":   {Type: "array", Description: "Array of {rule, status, notes} objects — one per criterion evaluated. rule = criterion ID (e.g. APP-001)"},
					"notes":      {Type: "string", Description: "General notes about the checkpoint (optional)"},
					"gate_passed": {Type: "boolean", Description: "true when ALL required criteria pass and score ≥ 70. Unlocks gated phase transitions."},
					"session_id": {Type: "string", Description: "Active session ID (optional)"},
				},
				Required: []string{"project", "phase", "status", "score"},
			},
		},
		{
			Name: "vault_team_context",
			Description: "Fetch your team's custom skills and private scrolls. " +
				"Returns team architecture guides, custom AI instructions, and internal knowledge docs. " +
				"Requires a session token passed as session_token in initialize params. " +
				"Call at the start of work on a team project to load all team-specific context.",
			InputSchema: Schema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
	}
}
