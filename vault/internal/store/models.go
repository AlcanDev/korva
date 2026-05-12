package store

import "time"

// ObservationType represents the category of a stored observation.
type ObservationType string

const (
	TypeDecision    ObservationType = "decision"
	TypePattern     ObservationType = "pattern"
	TypeBugfix      ObservationType = "bugfix"
	TypeLearning    ObservationType = "learning"
	TypeContext     ObservationType = "context"
	TypeAntiPattern ObservationType = "antipattern"
	TypeTask        ObservationType = "task"
	TypeFeature     ObservationType = "feature"   // new capability added to the codebase
	TypeRefactor    ObservationType = "refactor"  // structural improvement without behavior change
	TypeDiscovery   ObservationType = "discovery" // unexpected finding worth remembering
	TypeIncident    ObservationType = "incident"  // production issue or operational event
)

// AllObservationTypes lists every valid ObservationType.
// Used for enum validation and documentation.
var AllObservationTypes = []string{
	"decision", "pattern", "bugfix", "learning", "context",
	"antipattern", "task", "feature", "refactor", "discovery", "incident",
}

// ── SDD phase — Spec-Driven Development workflow state ─────────────────────

// SDDPhase represents the current phase of a Spec-Driven Development workflow.
// Models the current phase of a multi-stage SDD workflow.
type SDDPhase string

const (
	SDDExplore SDDPhase = "explore" // rapid investigation
	SDDPropose SDDPhase = "propose" // solution sketches
	SDDSpec    SDDPhase = "spec"    // detailed requirements
	SDDDesign  SDDPhase = "design"  // architecture definition
	SDDTasks   SDDPhase = "tasks"   // actionable decomposition
	SDDApply   SDDPhase = "apply"   // code implementation
	SDDVerify  SDDPhase = "verify"  // testing & validation
	SDDArchive SDDPhase = "archive" // documentation
	SDDOnboard SDDPhase = "onboard" // team knowledge capture
)

// AllSDDPhases lists every valid SDD phase in execution order.
var AllSDDPhases = []string{
	"explore", "propose", "spec", "design",
	"tasks", "apply", "verify", "archive", "onboard",
}

// SDDState records the active SDD phase for a project.
type SDDState struct {
	Project   string    `json:"project"`
	Phase     SDDPhase  `json:"phase"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ── Project conventions — specification metadata per project ────────────────

// OpenSpec holds per-project conventions (stack, rules, testing standards).
// It is injected automatically into every MCP session for the project so
// the AI always has architecture context without being asked.
type OpenSpec struct {
	Project   string    `json:"project"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ── Quality gate system ───────────────────────────────────────────────────────

// QualityStatus is the outcome of a quality checkpoint.
type QualityStatus string

const (
	QualityPass    QualityStatus = "pass"    // all criteria met, gate unlocked
	QualityFail    QualityStatus = "fail"    // blocking issues found, cannot advance
	QualityPartial QualityStatus = "partial" // some criteria met, needs attention
	QualitySkip    QualityStatus = "skip"    // explicitly skipped with justification
)

// QualityFinding records the result of a single quality criterion check.
type QualityFinding struct {
	Rule   string `json:"rule"`
	Status string `json:"status"` // "pass", "fail", "partial", "na"
	Notes  string `json:"notes,omitempty"`
}

// QualityCheckpoint records the outcome of a QA assessment at a SDD phase.
// The AI agent performs the assessment; Korva stores it for tracking and gating.
type QualityCheckpoint struct {
	ID         string           `json:"id"`
	Project    string           `json:"project"`
	SessionID  string           `json:"session_id,omitempty"`
	Phase      string           `json:"phase"`
	Language   string           `json:"language"`
	Status     QualityStatus    `json:"status"`
	Score      int              `json:"score"` // 0-100
	Findings   []QualityFinding `json:"findings"`
	Notes      string           `json:"notes,omitempty"`
	GatePassed bool             `json:"gate_passed"`
	CreatedAt  time.Time        `json:"created_at"`
}

// QualityCriterion describes a single quality requirement.
// These are defined statically per language/phase in quality_rules.go.
type QualityCriterion struct {
	ID       string `json:"id"`
	Category string `json:"category"` // testing | patterns | e2e | style | security | docs
	Rule     string `json:"rule"`
	Guidance string `json:"guidance,omitempty"`
	Severity string `json:"severity"` // error | warning | info
	Required bool   `json:"required"` // if true, must pass for gate_passed=true
}

// QualityChecklist is the full quality specification for a phase + language combination.
type QualityChecklist struct {
	Phase       string             `json:"phase"`
	Language    string             `json:"language"`
	GatePhase   string             `json:"gate_phase,omitempty"` // phase this checklist gates access to
	Description string             `json:"description"`
	Criteria    []QualityCriterion `json:"criteria"`
	E2ERequired bool               `json:"e2e_required"` // true for verify phase
}

// ProjectQualityScore summarizes quality trends for a project.
type ProjectQualityScore struct {
	Project        string    `json:"project"`
	LatestScore    int       `json:"latest_score"`
	AverageScore   int       `json:"average_score"`
	TotalChecks    int       `json:"total_checks"`
	PassedGates    int       `json:"passed_gates"`
	LastCheckPhase string    `json:"last_check_phase,omitempty"`
	LastCheckedAt  time.Time `json:"last_checked_at,omitempty"`
}

// Observation is a piece of knowledge saved to the Vault.
type Observation struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id,omitempty"`
	Project   string          `json:"project"`
	Team      string          `json:"team"`
	Country   string          `json:"country"`
	Type      ObservationType `json:"type"`
	Title     string          `json:"title"`
	Content   string          `json:"content"`
	Tags      []string        `json:"tags"`
	Author    string          `json:"author"`
	CreatedAt time.Time       `json:"created_at"`
	// TopicKey is an optional stable identifier for upsert. When set, vault_save
	// updates an existing observation with the same (project, topic_key) instead
	// of inserting a new row — ideal for knowledge that evolves over sessions.
	TopicKey string `json:"topic_key,omitempty"`
	// WorkingDir is the filesystem directory recorded at save time.
	// Used for project auto-detection audit trail.
	WorkingDir string `json:"working_dir,omitempty"`
	// ReasoningHint explains WHY this observation is relevant in the current search context.
	// Populated by the search layer when why=true is passed; empty otherwise to avoid noise.
	ReasoningHint string `json:"reasoning_hint,omitempty"`
}

