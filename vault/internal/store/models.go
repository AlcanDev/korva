package store

import "time"

// ObservationType represents the category of a stored observation.
type ObservationType string

const (
	TypeDecision   ObservationType = "decision"
	TypePattern    ObservationType = "pattern"
	TypeBugfix     ObservationType = "bugfix"
	TypeLearning   ObservationType = "learning"
	TypeContext    ObservationType = "context"
	TypeAntiPattern ObservationType = "antipattern"
	TypeTask       ObservationType = "task"
)

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
}

// Session represents a working session (a period of AI-assisted development).
type Session struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Team      string    `json:"team"`
	Country   string    `json:"country"`
	Agent     string    `json:"agent"`
	Goal      string    `json:"goal"`
	Summary   string    `json:"summary"`
	StartedAt time.Time `json:"started_at"`
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
	Limit  int
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

// DedupResult summarises what vault_clean found or removed.
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
