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
	Limit   int
}

// VaultStats aggregates counts across the vault.
type VaultStats struct {
	TotalObservations int            `json:"total_observations"`
	TotalSessions     int            `json:"total_sessions"`
	TotalPrompts      int            `json:"total_prompts"`
	ByType            map[string]int `json:"by_type"`
	ByProject         map[string]int `json:"by_project"`
	ByTeam            map[string]int `json:"by_team"`
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