// ── Observation relations ─────────────────────────────────────────────────────

// RelationType represents a semantic link between two observations.
type RelationType string

const (
	RelationSupersedes RelationType = "supersedes"     // source replaces target (target is outdated)
	RelationConflicts  RelationType = "conflicts_with" // source and target are contradictory
	RelationRelated    RelationType = "related"        // topically related, no conflict
	RelationCompatible RelationType = "compatible"     // complementary, no conflict
	RelationScoped     RelationType = "scoped"         // same topic but different scope/context
)

// AllRelationTypes lists all valid relation types.
var AllRelationTypes = []string{
	"supersedes", "conflicts_with", "related", "compatible", "scoped",
}

// JudgmentStatus captures the lifecycle of an auto-detected candidate conflict.
// pending  — Vault flagged a likely conflict; waiting on an actor to weigh in.
// judged   — Verdict recorded (the `relation` field carries the chosen verb).
// orphaned — Source/target observation was deleted before judgment.
// ignored  — Actor explicitly skipped this candidate (low confidence or noise).
type JudgmentStatus string

const (
	JudgmentPending  JudgmentStatus = "pending"
	JudgmentJudged   JudgmentStatus = "judged"
	JudgmentOrphaned JudgmentStatus = "orphaned"
	JudgmentIgnored  JudgmentStatus = "ignored"
)

// AllJudgmentStatuses lists every valid status, in lifecycle order.
var AllJudgmentStatuses = []string{"pending", "judged", "orphaned", "ignored"}

// ActorKind tags the role of the entity that filed a verdict.
type ActorKind string

const (
	ActorAgent ActorKind = "agent" // an AI agent acting on its own heuristic
	ActorUser  ActorKind = "user"  // a human operator
	ActorAdmin ActorKind = "admin" // Vault internal / admin tooling
)

// VerdictKind tags how the verdict was reached.
type VerdictKind string

const (
	VerdictHeuristic VerdictKind = "heuristic" // rule-based (e.g. FTS+confidence threshold)
	VerdictLLM       VerdictKind = "llm"       // delegated to an external LLM
	VerdictManual    VerdictKind = "manual"    // explicit human / admin action
)

