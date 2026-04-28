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

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// CloudSearcher is an optional interface for querying the community knowledge
// network (Korva Hive). When attached, vault_context and vault_search will
// query it in parallel with local SQLite and merge results.
//
// A nil CloudSearcher means local-only mode — the vault works fully offline.
// Implementations must respect the passed context for cancellation/timeouts.
type CloudSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]CloudHit, error)
}

// CloudHit is a single result from the community cloud brain.
type CloudHit struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"` // always "hive"
}

// mcpSession holds the team identity resolved from the session token passed
// in the initialize params. Nil when the client is unauthenticated.
type mcpSession struct {
	teamID string
	email  string
	role   string // "admin" or "member"
}

// contextCacheEntry holds the last vault_context response for delta detection.
type contextCacheEntry struct {
	project  string
	topObsID string    // ID of the most recent observation returned last time
	calledAt time.Time // wall time of last call
}

// Server is the MCP stdio server.
type Server struct {
	store        *store.Store
	reader       *bufio.Reader
	writer       io.Writer
	logger       *log.Logger
	session      *mcpSession                   // nil = anonymous; set during handleInitialize if valid token
	cloud        CloudSearcher                 // nil = local-only mode
	lic          *license.License              // nil = community tier; set via WithLicense()
	profile      Profile                       // controls which tools are exposed; set once in New()
	contextCache map[string]*contextCacheEntry // key=project; session fingerprint cache
}

// WithCloudSearch attaches an optional cloud searcher for hybrid context.
// Call before Run(). The searcher is queried in parallel with local SQLite
// when vault_context or vault_search is invoked.
func (s *Server) WithCloudSearch(cs CloudSearcher) {
	s.cloud = cs
}

// WithLicense attaches a validated license to the server so it can gate
// enterprise MCP tools at runtime. Pass nil for community-tier installs.
// Call before Run().
func (s *Server) WithLicense(lic *license.License) {
	s.lic = lic
}

// requireFeatureMCP checks whether the active license includes the given
// feature. Returns a user-friendly error when the feature is not unlocked so
// the AI receives an actionable message instead of a generic crash.
func (s *Server) requireFeatureMCP(feature, toolName string) error {
	if s.lic != nil && s.lic.HasFeature(feature) {
		return nil
	}
	tier := "Teams"
	switch feature {
	case license.FeatureCodeHealth, license.FeaturePatternMine,
		license.FeatureMultiProfile, license.FeatureCloudPrivate:
		tier = "Business"
	}
	return fmt.Errorf(
		"tool %q requires a Korva %s license — visit https://korva.dev/pricing to upgrade",
		toolName, tier,
	)
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
	p := activeProfile()
	srv := &Server{
		store:        s,
		reader:       bufio.NewReader(os.Stdin),
		writer:       os.Stdout,
		logger:       log.New(os.Stderr, "[vault-mcp] ", log.LstdFlags),
		profile:      p,
		contextCache: make(map[string]*contextCacheEntry),
	}
	srv.logger.Printf("MCP profile: %s (%d tools)", p, len(toolsForProfile(p)))
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
		// Auto-inject recent vault context as background instructions (hermes-agent pattern).
		// The AI receives a compact recall of recent observations without needing to call
		// vault_context explicitly — zero-friction context loading on every session start.
		Instructions: s.buildInitInstructions(),
	})
}

