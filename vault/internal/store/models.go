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
	// From claude-mem: richer vocabulary for development workflows.
	TypeFeature   ObservationType = "feature"   // new capability added to the codebase
	TypeRefactor  ObservationType = "refactor"  // structural improvement without behavior change
	TypeDiscovery ObservationType = "discovery" // unexpected finding worth remembering
)

// AllObservationTypes lists every valid ObservationType.
// Used for enum validation and documentation.
var AllObservationTypes = []string{
	"decision", "pattern", "bugfix", "learning", "context",
	"antipattern", "task", "feature", "refactor", "discovery",
}

// ── SDD phase (gentle-ai Engram / SDD workflow) ──────────────────────────────

// SDDPhase represents the current phase of a Spec-Driven Development workflow.
// Based on gentle-ai's nine-phase SDD orchestration model.
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

// ── OpenSpec (gentle-ai project conventions) ─────────────────────────────────

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
	// ReasoningHint explains WHY this observation is relevant in the current search context.
	// Populated by the search layer when why=true is passed; empty otherwise to avoid noise.
	ReasoningHint string `json:"reasoning_hint,omitempty"`
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
	ByType            map[string]int `json:"by_type"`
	ByProject         map[string]int `json:"by_project"`
	ByTeam            map[string]int `json:"by_team"`
	ByCountry         map[string]int `json:"by_country"`
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
