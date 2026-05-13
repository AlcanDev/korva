// Package harness implements Korva's Harness Engineering bootstrap and
// state machine. A "harness" is the small set of files that turn any repo
// into a place an AI agent can work autonomously and verifiably:
//
//	AGENTS.md             entry-point map for agents
//	feature_list.json     backlog with state machine
//	progress/current.md   live session log
//	progress/history.md   append-only past sessions
//	docs/architecture.md  what "good work" looks like
//	docs/conventions.md   style + naming rules
//	docs/verification.md  how to verify a feature is done
//	CHECKPOINTS.md        objective end-state criteria
//	init.sh               verify env + run tests
//	.claude/agents/*.md   optional subagent definitions
//
// The state machine has two flavors. The standard backlog uses
// {pending, in_progress, done, blocked} with at most one in_progress
// at a time. The SDD backlog (Phase 13 — `korva harness init --sdd`)
// inserts an intermediate `spec_ready` state so features with
// `sdd: true` must be drafted as `specs/<name>/{requirements,design,
// tasks}.md` and human-approved before any code touches the repo.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// FeatureStatus is the allowed states for a feature in the backlog.
type FeatureStatus string

const (
	StatusPending    FeatureStatus = "pending"
	StatusSpecReady  FeatureStatus = "spec_ready" // SDD only — spec drafted, awaiting human approval before implementation
	StatusInProgress FeatureStatus = "in_progress"
	StatusDone       FeatureStatus = "done"
	StatusBlocked    FeatureStatus = "blocked"
)

// ValidStatuses lists every legal status. Used by validation + CLI prompts.
// `spec_ready` is always accepted by the validator so a hand-edited
// feature_list.json survives a load even when no SDD rule is enabled — the
// state machine still gates which transitions can produce it.
var ValidStatuses = []FeatureStatus{StatusPending, StatusSpecReady, StatusInProgress, StatusDone, StatusBlocked}