// Relation is a directed semantic link between two observations.
type Relation struct {
	ID        string       `json:"id"`
	SourceID  string       `json:"source_id"`
	TargetID  string       `json:"target_id"`
	Relation  RelationType `json:"relation"`
	Status    string       `json:"status"` // confirmed | pending (legacy)
	Reason    string       `json:"reason,omitempty"`
	Author    string       `json:"author,omitempty"`
	Project   string       `json:"project"`
	CreatedAt time.Time    `json:"created_at"`

	// ── judgment workflow (added in Phase 2) ────────────────────────────────
	// Pre-Phase-2 rows materialize with JudgmentStatus="judged" + Confidence=1.0
	// + MarkedBy*="admin"/"manual", so the legacy AddRelation flow stays
	// semantically equivalent to "this verdict was already decided".
	JudgmentStatus JudgmentStatus `json:"judgment_status"`
	Confidence     float64        `json:"confidence"`
	Evidence       string         `json:"evidence,omitempty"`
	MarkedByActor  ActorKind      `json:"marked_by_actor"`
	MarkedByKind   VerdictKind    `json:"marked_by_kind"`
	MarkedByModel  string         `json:"marked_by_model,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	JudgedAt       *time.Time     `json:"judged_at,omitempty"`
}

// ObservationRelations holds all relations for a given observation.
type ObservationRelations struct {
	AsSource []Relation `json:"as_source"` // this obs is the source (supersedes, conflicts_with…)
	AsTarget []Relation `json:"as_target"` // this obs is the target (superseded_by…)
}

// UpdateObservationParams holds the optional fields for vault_update.
// Only non-nil / non-empty fields are applied.
type UpdateObservationParams struct {
	Title   *string
	Content *string
	Type    *ObservationType
	Tags    []string
}

// ProjectStats is a compact summary of a project's knowledge.
type ProjectStats struct {
	Name             string `json:"name"`
	ObservationCount int    `json:"observation_count"`
	SessionCount     int    `json:"session_count"`
}

// CaptureResult holds the outcome of a vault_capture call.
type CaptureResult struct {
	Saved   int      `json:"saved"`
	Skipped int      `json:"skipped"` // duplicates or empty
	IDs     []string `json:"ids"`
}

// Session represents a working session (a period of AI-assisted development).
type Session struct {
	ID        string     `json:"id"`
	Project   string     `json:"project"`
	Team      string     `json:"team"`
	Country   string     `json:"country"`
	Agent     string     `json:"agent"`
	Goal      string     `json:"goal"`
	Summary   string     `json:"summary"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Prompt is a reusable AI prompt template.
type Prompt struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchFilters constrains vault_search results.
type SearchFilters struct {
	Project string
	Team    string
	Country string
	Type    ObservationType
	Author  string
	// Since/Until filter by creation timestamp (zero value = no bound).
	Since time.Time
	Until time.Time
	Limit int
	// Offset skips this many results; used for page-based navigation alongside Limit.
	Offset int
}

// PurgeOptions controls the bulk-delete operation. At least one filter is required
// to prevent accidental full-table deletion.
type PurgeOptions struct {
	Project string
	Team    string
	Type    string
	// Before deletes observations created strictly before this time.
	Before time.Time
	// DryRun returns the count without deleting anything.
	DryRun bool
}

// ExportOptions constrains which observations are included in an export.
// All fields are optional; empty values mean "no filter".
type ExportOptions struct {
	Project string
	Team    string
	Type    string
}

// DedupResult summarizes what vault_clean found or removed.
type DedupResult struct {
	Total      int  `json:"total"`
	Duplicates int  `json:"duplicates"`
	Removed    int  `json:"removed"`
	DryRun     bool `json:"dry_run"`
}

// VaultStats aggregates counts across the vault.
type VaultStats struct {
	TotalObservations int            `json:"total_observations"`
	TotalSessions     int            `json:"total_sessions"`
	TotalPrompts      int            `json:"total_prompts"`
	TotalContentLen   int            `json:"total_content_len"` // sum of content char lengths
	ByType            map[string]int `json:"by_type"`
	ByProject         map[string]int `json:"by_project"`
	ByTeam            map[string]int `json:"by_team"`
	ByCountry         map[string]int `json:"by_country"`
	DailyActivity     []DailyCount   `json:"daily_activity"`  // last 30 days
	RecentSessions    []SessionRow   `json:"recent_sessions"` // last 8 sessions
}

// DailyCount is an observation count for one calendar day.
type DailyCount struct {
	Date  string `json:"date"` // "2026-05-01"
	Count int    `json:"count"`
}

// SessionRow is a compact session summary for the dashboard.
type SessionRow struct {
	ID          string  `json:"id"`
	Project     string  `json:"project"`
	Goal        string  `json:"goal"`
	Agent       string  `json:"agent"`
	ObsCount    int     `json:"obs_count"`
	StartedAt   string  `json:"started_at"`
	EndedAt     *string `json:"ended_at,omitempty"`
	DurationMin int     `json:"duration_min"`
}

// ProjectSummary is a high-level summary of knowledge for a project.
type ProjectSummary struct {
	Project      string        `json:"project"`
	Observations int           `json:"observations"`
	Sessions     int           `json:"sessions"`
	TopTags      []string      `json:"top_tags"`
	Recent       []Observation `json:"recent"`
	Decisions    []Observation `json:"decisions"`
}
