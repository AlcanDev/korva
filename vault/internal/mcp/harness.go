package mcp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/harness"
	"github.com/alcandev/korva/vault/internal/store"
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
	editors, err := parseEditorsArg(args, root)
	if err != nil {
		return nil, err
	}
	written, err := harness.Generate(harness.InitOptions{
		Root:        root,
		Project:     project,
		Description: stringArg(args, "description"),
		Stack:       stack,
		Editors:     editors,
		SDD:         boolArg(args, "sdd"),
		Overwrite:   boolArg(args, "overwrite"),
	})
	if err != nil {
		return nil, err
	}
	editorNames := make([]string, len(editors))
	for i, e := range editors {
		editorNames[i] = string(e)
	}
	return map[string]any{
		"root":          root,
		"project":       project,
		"stack":         string(stack),
		"editors":       editorNames,
		"sdd":           boolArg(args, "sdd"),
		"files_written": written,
	}, nil
}

// parseEditorsArg reads the `editors` arg in any of the shapes a JSON-RPC
// client may send:
//
//   - missing / null                → auto-detect from `root`
//   - "auto"                        → auto-detect from `root`
//   - "none"                        → install no editor rule files
//   - "claude,cursor"               → comma-separated string
//   - ["claude", "cursor"]          → array of strings
//
// Unknown editor names produce an error so typos surface early.
func parseEditorsArg(args map[string]any, root string) ([]harness.Editor, error) {
	raw, ok := args["editors"]
	if !ok || raw == nil {
		return harness.DetectEditors(root), nil
	}
	var names []string
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		switch strings.ToLower(s) {
		case "", "auto":
			return harness.DetectEditors(root), nil
		case "none":
			return nil, nil
		}
		for _, part := range strings.Split(s, ",") {
			names = append(names, strings.TrimSpace(part))
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				names = append(names, strings.TrimSpace(s))
			}
		}
	default:
		return nil, fmt.Errorf("editors arg must be a string or array of strings")
	}

	out := make([]harness.Editor, 0, len(names))
	seen := make(map[harness.Editor]bool, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		name := harness.Editor(strings.ToLower(n))
		if !harness.IsKnownEditor(name) {
			return nil, fmt.Errorf("unknown editor %q", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out, nil
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
		// Snapshot the previous status before SetStatus mutates it —
		// the transition log needs both sides.
		prev := fl.FindByID(id)
		var fromStatus harness.FeatureStatus
		if prev != nil {
			fromStatus = prev.Status
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
		bridged := s.bridgeSDDPhase(args, target)
		mirrored := s.persistHarnessSnapshot(args, root)
		s.recordHarnessTransition(args, root, id, fromStatus, target, owner)
		return map[string]any{
			"root":             root,
			"id":               id,
			"name":             f.Name,
			"status":           string(target),
			"owner":            owner,
			"updated":          now,
			"sdd_phase_synced": bridged,
			"snapshot_synced":  mirrored,
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
		SDD:         boolArg(args, "sdd"),
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

// ── vault_harness_spec ───────────────────────────────────────────────────────
//
// Materializes specs/<feature.Name>/{requirements,design,tasks}.md from
// the EARS templates. Refuses to operate on non-SDD features so the
// operator can't accidentally pollute a plain harness with spec
// scaffolding.

func (s *Server) toolHarnessSpec(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	id, err := readIDArg(args)
	if err != nil {
		return nil, err
	}
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	f := fl.FindByID(id)
	if f == nil {
		return nil, fmt.Errorf("feature %d not found", id)
	}
	if !f.SDD {
		return nil, fmt.Errorf("feature %d (%s) is not SDD-flagged", id, f.Name)
	}
	res, err := harness.MaterializeSpec(root, f, boolArg(args, "overwrite"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"root":     root,
		"id":       id,
		"name":     f.Name,
		"dir":      res.Dir,
		"written":  res.Written,
		"skipped":  res.Skipped,
		"complete": harness.SpecComplete(root, f.Name),
	}, nil
}

// ── vault_harness_ready ──────────────────────────────────────────────────────
//
// pending → spec_ready handoff: the spec_author subagent calls this
// when the three spec files are drafted and ready for human review.
// Refuses when the files aren't there (so the agent can't pretend a
// spec was written without writing it).

func (s *Server) toolHarnessReady(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	id, err := readIDArg(args)
	if err != nil {
		return nil, err
	}
	fl, err := harness.LoadFeatureList(root)
	if err != nil {
		return nil, err
	}
	f := fl.FindByID(id)
	if f == nil {
		return nil, fmt.Errorf("feature %d not found", id)
	}
	if !f.SDD {
		return nil, fmt.Errorf("feature %d (%s) is not SDD-flagged", id, f.Name)
	}
	if !harness.SpecComplete(root, f.Name) {
		return nil, fmt.Errorf("spec files missing for %s — call vault_harness_spec first, then draft them", f.Name)
	}
	fromStatus := f.Status
	owner := s.agentName(args)
	now := time.Now().UTC().Format(time.RFC3339)
	if err := fl.SetStatus(id, harness.StatusSpecReady, owner, now); err != nil {
		return nil, err
	}
	if err := harness.SaveFeatureList(root, fl); err != nil {
		return nil, err
	}
	bridged := s.bridgeSDDPhase(args, harness.StatusSpecReady)
	mirrored := s.persistHarnessSnapshot(args, root)
	s.recordHarnessTransition(args, root, id, fromStatus, harness.StatusSpecReady, owner)
	return map[string]any{
		"root":             root,
		"id":               id,
		"name":             f.Name,
		"status":           string(harness.StatusSpecReady),
		"owner":            owner,
		"updated":          now,
		"sdd_phase_synced": bridged,
		"snapshot_synced":  mirrored,
	}, nil
}

// ── vault_harness_check ──────────────────────────────────────────────────────
//
// Runs the same invariant suite as `korva harness check` but exposed
// over MCP so an agent can ask "is this harness in a good state?"
// without shelling out. Read-only.

func (s *Server) toolHarnessCheck(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	report, err := harness.Check(root)
	if err != nil {
		return nil, err
	}
	return report, nil
}

// ── vault_harness_spec_review ────────────────────────────────────────────────
//
// Phase 15.B — lints an SDD feature's spec (EARS validity + R↔T
// coverage + traceability) and returns the structured report. The
// reviewer subagent gates on `ok: true` before approving a feature
// for transition to in_progress.

func (s *Server) toolHarnessSpecReview(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	id, err := readIDArg(args)
	if err != nil {
		return nil, err
	}
	report, err := harness.ReviewSpec(root, id)
	if err != nil {
		return nil, err
	}
	return report, nil
}

// ── vault_harness_ci_install ─────────────────────────────────────────────────
//
// Materializes a CI workflow (Phase 15.A). Mirrors `korva harness ci
// install --provider=<X>` so an agent inside an MCP-aware editor can
// install the gate without dropping to a shell.

func (s *Server) toolHarnessCIInstall(args map[string]any) (any, error) {
	root := resolveHarnessRoot(args)
	provider := harness.CIProvider(strings.ToLower(strings.TrimSpace(stringArg(args, "provider"))))
	if provider == "" {
		return nil, fmt.Errorf("provider is required (one of github-actions, gitlab-ci)")
	}
	if !harness.IsKnownCIProvider(provider) {
		return nil, fmt.Errorf("unknown CI provider %q", provider)
	}
	res, err := harness.InstallCI(root, provider, boolArg(args, "overwrite"))
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ── bridge: harness state → vault_sdd_phase ──────────────────────────────────
//
// When a harness MCP tool is called with an explicit `project` arg, the
// resulting state transition is also pushed to the vault's per-project
// SDD phase tracker so `vault_context` users see "this project is in
// 'apply' phase" without parsing feature_list.json.
//
// The bridge is best-effort:
//   - missing project arg → skip silently (return false)
//   - unmappable status (pending / blocked) → skip silently (return false)
//   - store error → log + skip (return false)
// The harness state machine remains the source of truth either way.

// harnessToSDDPhase maps a harness status to the equivalent vault SDD
// phase. Pending and blocked don't map (the agent hasn't committed to a
// phase yet); the others land on the phase the operator would expect to
// see in the vault dashboard.
func harnessToSDDPhase(status harness.FeatureStatus) (store.SDDPhase, bool) {
	switch status {
	case harness.StatusSpecReady:
		return store.SDDSpec, true
	case harness.StatusInProgress:
		return store.SDDApply, true
	case harness.StatusDone:
		return store.SDDVerify, true
	}
	return "", false
}

// bridgeSDDPhase pushes the mapped phase to the vault when args carries
// a non-empty `project`. Returns true on a successful push (so callers
// can advertise the bridged state in their response payload).
func (s *Server) bridgeSDDPhase(args map[string]any, status harness.FeatureStatus) bool {
	project := strings.TrimSpace(stringArg(args, "project"))
	if project == "" {
		return false
	}
	phase, ok := harnessToSDDPhase(status)
	if !ok {
		return false
	}
	if err := s.store.SetSDDPhase(project, phase); err != nil {
		log.Printf("harness→sdd_phase bridge failed for project=%q phase=%s: %v", project, phase, err)
		return false
	}
	return true
}

// persistHarnessSnapshot is the Phase 14.1 sibling of bridgeSDDPhase:
// when a harness MCP write tool fires with `project` set, the resulting
// feature_list.json is also mirrored into the vault so Beacon's
// dashboard can render it across projects without each operator
// pushing snapshots manually.
//
// Best-effort like the bridge: missing project arg → skip; I/O errors
// → log + skip. Returns true on a successful upsert.
func (s *Server) persistHarnessSnapshot(args map[string]any, root string) bool {
	project := strings.TrimSpace(stringArg(args, "project"))
	if project == "" {
		return false
	}
	payload, err := os.ReadFile(filepath.Join(root, harness.FeatureListPath))
	if err != nil {
		log.Printf("harness snapshot read failed for project=%q root=%q: %v", project, root, err)
		return false
	}
	if err := s.store.SaveHarnessSnapshot(s.callerTeamID(), project, root, string(payload)); err != nil {
		log.Printf("harness snapshot save failed for project=%q root=%q: %v", project, root, err)
		return false
	}
	return true
}

// callerTeamID extracts the team_id from the MCP session, or returns ""
// for anonymous callers. The harness CLI doesn't authenticate to the
// vault, so empty is the common case in offline use; team-scoped REST
// queries treat empty rows as orphans (invisible). Beacon-driven flows
// always come through an authenticated session and produce non-empty
// team_id.
func (s *Server) callerTeamID() string {
	if s.session == nil {
		return ""
	}
	return s.session.teamID
}

// recordHarnessTransition appends one row to the harness_transitions
// log when `project` is set. Caller passes the from/to statuses
// explicitly because the in-memory feature_list before / after the
// SetStatus call is the only place that knows them — saves an extra
// query on the hot path.
func (s *Server) recordHarnessTransition(args map[string]any, root string, featureID int, from, to harness.FeatureStatus, owner string) bool {
	project := strings.TrimSpace(stringArg(args, "project"))
	if project == "" {
		return false
	}
	err := s.store.RecordHarnessTransition(store.HarnessTransition{
		TeamID:     s.callerTeamID(),
		Project:    project,
		Root:       root,
		FeatureID:  featureID,
		FromStatus: string(from),
		ToStatus:   string(to),
		Owner:      owner,
	})
	if err != nil {
		log.Printf("harness transition log failed for project=%q feature=%d: %v", project, featureID, err)
		return false
	}
	return true
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
	if f.SDD {
		out["sdd"] = true
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
