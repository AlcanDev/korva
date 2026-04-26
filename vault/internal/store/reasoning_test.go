package store

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReasoningHint(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		obs      Observation
		query    string
		contains []string
		empty    bool
	}{
		{
			name:     "decision type recent",
			obs:      Observation{Type: TypeDecision, CreatedAt: now.Add(-1 * time.Hour)},
			contains: []string{"Architectural decision", "saved this week"},
		},
		{
			name:     "pattern type this month",
			obs:      Observation{Type: TypePattern, CreatedAt: now.Add(-15 * 24 * time.Hour)},
			contains: []string{"Reusable pattern", "saved this month"},
		},
		{
			name:     "antipattern stale",
			obs:      Observation{Type: TypeAntiPattern, CreatedAt: now.Add(-200 * 24 * time.Hour)},
			contains: []string{"Anti-pattern to avoid", "months ago", "verify still current"},
		},
		{
			name:     "bugfix type",
			obs:      Observation{Type: TypeBugfix, CreatedAt: now.Add(-1 * time.Hour)},
			contains: []string{"Bug fix record"},
		},
		{
			name:     "learning type",
			obs:      Observation{Type: TypeLearning, CreatedAt: now.Add(-1 * time.Hour)},
			contains: []string{"Learning note"},
		},
		{
			name:     "sdd phase tag",
			obs:      Observation{Type: TypeContext, Tags: []string{"sdd:apply", "nestjs"}, CreatedAt: now.Add(-1 * time.Hour)},
			contains: []string{"from SDD apply phase"},
		},
		{
			name:     "query matching tag",
			obs:      Observation{Type: TypeContext, Tags: []string{"nestjs", "auth"}, CreatedAt: now.Add(-1 * time.Hour)},
			query:    "nestjs authentication",
			contains: []string{"tagged:", "nestjs"},
		},
		{
			name:  "context type mid-age returns empty",
			obs:   Observation{Type: TypeContext, CreatedAt: now.Add(-60 * 24 * time.Hour)},
			query: "",
			empty: true,
		},
		{
			name:     "separator dot between multiple signals",
			obs:      Observation{Type: TypeDecision, Tags: []string{"sdd:design"}, CreatedAt: now.Add(-1 * time.Hour)},
			contains: []string{"·"},
		},
		{
			name:     "sdd tag not doubled in query match",
			obs:      Observation{Type: TypeContext, Tags: []string{"sdd:verify"}, CreatedAt: now.Add(-1 * time.Hour)},
			query:    "sdd verify phase",
			contains: []string{"from SDD verify phase"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := BuildReasoningHint(tt.obs, tt.query)
			if tt.empty {
				if hint != "" {
					t.Errorf("expected empty hint, got %q", hint)
				}
				return
			}
			for _, want := range tt.contains {
				if !strings.Contains(hint, want) {
					t.Errorf("hint %q does not contain %q", hint, want)
				}
			}
		})
	}
}

func TestBuildReasoningHint_NoQuery(t *testing.T) {
	obs := Observation{
		Type:      TypeDecision,
		Tags:      []string{"auth", "jwt"},
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	hint := BuildReasoningHint(obs, "")
	if !strings.Contains(hint, "Architectural decision") {
		t.Errorf("expected type signal in hint, got %q", hint)
	}
	if strings.Contains(hint, "tagged:") {
		t.Errorf("should not produce tag match when query is empty, got %q", hint)
	}
}