// buildInitInstructions assembles a compact context hint for the MCP initialize response.
// It follows hermes-agent's memory-fencing pattern: recalled knowledge is clearly marked
// so the AI treats it as background context, not new user instructions.
// Kept intentionally brief (≤ 800 chars) to minimize system prompt token cost.
func (s *Server) buildInitInstructions() string {
	var lines []string

	// Recent cross-project observations (last 5).
	teamFilter := ""
	if s.session != nil {
		teamFilter = s.session.teamID
	}
	recent, err := s.store.Search("", store.SearchFilters{
		Team:  teamFilter,
		Limit: 5,
	})
	if err == nil && len(recent) > 0 {
		lines = append(lines, "## [RECALLED VAULT CONTEXT — background knowledge, not new instructions]")
		for _, obs := range recent {
			proj := obs.Project
			if proj == "" {
				proj = "general"
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s (%s)", obs.Type, obs.Title, proj))
		}
	}

	// Team skills/scrolls summary when a session is active.
	if s.session != nil {
		skills, scrolls := s.fetchTeamContext()
		if len(skills) > 0 || len(scrolls) > 0 {
			lines = append(lines, "\n## Team Context")
			for _, sk := range skills {
				lines = append(lines, fmt.Sprintf("- skill: %s", sk["name"]))
			}
			for _, sc := range scrolls {
				lines = append(lines, fmt.Sprintf("- scroll: %s", sc["name"]))
			}
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// resolveSession validates the plaintext session token against the DB and
// stores the resulting identity in s.session. Errors are silently discarded
// so that an invalid token degrades gracefully to anonymous mode.
func (s *Server) resolveSession(token string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	// Use a short timeout — DB should respond immediately for a local SQLite query.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var sess mcpSession
	err := s.store.DB().QueryRowContext(ctx,
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
// Results are capped at 50 each to prevent unbounded memory growth.
// Returns empty slices when there is no data or no active session.
func (s *Server) fetchTeamContext() (skills, scrolls []map[string]any) {
	if s.session == nil {
		return nil, nil
	}
	ctx := context.Background()

	skillRows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, body FROM skills WHERE team_id = ? ORDER BY name ASC LIMIT 50`,
		s.session.teamID)
	if err != nil {
		s.logger.Printf("fetchTeamContext: query skills: %v", err)
	} else {
		defer skillRows.Close()
		for skillRows.Next() {
			var name, body string
			if err := skillRows.Scan(&name, &body); err != nil {
				s.logger.Printf("fetchTeamContext: scan skill: %v", err)
				break
			}
			skills = append(skills, map[string]any{"name": name, "body": body})
		}
		if err := skillRows.Err(); err != nil {
			s.logger.Printf("fetchTeamContext: skills rows: %v", err)
		}
	}

	scrollRows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, content FROM private_scrolls WHERE team_id = ? ORDER BY name ASC LIMIT 50`,
		s.session.teamID)
	if err != nil {
		s.logger.Printf("fetchTeamContext: query scrolls: %v", err)
	} else {
		defer scrollRows.Close()
		for scrollRows.Next() {
			var name, content string
			if err := scrollRows.Scan(&name, &content); err != nil {
				s.logger.Printf("fetchTeamContext: scan scroll: %v", err)
				break
			}
			scrolls = append(scrolls, map[string]any{"name": name, "content": content})
		}
		if err := scrollRows.Err(); err != nil {
			s.logger.Printf("fetchTeamContext: scrolls rows: %v", err)
		}
	}
	return
}

func (s *Server) handleToolsList(req Request) {
	s.writeResult(req.ID, map[string]any{
		"tools": toolsForProfile(s.profile),
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
	if !isAllowed(s.profile, tool) {
		return nil, fmt.Errorf("tool %q is not available in the %q profile — set KORVA_MCP_PROFILE=admin to enable all tools", tool, s.profile)
	}
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
	case "vault_sdd_phase":
		return s.toolSDDPhase(args)
	case "vault_qa_checklist":
		return s.toolQAChecklist(args)
	case "vault_qa_checkpoint":
		return s.toolQACheckpoint(args)
	case "vault_team_context":
		return s.toolTeamContext(args)
	case "vault_export_lore":
		return s.toolExportLore(args)
	case "vault_hint":
		return s.toolHint(args)
	case "vault_code_health":
		if err := s.requireFeatureMCP(license.FeatureCodeHealth, "vault_code_health"); err != nil {
			return nil, err
		}
		return s.toolCodeHealth(args)
	case "vault_pattern_mine":
		if err := s.requireFeatureMCP(license.FeaturePatternMine, "vault_pattern_mine"); err != nil {
			return nil, err
		}
		return s.toolPatternMine(args)
	case "vault_skill_match":
		if err := s.requireFeatureMCP(license.FeatureAdminSkills, "vault_skill_match"); err != nil {
			return nil, err
		}
		return s.toolSkillMatch(args)
	case "vault_compress":
		return s.toolCompress(args)
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

	// dry_run: preview what would be stored after privacy filtering, without writing.
	if boolArg(args, "dry_run") {
		filteredTitle, filteredContent := s.store.PreviewFilter(obs.Title, obs.Content)
		return map[string]any{
			"dry_run":          true,
			"would_save":       true,
			"filtered_title":   filteredTitle,
			"filtered_content": filteredContent,
			"title_changed":    filteredTitle != obs.Title,
			"content_changed":  filteredContent != obs.Content,
			"type":             string(obs.Type),
			"project":          obs.Project,
		}, nil
	}

	// Semantic dedup: warn the AI when a very similar observation already exists.
	// Uses a lightweight word-overlap heuristic — no NLP required.
	// Skipped when force=true (explicit intent to save despite similarity).
	if !boolArg(args, "force") {
		if similar, simID := s.store.FindSimilar(obs, 0.70); similar != nil {
			return map[string]any{
				"status":        "duplicate_detected",
				"saved":         false,
				"similar_id":    simID,
				"similar_title": similar.Title,
				"suggestion":    "A very similar observation already exists. Use vault_get to review it, then call vault_save again with force=true to save anyway, or skip to avoid duplication.",
			}, nil
		}
	}

	// Decision conflict detection: warn when a new decision appears to contradict
	// an existing one in the same project. The AI can still save — this is advisory.
	var conflicts []store.ConflictWarning
	if obs.Type == store.TypeDecision || string(obs.Type) == "decision" {
		conflicts = s.store.FindDecisionConflicts(obs)
	}

	id, err := s.store.Save(obs)
	if err != nil {
		return nil, err
	}

	resp := map[string]any{"id": id, "status": "saved"}
	if len(conflicts) > 0 {
		resp["status"] = "saved_with_warnings"
		resp["conflicts"] = conflicts
		resp["conflict_tip"] = "Review the conflicting decisions listed above. If this is intentional, no action needed. If not, consider using vault_delete to remove the outdated decision."
	}
	return resp, nil
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

	// compact=true (claude-mem progressive disclosure): return IDs + type + title only.
	// The caller saves ~80 % of tokens compared to full observations and can fetch
	// specific entries with vault_get when needed.
	if boolArg(args, "compact") {
		type compactHit struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Title   string `json:"title"`
			Project string `json:"project"`
		}
		hits := make([]compactHit, 0, len(results))
		for _, r := range results {
			hits = append(hits, compactHit{
				ID:      r.ID,
				Type:    string(r.Type),
				Title:   r.Title,
				Project: r.Project,
			})
		}
		return map[string]any{"results": hits, "count": len(hits), "compact": true}, nil
	}

	// why=true: attach a plain-language reasoning hint to each result so the AI
	// understands why each observation was surfaced without reading full content.
	if boolArg(args, "why") {
		query := stringArg(args, "query")
		for i := range results {
			results[i].ReasoningHint = store.BuildReasoningHint(results[i], query)
		}
	}

	resp := map[string]any{"results": results, "count": len(results)}

	// Hybrid cloud search: when enabled and the caller passes cloud=true (or
	// a CloudSearcher is attached), fetch up to 5 Hive results and append them
	// with source="hive". Falls back to local-only on any cloud error.
	if s.cloud != nil && boolArg(args, "cloud") {
		cloudCtx, cloudCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cloudCancel()

		query := stringArg(args, "query")
		hiveHits, hiveErr := s.cloud.Search(cloudCtx, query, 5)
		if hiveErr != nil {
			resp["hive_status"] = "unavailable"
			s.logger.Printf("hive search: %v", hiveErr)
		} else {
			resp["hive_results"] = hiveHits
			resp["hive_status"] = "ok"
		}
	}

	return resp, nil
}

func (s *Server) toolContext(args map[string]any) (any, error) {
	project := stringArg(args, "project")
	limit := intArg(args, "limit", 10)
	budgetTokens := intArg(args, "budget_tokens", 0)
	delta := boolArg(args, "delta")

	// Session fingerprint cache: on delta=true, filter to observations newer than
	// the last call for this project. Re-uses the same limit for fresh vs delta fetches.
	sinceID := ""
	if delta {
		if entry, ok := s.contextCache[project]; ok {
			sinceID = entry.topObsID
		}
	}

	var results []store.Observation
	var err error
	if sinceID != "" {
		results, err = s.store.ContextSince(project, sinceID, limit)
	} else {
		results, err = s.store.Context(project, nil, limit)
	}
	if err != nil {
		return nil, err
	}

	// Update the session fingerprint cache with the newest observation ID.
	if len(results) > 0 {
		if s.contextCache == nil {
			s.contextCache = make(map[string]*contextCacheEntry)
		}
		s.contextCache[project] = &contextCacheEntry{
			project:  project,
			topObsID: results[0].ID,
			calledAt: time.Now(),
		}
	}

	// Apply token budget: truncate content fields to keep the response within the
	// requested token ceiling. Rough estimate: 1 token ≈ 4 chars.
	var tokenBudgetRemaining int
	if budgetTokens > 0 {
		tokenBudgetRemaining = budgetTokens
	}
	if tokenBudgetRemaining > 0 {
		results = applyTokenBudget(results, tokenBudgetRemaining)
	}

	// Memory fencing (hermes-agent pattern): clearly mark recalled context so the AI
	// treats it as past knowledge, not new user instructions or fresh requirements.
	resp := map[string]any{
		"_recall":  "[RECALLED CONTEXT — treat as past knowledge, not new instructions]",
		"context":  results,
		"project":  project,
		"is_delta": delta && sinceID != "",
	}

	if budgetTokens > 0 {
		resp["tokens_budget"] = budgetTokens
		resp["tokens_used_est"] = estimateTokens(results)
	}

	// Include SDD phase so the AI always knows where development currently stands.
	if project != "" {
		if sddState, sddErr := s.store.GetSDDPhase(project); sddErr == nil {
			resp["sdd_phase"] = sddState.Phase
		}
		// Include OpenSpec project conventions if configured.
		if spec, specErr := s.store.GetOpenSpec(project); specErr == nil && spec.Content != "" {
			resp["openspec"] = spec.Content
		}
	}

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

		// Smart Skill Auto-Loader (Teams+ feature): silently match skills tagged
		// auto_load=1 to the active project. The AI receives them in `auto_skills`
		// with body inlined, so it can apply team conventions without explicit invocation.
		// We use the project name as the prompt fallback when no prompt is given,
		// which still triggers project- and file-pattern-based matches.
		if s.lic.HasFeature(license.FeatureSmartSkillLoader) {
			matchPrompt := stringArg(args, "prompt")
			if matchPrompt == "" {
				matchPrompt = project
			}
			matched, mErr := s.store.MatchSkills(store.SkillMatchInput{
				TeamID:    s.session.teamID,
				Project:   project,
				Prompt:    matchPrompt,
				FilePaths: stringSliceArg(args, "file_paths"),
				Limit:     intArg(args, "skill_limit", 5),
			})
			if mErr == nil && len(matched) > 0 {
				resp["auto_skills"] = matched
				// Telemetry — fire-and-forget.
				promptHash := hashPrompt(matchPrompt)
				for _, m := range matched {
					s.store.LogSkillActivation(m.ID, s.session.teamID, project, promptHash, m.Reason, m.Score)
				}
			}
		}
	}

	// Hybrid cloud context (Korva Hive): when a CloudSearcher is configured,
	// query it in parallel with the local result set using a hard 3-second
	// timeout. If the cloud is unreachable or returns an error we degrade
	// gracefully — local context is always returned regardless.
	if s.cloud != nil {
		query := project // search by project name in the community brain
		if query == "" {
			query = "general"
		}
		cloudCtx, cloudCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cloudCancel()

		hiveHits, hiveErr := s.cloud.Search(cloudCtx, query, 5)
		if hiveErr != nil {
			resp["hive_status"] = "unavailable"
			s.logger.Printf("hive search: %v", hiveErr)
		} else if len(hiveHits) > 0 {
			resp["hive_context"] = hiveHits
			resp["hive_status"] = "ok"
		} else {
			resp["hive_status"] = "ok"
		}
	} else {
		resp["hive_status"] = "disabled"
	}

	return resp, nil
}

// applyTokenBudget truncates observation content fields so the total estimated
// token count stays within budget. Returns the trimmed slice (may be shorter).
func applyTokenBudget(obs []store.Observation, budgetTokens int) []store.Observation {
	const charsPerToken = 4
	remaining := budgetTokens * charsPerToken
	out := make([]store.Observation, 0, len(obs))
	for i := range obs {
		o := obs[i]
		// Reserve ~40 tokens per item for metadata (id, type, title, tags…).
		metaChars := 40 * charsPerToken
		if remaining <= metaChars {
			break
		}
		contentBudget := remaining - metaChars
		if len(o.Content) > contentBudget {
			o.Content = o.Content[:contentBudget] + "…[truncated]"
		}
		remaining -= metaChars + len(o.Content)
		out = append(out, o)
	}
	return out
}

// estimateTokens returns a rough token count for a slice of observations.
func estimateTokens(obs []store.Observation) int {
	total := 0
	for _, o := range obs {
		total += (len(o.Title) + len(o.Content) + 60) / 4
	}
	return total
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

	// Memory fencing: mark this as recalled team configuration, not live instructions.
	return map[string]any{
		"_recall": "[RECALLED TEAM CONTEXT — these are your team's standing architecture rules and knowledge docs]",
		"team_id": s.session.teamID,
		"email":   s.session.email,
		"role":    s.session.role,
		"skills":  skills,
		"scrolls": scrolls,
	}, nil
}

// toolSDDPhase reads or updates the SDD phase for a project.
// GET: pass only "project" → returns current phase.
// SET: pass "project" + "phase" → validates gate, then updates and returns new state.
//
// Quality gate enforcement: transitions apply→verify and verify→archive require a
// quality checkpoint with gate_passed=true before the phase advance is allowed.
func (s *Server) toolSDDPhase(args map[string]any) (any, error) {
	project := stringArg(args, "project")
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	newPhase := stringArg(args, "phase")
	if newPhase != "" {
		// Validate phase value.
		valid := false
		for _, p := range store.AllSDDPhases {
			if p == newPhase {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid phase %q — valid phases: %s",
				newPhase, strings.Join(store.AllSDDPhases, ", "))
		}

		// Gate enforcement: check whether the current → new transition requires a
		// passing quality checkpoint.
		current, err := s.store.GetSDDPhase(project)
		if err != nil {
			return nil, fmt.Errorf("get current phase: %w", err)
		}
		if store.IsGatedTransition(string(current.Phase), newPhase) {
			cp, err := s.store.GetLatestCheckpointForPhase(project, string(current.Phase))
			if err != nil {
				return nil, fmt.Errorf("checking quality gate: %w", err)
			}
			if cp == nil || !cp.GatePassed {
				return nil, fmt.Errorf(
					"quality gate: cannot advance from %q to %q — "+
						"a vault_qa_checkpoint with gate_passed=true is required for the %q phase. "+
						"Call vault_qa_checklist then vault_qa_checkpoint to complete the assessment",
					current.Phase, newPhase, string(current.Phase),
				)
			}
		}

		if err := s.store.SetSDDPhase(project, store.SDDPhase(newPhase)); err != nil {
			return nil, fmt.Errorf("set sdd phase: %w", err)
		}
	}

	state, err := s.store.GetSDDPhase(project)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"project":      state.Project,
		"phase":        state.Phase,
		"updated_at":   state.UpdatedAt.UTC().Format(time.RFC3339),
		"all_phases":   store.AllSDDPhases,
		"gated_phases": []string{"apply", "verify"},
	}, nil
}

// toolQAChecklist returns the quality criteria for a phase + optional language.
func (s *Server) toolQAChecklist(args map[string]any) (any, error) {
	phase := stringArg(args, "phase")
	if phase == "" {
		return nil, fmt.Errorf("phase is required")
	}
	language := stringArg(args, "language")
	checklist := store.GetQualityChecklist(phase, language)
	return map[string]any{
		"checklist":    checklist,
		"phase_gates":  store.PhaseGates,
		"gated_phases": []string{"apply", "verify"},
		"hint": "Evaluate each criterion. Build findings with the criterion ID as 'rule'. " +
			"Call vault_qa_checkpoint with your assessment. " +
			"gate_passed=true requires ALL required criteria to pass AND score ≥ 70.",
	}, nil
}

// toolQACheckpoint records a QA assessment result and optionally unlocks a phase gate.
func (s *Server) toolQACheckpoint(args map[string]any) (any, error) {
	project := stringArg(args, "project")
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	phase := stringArg(args, "phase")
	if phase == "" {
		return nil, fmt.Errorf("phase is required")
	}
	status := stringArg(args, "status")
	if status == "" {
		return nil, fmt.Errorf("status is required")
	}

	// Parse findings array.
	var findings []store.QualityFinding
	if raw, ok := args["findings"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					findings = append(findings, store.QualityFinding{
						Rule:   stringArg(m, "rule"),
						Status: stringArg(m, "status"),
						Notes:  stringArg(m, "notes"),
					})
				}
			}
		}
	}
	if findings == nil {
		findings = []store.QualityFinding{}
	}

	score := intArg(args, "score", 0)
	gatePassed := boolArg(args, "gate_passed")

	cp := store.QualityCheckpoint{
		Project:    project,
		SessionID:  stringArg(args, "session_id"),
		Phase:      phase,
		Language:   stringArg(args, "language"),
		Status:     store.QualityStatus(status),
		Score:      score,
		Findings:   findings,
		Notes:      stringArg(args, "notes"),
		GatePassed: gatePassed,
	}

	id, err := s.store.SaveQualityCheckpoint(cp)
	if err != nil {
		return nil, fmt.Errorf("save checkpoint: %w", err)
	}

	result := map[string]any{
		"id":          id,
		"status":      "saved",
		"gate_passed": gatePassed,
		"score":       score,
		"phase":       phase,
	}

	if gatePassed {
		if nextPhase, ok := store.PhaseGates[phase]; ok {
			result["gate_unlocked"] = fmt.Sprintf(
				"Phase gate passed. Transition %s → %s is now unlocked. "+
					"Call vault_sdd_phase with phase=%q to advance.",
				phase, nextPhase, nextPhase,
			)
		}
	} else if _, gated := store.PhaseGates[phase]; gated {
		result["gate_note"] = fmt.Sprintf(
			"Phase %q is a gate. A checkpoint with gate_passed=true is required before advancing. "+
				"Resolve failing criteria and submit a new checkpoint.",
			phase,
		)
	}

	return result, nil
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

// toolExportLore exports the team's private scrolls as structured notes.
// Supports incremental export via the "since" timestamp parameter.
func (s *Server) toolExportLore(args map[string]any) (any, error) {
	opts := store.ExportScrollsOptions{}

	if s.session != nil {
		opts.TeamID = s.session.teamID
	}

	if sinceStr := stringArg(args, "since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			opts.Since = t
		}
	}

	notes, total, err := s.store.ExportScrolls(opts)
	if err != nil {
		return nil, fmt.Errorf("export lore: %w", err)
	}

	exportedAt := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"notes":       notes,
		"count":       len(notes),
		"total":       total,
		"exported_at": exportedAt,
		"team_id":     opts.TeamID,
		"incremental": !opts.Since.IsZero(),
	}, nil
}