// Feature is one row in the backlog. ID is integer + monotonic so agents
// pick "the smallest pending id" deterministically.
//
// When `SDD` is true the feature follows the spec-driven workflow:
// pending → spec_ready → in_progress → done. The spec_author subagent
// drafts specs/<name>/{requirements,design,tasks}.md before the
// implementer is allowed to touch code. A human approves the spec by
// transitioning the feature to in_progress.
type Feature struct {
	ID          int           `json:"id"`
	Name        string        `json:"name"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Acceptance  []string      `json:"acceptance,omitempty"`
	Status      FeatureStatus `json:"status"`
	SDD         bool          `json:"sdd,omitempty"`         // spec-driven feature?
	OwnerAgent  string        `json:"owner_agent,omitempty"` // who claimed it
	UpdatedAt   string        `json:"updated_at,omitempty"`  // ISO 8601 — set on every state change
}

// FeatureList is the wire shape of feature_list.json.
type FeatureList struct {
	Project     string    `json:"project"`
	Description string    `json:"description,omitempty"`
	Rules       Rules     `json:"rules"`
	Features    []Feature `json:"features"`
}

// Rules pins the invariants the state machine enforces. Embedded in the
// file so a human reading it knows the contract.
type Rules struct {
	OneFeatureAtATime              bool            `json:"one_feature_at_a_time"`
	RequireTestsToClose            bool            `json:"require_tests_to_close"`
	RequireApprovedSpecToImplement bool            `json:"require_approved_spec_to_implement,omitempty"`
	ValidStatuses                  []FeatureStatus `json:"valid_status"`
}

// DefaultRules returns the canonical ruleset for a standard harness.
// SDD mode (`RequireApprovedSpecToImplement: true`) is opt-in via
// SDDRules — initialized by `korva harness init --sdd`.
func DefaultRules() Rules {
	return Rules{
		OneFeatureAtATime:   true,
		RequireTestsToClose: true,
		ValidStatuses:       slices.Clone(ValidStatuses),
	}
}

// SDDRules returns the canonical ruleset for a spec-driven harness.
// Features flagged with `sdd: true` must be drafted (pending →
// spec_ready) and approved (spec_ready → in_progress) before any
// implementation touches code.
func SDDRules() Rules {
	r := DefaultRules()
	r.RequireApprovedSpecToImplement = true
	return r
}

// FeatureListPath is the conventional location of the backlog file.
const FeatureListPath = "feature_list.json"

// LoadFeatureList reads + validates feature_list.json from `root`.
func LoadFeatureList(root string) (*FeatureList, error) {
	path := filepath.Join(root, FeatureListPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read feature list: %w", err)
	}
	var fl FeatureList
	if err := json.Unmarshal(data, &fl); err != nil {
		return nil, fmt.Errorf("parse feature list: %w", err)
	}
	if err := Validate(&fl); err != nil {
		return nil, err
	}
	return &fl, nil
}

// SaveFeatureList writes feature_list.json atomically (temp file + rename),
// pretty-printed with 2-space indent. Validates before writing so we never
// persist an invalid file.
func SaveFeatureList(root string, fl *FeatureList) error {
	if err := Validate(fl); err != nil {
		return err
	}
	path := filepath.Join(root, FeatureListPath)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".feature_list-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(fl); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Validate enforces the invariants the state machine relies on. Called by
// LoadFeatureList and SaveFeatureList so neither read nor write can produce
// a file that the state machine wouldn't accept.
func Validate(fl *FeatureList) error {
	if fl == nil {
		return fmt.Errorf("nil feature list")
	}
	if strings.TrimSpace(fl.Project) == "" {
		return fmt.Errorf("project name is required")
	}
	seenIDs := map[int]bool{}
	inProgress := 0
	for i, f := range fl.Features {
		if f.ID <= 0 {
			return fmt.Errorf("feature %d: id must be > 0", i)
		}
		if seenIDs[f.ID] {
			return fmt.Errorf("duplicate feature id: %d", f.ID)
		}
		seenIDs[f.ID] = true
		if strings.TrimSpace(f.Name) == "" {
			return fmt.Errorf("feature %d: name is required", f.ID)
		}
		if !slices.Contains(ValidStatuses, f.Status) {
			return fmt.Errorf("feature %d (%s): invalid status %q", f.ID, f.Name, f.Status)
		}
		if f.Status == StatusInProgress {
			inProgress++
		}
	}
	if fl.Rules.OneFeatureAtATime && inProgress > 1 {
		return fmt.Errorf("more than one feature in_progress (%d) but one_feature_at_a_time is enforced", inProgress)
	}
	return nil
}

// NextPending returns the lowest-id `pending` feature, or nil when there's
// nothing left to start. Used by `korva harness next`.
func (fl *FeatureList) NextPending() *Feature {
	pending := make([]Feature, 0, len(fl.Features))
	for _, f := range fl.Features {
		if f.Status == StatusPending {
			pending = append(pending, f)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].ID < pending[j].ID })
	out := pending[0]
	return &out
}

// NextSpecReady returns the lowest-id feature in spec_ready (the one a
// human reviewer should approve next), or nil when none exists.
func (fl *FeatureList) NextSpecReady() *Feature {
	ready := make([]Feature, 0, len(fl.Features))
	for _, f := range fl.Features {
		if f.Status == StatusSpecReady {
			ready = append(ready, f)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	sort.SliceStable(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })
	out := ready[0]
	return &out
}

// CurrentInProgress returns the feature currently in_progress (or nil).
// The invariant max-one is enforced by Validate.
func (fl *FeatureList) CurrentInProgress() *Feature {
	for _, f := range fl.Features {
		if f.Status == StatusInProgress {
			out := f
			return &out
		}
	}
	return nil
}

// FindByID returns the feature by id (or nil).
func (fl *FeatureList) FindByID(id int) *Feature {
	for i := range fl.Features {
		if fl.Features[i].ID == id {
			return &fl.Features[i]
		}
	}
	return nil
}

// SetStatus changes a feature's status with state-machine guardrails.
// Returns an error when:
//   - id doesn't exist
//   - the transition is illegal for this feature's flavor (standard
//     vs SDD)
//   - moving another feature to in_progress while one is already there
//     (and one_feature_at_a_time is set)
//   - an SDD-gated feature is shoved straight from pending into
//     in_progress (the rule require_approved_spec_to_implement forbids
//     skipping the spec phase)
//
// On success it updates UpdatedAt and OwnerAgent (when provided).
func (fl *FeatureList) SetStatus(id int, status FeatureStatus, owner string, now string) error {
	f := fl.FindByID(id)
	if f == nil {
		return fmt.Errorf("feature %d not found", id)
	}
	if !slices.Contains(ValidStatuses, status) {
		return fmt.Errorf("invalid target status %q", status)
	}
	// SDD gate check fires *before* the generic illegal-transition error so
	// the operator gets a hint about the missing spec_ready step instead
	// of a bare "illegal transition" message.
	if fl.Rules.RequireApprovedSpecToImplement && f.SDD &&
		status == StatusInProgress && f.Status == StatusPending {
		return fmt.Errorf("feature %d is SDD-gated — run `korva harness ready %d` first (must pass through spec_ready)", id, id)
	}
	if !legalTransition(f.Status, status, f.SDD) {
		return fmt.Errorf("illegal transition: %s → %s (sdd=%v)", f.Status, status, f.SDD)
	}
	if status == StatusInProgress && fl.Rules.OneFeatureAtATime {
		cur := fl.CurrentInProgress()
		if cur != nil && cur.ID != id {
			return fmt.Errorf("cannot start: feature %d (%s) is already in_progress", cur.ID, cur.Name)
		}
	}
	f.Status = status
	if owner != "" {
		f.OwnerAgent = owner
	}
	if now != "" {
		f.UpdatedAt = now
	}
	return nil
}

// legalTransition enforces the directional state machine. The shape
// depends on whether the feature is SDD-flagged:
//
//	Standard (sdd=false):
//	  pending     → in_progress, blocked
//	  in_progress → done, blocked, pending
//	  blocked     → pending, in_progress
//	  spec_ready  → pending (defensive — only reachable via manual edit)
//	  done        → done (idempotent)
//
//	SDD (sdd=true):
//	  pending     → spec_ready, blocked
//	  spec_ready  → in_progress, pending, blocked
//	  in_progress → done, blocked, pending, spec_ready
//	  blocked     → pending, spec_ready, in_progress
//	  done        → done (idempotent)
func legalTransition(from, to FeatureStatus, sdd bool) bool {
	if from == to {
		return true
	}
	if sdd {
		return legalTransitionSDD(from, to)
	}
	return legalTransitionStd(from, to)
}

func legalTransitionStd(from, to FeatureStatus) bool {
	switch from {
	case StatusPending:
		return to == StatusInProgress || to == StatusBlocked
	case StatusInProgress:
		return to == StatusDone || to == StatusBlocked || to == StatusPending
	case StatusBlocked:
		return to == StatusPending || to == StatusInProgress
	case StatusSpecReady:
		// spec_ready isn't supposed to exist on a non-SDD feature, but if
		// it does (manual edit) accept transitions back to pending so the
		// operator can fix it without barfing.
		return to == StatusPending
	case StatusDone:
		return false
	}
	return false
}

func legalTransitionSDD(from, to FeatureStatus) bool {
	switch from {
	case StatusPending:
		return to == StatusSpecReady || to == StatusBlocked
	case StatusSpecReady:
		return to == StatusInProgress || to == StatusPending || to == StatusBlocked
	case StatusInProgress:
		return to == StatusDone || to == StatusBlocked || to == StatusPending || to == StatusSpecReady
	case StatusBlocked:
		return to == StatusPending || to == StatusSpecReady || to == StatusInProgress
	case StatusDone:
		return false
	}
	return false
}

// Counts is a small struct that powers `korva harness status`.
// SpecReady is zero in a standard (non-SDD) backlog and the dashboard
// can hide it; in an SDD backlog it tells the operator how many
// features are awaiting human approval.
type Counts struct {
	Pending    int `json:"pending"`
	SpecReady  int `json:"spec_ready,omitempty"`
	InProgress int `json:"in_progress"`
	Done       int `json:"done"`
	Blocked    int `json:"blocked"`
	Total      int `json:"total"`
}

// CountByStatus aggregates the backlog for the dashboard / CLI status view.
func (fl *FeatureList) CountByStatus() Counts {
	var c Counts
	for _, f := range fl.Features {
		switch f.Status {
		case StatusPending:
			c.Pending++
		case StatusSpecReady:
			c.SpecReady++
		case StatusInProgress:
			c.InProgress++
		case StatusDone:
			c.Done++
		case StatusBlocked:
			c.Blocked++
		}
	}
	c.Total = len(fl.Features)
	return c
}
