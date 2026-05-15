package api

import (
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 20.A — direct coverage for validate.go's input guards.
// These functions are the API boundary's only line of defense
// against oversized fields + unknown observation types. The flows
// tests exercise them indirectly; this file pins each rule.

func TestValidateObservation_HappyPath(t *testing.T) {
	obs := store.Observation{
		Type: store.TypeLearning, Project: "p", Title: "t", Content: "c",
		Tags: []string{"a", "b"},
	}
	if err := validateObservation(&obs); err != nil {
		t.Errorf("happy path: %v", err)
	}
}

func TestValidateObservation_RejectsMissingType(t *testing.T) {
	obs := store.Observation{Project: "p", Title: "t", Content: "c"}
	err := validateObservation(&obs)
	if err == nil || !strings.Contains(err.Error(), "type is required") {
		t.Errorf("expected missing-type error, got %v", err)
	}
}

func TestValidateObservation_RejectsUnknownType(t *testing.T) {
	obs := store.Observation{
		Type: store.ObservationType("invented"), Project: "p", Title: "t", Content: "c",
	}
	err := validateObservation(&obs)
	if err == nil || !strings.Contains(err.Error(), "unknown observation type") {
		t.Errorf("expected unknown-type error, got %v", err)
	}
	// Error message must list the valid types so the caller can fix the bad input.
	if !strings.Contains(err.Error(), "learning") {
		t.Errorf("error should hint valid types, got %v", err)
	}
}

func TestValidateObservation_AcceptsAllKnownTypes(t *testing.T) {
	for _, ty := range store.AllObservationTypes {
		ty := ty
		t.Run(ty, func(t *testing.T) {
			obs := store.Observation{
				Type: store.ObservationType(ty), Project: "p", Title: "t", Content: "c",
			}
			if err := validateObservation(&obs); err != nil {
				t.Errorf("known type %q rejected: %v", ty, err)
			}
		})
	}
}

func TestValidateObservation_RejectsOversizedShortFields(t *testing.T) {
	long := strings.Repeat("x", maxShortField+1)
	cases := []struct {
		name string
		mod  func(*store.Observation)
	}{
		{"project", func(o *store.Observation) { o.Project = long }},
		{"team", func(o *store.Observation) { o.Team = long }},
		{"country", func(o *store.Observation) { o.Country = long }},
		{"author", func(o *store.Observation) { o.Author = long }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			obs := store.Observation{Type: store.TypeLearning, Project: "p", Title: "t", Content: "c"}
			tc.mod(&obs)
			err := validateObservation(&obs)
			if err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Errorf("expected %q error, got %v", tc.name, err)
			}
		})
	}
}

func TestValidateObservation_RejectsOversizedTitleAndContent(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		obs := store.Observation{
			Type: store.TypeLearning, Project: "p", Content: "c",
			Title: strings.Repeat("x", maxMediumField+1),
		}
		if err := validateObservation(&obs); err == nil || !strings.Contains(err.Error(), "title") {
			t.Errorf("expected title error, got %v", err)
		}
	})
	t.Run("content", func(t *testing.T) {
		obs := store.Observation{
			Type: store.TypeLearning, Project: "p", Title: "t",
			Content: strings.Repeat("x", maxContentLen+1),
		}
		if err := validateObservation(&obs); err == nil || !strings.Contains(err.Error(), "content") {
			t.Errorf("expected content error, got %v", err)
		}
	})
}

func TestValidateObservation_RejectsTooManyTags(t *testing.T) {
	tags := make([]string, maxTagsCount+1)
	for i := range tags {
		tags[i] = "tag"
	}
	obs := store.Observation{
		Type: store.TypeLearning, Project: "p", Title: "t", Content: "c", Tags: tags,
	}
	err := validateObservation(&obs)
	if err == nil || !strings.Contains(err.Error(), "tags") {
		t.Errorf("expected too-many-tags error, got %v", err)
	}
}

func TestValidateObservation_RejectsOversizedTag(t *testing.T) {
	obs := store.Observation{
		Type: store.TypeLearning, Project: "p", Title: "t", Content: "c",
		Tags: []string{"ok", strings.Repeat("x", maxTagLen+1)},
	}
	err := validateObservation(&obs)
	if err == nil || !strings.Contains(err.Error(), "tag") {
		t.Errorf("expected oversized-tag error, got %v", err)
	}
}

func TestValidateObservation_BoundaryExactMaxLengthsPass(t *testing.T) {
	// At-the-limit values must NOT be rejected — only EXCEEDING the
	// limit is illegal. Pin the strict-greater-than semantics.
	obs := store.Observation{
		Type:    store.TypeLearning,
		Project: strings.Repeat("p", maxShortField),
		Team:    strings.Repeat("t", maxShortField),
		Country: strings.Repeat("c", maxShortField),
		Author:  strings.Repeat("a", maxShortField),
		Title:   strings.Repeat("T", maxMediumField),
		Content: strings.Repeat("X", maxContentLen),
		Tags: func() []string {
			tags := make([]string, maxTagsCount)
			for i := range tags {
				tags[i] = strings.Repeat("t", maxTagLen)
			}
			return tags
		}(),
	}
	if err := validateObservation(&obs); err != nil {
		t.Errorf("at-the-limit values must pass, got %v", err)
	}
}

func TestValidateSession_HappyAndOversize(t *testing.T) {
	if err := validateSession("p", "t", "CL", "claude", "implement X"); err != nil {
		t.Errorf("happy: %v", err)
	}
	long := strings.Repeat("x", maxShortField+1)
	if err := validateSession(long, "", "", "", ""); err == nil {
		t.Error("oversized project should fail")
	}
	if err := validateSession("", long, "", "", ""); err == nil {
		t.Error("oversized team should fail")
	}
	if err := validateSession("", "", long, "", ""); err == nil {
		t.Error("oversized country should fail")
	}
	if err := validateSession("", "", "", long, ""); err == nil {
		t.Error("oversized agent should fail")
	}
	if err := validateSession("", "", "", "", strings.Repeat("g", maxMediumField+1)); err == nil {
		t.Error("oversized goal should fail")
	}
}

func TestValidatePrompt_RequiresName(t *testing.T) {
	if err := validatePrompt("", "ok"); err == nil {
		t.Error("empty name should fail")
	}
	if err := validatePrompt("   ", "ok"); err == nil {
		t.Error("whitespace-only name should fail")
	}
}

func TestValidatePrompt_RejectsOversize(t *testing.T) {
	if err := validatePrompt(strings.Repeat("n", maxShortField+1), "ok"); err == nil {
		t.Error("oversized name should fail")
	}
	if err := validatePrompt("ok", strings.Repeat("c", maxContentLen+1)); err == nil {
		t.Error("oversized content should fail")
	}
}

func TestValidatePrompt_HappyPath(t *testing.T) {
	if err := validatePrompt("my-prompt", "use this template"); err != nil {
		t.Errorf("happy path: %v", err)
	}
}