// toolSkillMatch returns the team skills most relevant to a prompt + project.
// This is the transparent auto-loader — the AI calls it (or vault_context calls
// it implicitly) to discover which skills should be applied to a task.
func (s *Server) toolSkillMatch(args map[string]any) (any, error) {
	teamID := ""
	if s.session != nil {
		teamID = s.session.teamID
	}

	in := store.SkillMatchInput{
		TeamID:    teamID,
		Project:   stringArg(args, "project"),
		Prompt:    stringArg(args, "prompt"),
		FilePaths: stringSliceArg(args, "file_paths"),
		Limit:     intArg(args, "limit", 5),
	}

	matches, err := s.store.MatchSkills(in)
	if err != nil {
		return nil, fmt.Errorf("skill match: %w", err)
	}

	// Telemetry — best-effort, never blocks the response.
	promptHash := hashPrompt(in.Prompt)
	for _, m := range matches {
		s.store.LogSkillActivation(m.ID, teamID, in.Project, promptHash, m.Reason, m.Score)
	}

	tip := "No auto-matched skills. Ask the team admin to mark skills with auto_load=1 and add triggers."
	if len(matches) > 0 {
		tip = "These skills were auto-loaded based on your prompt + project. The body field contains the skill content — apply it as guidance for the current task."
	}

	return map[string]any{
		"skills":  matches,
		"count":   len(matches),
		"team_id": teamID,
		"project": in.Project,
		"tip":     tip,
	}, nil
}

