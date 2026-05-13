package mcp

// obsTypeEnum lists all valid observation types (used in tool schemas).
var obsTypeEnum = []string{
	"decision", "pattern", "bugfix", "learning", "context",
	"antipattern", "task", "feature", "refactor", "discovery", "incident",
}

// tools returns the list of all MCP tools exposed by Vault.
func tools() []Tool {
	return []Tool{
		{
			Name: "vault_save",
			Description: "Save a knowledge observation to the Vault. Use after completing tasks, discovering patterns, fixing bugs, or making architectural decisions. " +
				"Pass dry_run=true to preview what would be stored after privacy filtering, without actually saving.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"title":   {Type: "string", Description: "Short descriptive title (max 100 chars)"},
					"content": {Type: "string", Description: "Full content of the observation"},
					"type": {Type: "string", Description: "Observation type",
						Enum: obsTypeEnum},
					"tags":                  {Type: "array", Description: "List of relevant tags"},
					"project":               {Type: "string", Description: "Project name (e.g., 'home-api')"},
					"team":                  {Type: "string", Description: "Team name (e.g., 'backend-seguros')"},
					"country":               {Type: "string", Description: "Country code: CL, PE, CO, or ALL"},
					"author":                {Type: "string", Description: "Author (developer username or AI agent name)"},
					"session_id":            {Type: "string", Description: "Active session ID (optional)"},
					"dry_run":               {Type: "boolean", Description: "Preview the filtered content without saving (default: false)"},
					"force":                 {Type: "boolean", Description: "Save even if a similar observation already exists (bypass semantic dedup). Default: false."},
					"topic_key":             {Type: "string", Description: "Stable key for upsert: if an observation with the same (project, topic_key) exists it is updated in-place instead of creating a new entry. Ideal for evolving knowledge tracked across sessions."},
					"working_dir":           {Type: "string", Description: "Filesystem path of the working directory at save time. Used for project auto-detection when project is omitted."},
					"project_choice_reason": {Type: "string", Description: "Reason the agent chose this project name (e.g. 'from git remote', 'user confirmed'). Stored for audit trail."},
				},
				Required: []string{"title", "content", "type"},
			},
		},
		{
			Name: "vault_update",
			Description: "Partially update an existing observation. Only the fields you provide are changed; omitted fields are left as-is. " +
				"Use to correct a title, extend content, change the type, or adjust tags without replacing the whole entry.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"id":      {Type: "string", Description: "Observation ULID to update"},
					"title":   {Type: "string", Description: "New title (omit to keep current)"},
					"content": {Type: "string", Description: "New content (omit to keep current)"},
					"type": {Type: "string", Description: "New observation type (omit to keep current)",
						Enum: obsTypeEnum},
					"tags": {Type: "array", Description: "Replacement tag list (omit to keep current)"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name: "vault_relate",
			Description: "Create a semantic relation between two observations. " +
				"Relations help the AI understand how knowledge evolves: supersedes (source replaces target), " +
				"conflicts_with (contradictory findings), related (topically linked), compatible (complementary), scoped (same topic, different context). " +
				"Re-calling with the same (source_id, target_id) pair updates the existing relation (upsert).",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"source_id": {Type: "string", Description: "ID of the source observation"},
					"target_id": {Type: "string", Description: "ID of the target observation"},
					"relation": {Type: "string", Description: "Semantic relation type",
						Enum: []string{"supersedes", "conflicts_with", "related", "compatible", "scoped"}},
					"reason": {Type: "string", Description: "Brief explanation of why this relation holds (optional)"},
					"author": {Type: "string", Description: "Who created the relation (optional)"},
				},
				Required: []string{"source_id", "target_id", "relation"},
			},
		},
		{
			Name: "vault_judge",
			Description: "Record a verdict for a pending judgment surfaced by vault_save's auto-scan. " +
				"vault_save returns judgment_ids alongside candidate observations whose meaning overlaps with the one you just stored; call vault_judge to resolve each pending row. " +
				"Pick the relation that captures how source relates to target: supersedes (source replaces target), conflicts_with (they contradict), related (topically linked), compatible (complementary), scoped (same topic, different context). " +
				"Use vault_compare instead when an external LLM already produced a verdict — that path skips the pending step.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"judgment_id": {Type: "string", Description: "ID of the pending judgment to resolve (from vault_save response)"},
					"relation": {Type: "string", Description: "Semantic verdict for this conflict pair",
						Enum: []string{"supersedes", "conflicts_with", "related", "compatible", "scoped"}},
					"reason":          {Type: "string", Description: "Short rationale (1-2 sentences). Stored verbatim for audit."},
					"evidence":        {Type: "string", Description: "Long-form evidence (e.g. the LLM's chain of thought). Optional."},
					"confidence":      {Type: "number", Description: "Confidence in [0, 1]. Default 1.0 when the verdict is unambiguous."},
					"marked_by_actor": {Type: "string", Description: "Who is recording the verdict", Enum: []string{"agent", "user", "admin"}},
					"marked_by_kind":  {Type: "string", Description: "How the verdict was reached", Enum: []string{"heuristic", "llm", "manual"}},
					"marked_by_model": {Type: "string", Description: "Model name when marked_by_kind=llm (e.g. claude-opus-4-7). Optional."},
				},
				Required: []string{"judgment_id", "relation"},
			},
		},
		{
			Name: "vault_compare",
			Description: "Persist an already-adjudicated comparison between two observations as a single judged relation. " +
				"Use this when an external LLM evaluated the pair end-to-end and you have a verdict ready — no pending step needed. " +
				"Idempotent on the (source_id, target_id) pair: a second call with the same source/target upserts the row.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"source_id":       {Type: "string", Description: "ID of the source observation"},
					"target_id":       {Type: "string", Description: "ID of the target observation"},
					"relation":        {Type: "string", Description: "Verdict verb", Enum: []string{"supersedes", "conflicts_with", "related", "compatible", "scoped"}},
					"reason":          {Type: "string", Description: "Short rationale"},
					"evidence":        {Type: "string", Description: "Long-form evidence (LLM reasoning)"},
					"confidence":      {Type: "number", Description: "Confidence in [0, 1]"},
					"marked_by_model": {Type: "string", Description: "LLM model name (e.g. claude-opus-4-7)"},
				},
				Required: []string{"source_id", "target_id", "relation"},
			},
		},
		{
			Name: "vault_current_project",
			Description: "Auto-detect the active project for the given working directory. " +
				"This tool never errors — even when nothing matches it returns a best-effort guess and tells you how it was reached. " +
				"Call it first thing in a session so subsequent vault_save / vault_search calls land in the right project bucket, " +
				"or whenever the agent switches between repositories on the same machine.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"working_dir": {Type: "string", Description: "Absolute path to inspect (optional — defaults to the vault process cwd)"},
				},
			},
		},
		{
			Name: "vault_suggest_topic_key",
			Description: "Propose a stable topic_key for an observation so future vault_save calls upsert into the same row instead of accumulating duplicates. " +
				"Pass the proposed title (required) plus optional type and project. The server returns a slug derived from the title plus any near-matches against the existing topic_keys in the same project. " +
				"Use the returned key in vault_save's topic_key argument.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"title":   {Type: "string", Description: "The observation title you intend to save"},
					"type":    {Type: "string", Description: "Observation type — when set the suggestion is namespaced (e.g. 'decision/...')", Enum: obsTypeEnum},
					"project": {Type: "string", Description: "Restrict the similar-key lookup to this project (recommended)"},
				},
				Required: []string{"title"},
			},
		},
		{
			Name: "vault_capture_passive",
			Description: "Bulk-save observations parsed from a freeform markdown block (session notes, tool output, retrospective summary). " +
				"Looks for sections like '## Key Learnings:' or '## Decisions:' and turns each bullet under them into a separate vault_save. " +
				"Returns a summary {saved, skipped, ids} so the agent can audit what was persisted.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"text":         {Type: "string", Description: "Markdown block to scan for capture-friendly sections"},
					"project":      {Type: "string", Description: "Project to attribute the captured observations to (required)"},
					"default_type": {Type: "string", Description: "Type assigned when a section's heading does not map to one (default: 'learning')", Enum: obsTypeEnum},
					"author":       {Type: "string", Description: "Author tag for the captured observations (optional)"},
				},
				Required: []string{"text", "project"},
			},
		},
		{
			Name: "vault_capture",
			Description: "Extract and persist multiple learnings from a block of freeform text in one call. " +
				"Pass the raw text (session notes, code review comments, retrospective output, chat transcript) and Korva will identify distinct observations and save them. " +
				"Returns a summary of saved vs skipped entries.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"text":       {Type: "string", Description: "Raw text to extract learnings from"},
					"project":    {Type: "string", Description: "Project to assign to extracted observations"},
					"session_id": {Type: "string", Description: "Active session ID (optional)"},
					"author":     {Type: "string", Description: "Author to tag on saved observations (optional)"},
				},
				Required: []string{"text"},
			},
		},
		{
			Name: "vault_merge_projects",
			Description: "Consolidate multiple project name variants into a single canonical name. " +
				"All observations, relations, sessions, and quality checkpoints under the source names are re-tagged to the canonical name. " +
				"Use when the same project was saved under inconsistent names (e.g. 'home-api', 'homeapi', 'home_api').",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"sources":   {Type: "array", Description: "List of project name variants to merge (e.g. [\"home-api\", \"homeapi\"])"},
					"canonical": {Type: "string", Description: "The single canonical project name to keep"},
				},
				Required: []string{"sources", "canonical"},
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
					"why":     {Type: "boolean", Description: "Attach a plain-language reasoning hint to each result explaining why it is relevant. Default: false"},
				},
			},
		},
		{
			Name: "vault_context",
			Description: "Retrieve recent observations for the current project. Call at the START of each session to restore context. " +
				"When prompt and file_paths are provided, the response also includes auto_skills — team skills automatically matched to the current task " +
				"without the developer needing to invoke them explicitly. " +
				"Use budget_tokens to cap the token cost; use delta=true on subsequent calls to receive only new observations since the last call.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project":       {Type: "string", Description: "Project name"},
					"limit":         {Type: "number", Description: "Max observations (default: 10)"},
					"budget_tokens": {Type: "number", Description: "Soft cap on response size in tokens (~4 chars/token). Content is truncated to fit. Default: 0 (unlimited)."},
					"delta":         {Type: "boolean", Description: "Return only observations added since the last vault_context call for this project. Saves tokens on repeated calls."},
					"prompt":        {Type: "string", Description: "Developer's current prompt or task description — used to auto-match team skills. Optional."},
					"file_paths":    {Type: "array", Description: "Currently relevant file paths (optional) — used to auto-match team skills."},
					"skill_limit":   {Type: "number", Description: "Max auto-matched skills to include (default: 5)"},
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
				Type:       "object",
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
					"score":       {Type: "number", Description: "Quality score 0–100. gate_passed=true requires score ≥ 70"},
					"findings":    {Type: "array", Description: "Array of {rule, status, notes} objects — one per criterion evaluated. rule = criterion ID (e.g. APP-001)"},
					"notes":       {Type: "string", Description: "General notes about the checkpoint (optional)"},
					"gate_passed": {Type: "boolean", Description: "true when ALL required criteria pass and score ≥ 70. Unlocks gated phase transitions."},
					"session_id":  {Type: "string", Description: "Active session ID (optional)"},
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
		{
			Name: "vault_export_lore",
			Description: "Export team private scrolls as structured notes for use in external tools. " +
				"Returns name, content, hash (for change detection), and updated_at for each scroll. " +
				"Pass since (RFC3339) to fetch only scrolls updated after that timestamp — useful for incremental sync. " +
				"Requires an active team session.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"since": {Type: "string", Description: "RFC3339 timestamp — return only scrolls updated after this time. Omit for full export."},
				},
			},
		},
		{
			Name: "vault_compress",
			Description: "Caveman-style output compression — strips filler, articles, and pleasantries from text while preserving technical accuracy. " +
				"Use to reduce token cost of long responses, summaries, or memory entries. " +
				"Modes: off, lite, full (default), ultra. Average ~65% reduction on prose; code blocks pass through untouched.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"text": {Type: "string", Description: "Text to compress"},
					"mode": {Type: "string", Description: "Compression intensity: off, lite, full, ultra (default: env KORVA_OUTPUT_MODE or full)"},
				},
				Required: []string{"text"},
			},
		},
		{
			Name: "vault_skill_match",
			Description: "Smart Skill Auto-Loader: returns the team skills most relevant to the current prompt + project. " +
				"Skills are matched by file patterns, keywords, project name, and tags. " +
				"Pass the developer's prompt and the active file paths to get scored matches with explanations. " +
				"This runs implicitly inside vault_context — only call directly if you need a custom prompt.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"prompt":     {Type: "string", Description: "Developer's prompt or task description"},
					"project":    {Type: "string", Description: "Active project name"},
					"file_paths": {Type: "array", Description: "Currently relevant file paths (optional)"},
					"limit":      {Type: "number", Description: "Max skills to return (default: 5)"},
				},
			},
		},
		{
			Name: "vault_pattern_mine",
			Description: "Scan recent observations for emerging implicit patterns — clusters of related work that haven't been explicitly documented as patterns. " +
				"Returns suggested pattern titles with example observation IDs. " +
				"Use periodically to surface undocumented conventions worth preserving.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"project":   {Type: "string", Description: "Project to scan (required)"},
					"max":       {Type: "number", Description: "Max patterns to return (default: 5)"},
					"min_count": {Type: "number", Description: "Min observations to qualify as a pattern (default: 2)"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name: "vault_code_health",
			Description: "Returns a composite code health score (0-100) and grade (A-F) per project, " +
				"based on QA checkpoint history, gate pass rate, and bug/pattern signal. " +
				"Use to quickly assess project quality trends before making architectural decisions.",
			InputSchema: Schema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name: "vault_hint",
			Description: "Ultra-lightweight search returning only IDs, types, and titles — no content. " +
				"Use when you need to discover what exists in the vault before deciding which entries to fully load with vault_get. " +
				"Costs ~10x fewer tokens than vault_search with full content.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"query":   {Type: "string", Description: "Full-text search query"},
					"project": {Type: "string", Description: "Filter by project name"},
					"type":    {Type: "string", Description: "Filter by observation type"},
					"limit":   {Type: "number", Description: "Max results (default: 20, max: 100)"},
				},
			},
		},
		// ── Harness Engineering ────────────────────────────────────────
		// Tools that let an agent drive a repo's Harness state machine
		// (feature_list.json) without shelling out. The `root` argument is
		// optional everywhere — falls back to $KORVA_HARNESS_ROOT and
		// finally to the server's CWD.
		{
			Name: "vault_harness_init",
			Description: "Bootstrap a Harness Engineering layout in a repo: AGENTS.md, init.sh, feature_list.json, docs/, progress/, optional .claude/agents/. " +
				"Idempotent: existing files are kept unless overwrite=true.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":           {Type: "string", Description: "Target directory (defaults to $KORVA_HARNESS_ROOT or CWD)"},
					"project":        {Type: "string", Description: "Project name (required)"},
					"description":    {Type: "string", Description: "Short blurb for AGENTS.md and feature_list.json"},
					"stack":          {Type: "string", Description: "Stack preset", Enum: []string{"go", "typescript", "python", "generic"}},
					"with_subagents": {Type: "boolean", Description: "Also install .claude/agents/{leader,implementer,reviewer}.md"},
					"overwrite":      {Type: "boolean", Description: "Replace existing harness files"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "vault_harness_status",
			Description: "Show backlog counts + currently in_progress feature + next pending id for the harness at `root`.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root": {Type: "string", Description: "Target directory (defaults to $KORVA_HARNESS_ROOT or CWD)"},
				},
			},
		},
		{
			Name:        "vault_harness_list",
			Description: "List every feature in the backlog with its status. Optional status filter narrows the response.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":   {Type: "string", Description: "Target directory (defaults to $KORVA_HARNESS_ROOT or CWD)"},
					"status": {Type: "string", Description: "Filter by status", Enum: []string{"pending", "in_progress", "done", "blocked"}},
				},
			},
		},
		{
			Name:        "vault_harness_next",
			Description: "Return the lowest-id pending feature with its acceptance criteria, or null when the backlog is clear.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root": {Type: "string", Description: "Target directory (defaults to $KORVA_HARNESS_ROOT or CWD)"},
				},
			},
		},
		{
			Name:        "vault_harness_start",
			Description: "Move a feature to in_progress. Fails if another feature is already in_progress (one-at-a-time rule).",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":  {Type: "string", Description: "Target directory"},
					"id":    {Type: "number", Description: "Feature id"},
					"agent": {Type: "string", Description: "Override the recorded owner (defaults to the MCP session email or 'mcp')"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_harness_done",
			Description: "Mark a feature as done. Requires the feature to be in_progress.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":  {Type: "string", Description: "Target directory"},
					"id":    {Type: "number", Description: "Feature id"},
					"agent": {Type: "string", Description: "Override the recorded owner"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_harness_block",
			Description: "Mark a feature as blocked. The reviewer / leader is expected to write the blocker into progress/current.md.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":  {Type: "string", Description: "Target directory"},
					"id":    {Type: "number", Description: "Feature id"},
					"agent": {Type: "string", Description: "Override the recorded owner"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_harness_reopen",
			Description: "Return a blocked or in_progress feature to pending so another session can pick it up.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":  {Type: "string", Description: "Target directory"},
					"id":    {Type: "number", Description: "Feature id"},
					"agent": {Type: "string", Description: "Override the recorded owner"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "vault_harness_add",
			Description: "Append a new feature to the backlog with a pending status.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"root":        {Type: "string", Description: "Target directory"},
					"name":        {Type: "string", Description: "Short slug (required)"},
					"title":       {Type: "string", Description: "Human-readable title (defaults to name)"},
					"description": {Type: "string", Description: "Longer description"},
					"acceptance":  {Type: "array", Description: "Acceptance criteria — one bullet per item"},
				},
				Required: []string{"name"},
			},
		},
	}
}