// hashPrompt produces a short stable hash for telemetry (privacy-preserving).
func hashPrompt(prompt string) string {
	if prompt == "" {
		return ""
	}
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h[:6])
}

// toolCompress applies caveman-style compression to a text input. The mode is
// controlled by the optional "mode" arg or the KORVA_OUTPUT_MODE env var.
//
// Supported modes:
//   - "off"    — no compression (passthrough)
//   - "lite"   — strip filler words, keep grammar
//   - "full"   — drop articles + use fragments (default when enabled)
//   - "ultra"  — telegraphic; abbreviates aggressively
func (s *Server) toolCompress(args map[string]any) (any, error) {
	text := stringArg(args, "text")
	mode := stringArg(args, "mode")
	if mode == "" {
		mode = os.Getenv("KORVA_OUTPUT_MODE")
	}
	if mode == "" {
		mode = "full"
	}

	out, savedPct := compressText(text, mode)
	return map[string]any{
		"original":    text,
		"compressed":  out,
		"mode":        mode,
		"saved_pct":   savedPct,
		"orig_chars":  len(text),
		"final_chars": len(out),
	}, nil
}

// toolPatternMine scans recent observations for emerging implicit patterns.
func (s *Server) toolPatternMine(args map[string]any) (any, error) {
	project := stringArg(args, "project")
	maxResults := intArg(args, "max", 5)
	minCount := intArg(args, "min_count", 2)

	patterns, err := s.store.MinePatterns(project, maxResults, minCount)
	if err != nil {
		return nil, fmt.Errorf("pattern mine: %w", err)
	}

	tip := "No emerging patterns found in recent observations."
	if len(patterns) > 0 {
		tip = "Consider saving these implicit conventions as explicit patterns with vault_save (type=pattern) to preserve team knowledge."
	}

	return map[string]any{
		"patterns": patterns,
		"count":    len(patterns),
		"project":  project,
		"tip":      tip,
	}, nil
}

// toolCodeHealth returns composite health scores per project.
func (s *Server) toolCodeHealth(_ map[string]any) (any, error) {
	scores, err := s.store.CodeHealthSummary()
	if err != nil {
		return nil, fmt.Errorf("code health: %w", err)
	}
	return map[string]any{
		"projects": scores,
		"count":    len(scores),
	}, nil
}

// toolHint returns a minimal index of matching observations: id, type, title, project only.
// It costs ~10x fewer tokens than vault_search with full content, making it ideal for
// "does anything related to X exist?" queries before deciding what to fully load.
func (s *Server) toolHint(args map[string]any) (any, error) {
	limit := intArg(args, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	filters := store.SearchFilters{
		Project: stringArg(args, "project"),
		Type:    store.ObservationType(stringArg(args, "type")),
		Limit:   limit,
	}

	results, err := s.store.Search(stringArg(args, "query"), filters)
	if err != nil {
		return nil, fmt.Errorf("vault_hint: %w", err)
	}

	// Return only the lightweight fields — no content field.
	type hintItem struct {
		ID      string   `json:"id"`
		Type    string   `json:"type"`
		Title   string   `json:"title"`
		Project string   `json:"project"`
		Tags    []string `json:"tags,omitempty"`
	}
	hints := make([]hintItem, len(results))
	for i, obs := range results {
		hints[i] = hintItem{
			ID:      obs.ID,
			Type:    string(obs.Type),
			Title:   obs.Title,
			Project: obs.Project,
			Tags:    obs.Tags,
		}
	}

	return map[string]any{
		"hints": hints,
		"count": len(hints),
		"tip":   "Use vault_get to retrieve full content for specific IDs.",
	}, nil
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
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
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

func boolArg(args map[string]any, key string) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
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
