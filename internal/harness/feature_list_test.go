package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_RejectsDuplicateIDs(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusPending},
			{ID: 1, Name: "b", Status: StatusPending},
		},
	}
	if err := Validate(fl); err == nil {
		t.Error("expected duplicate-id error")
	}
}

func TestValidate_RejectsMultipleInProgress(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusInProgress},
			{ID: 2, Name: "b", Status: StatusInProgress},
		},
	}
	if err := Validate(fl); err == nil {
		t.Error("expected multiple-in-progress error")
	}
}

func TestValidate_RejectsInvalidStatus(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: "fictional"},
		},
	}
	if err := Validate(fl); err == nil {
		t.Error("expected invalid-status error")
	}
}

func TestNextPending_PicksLowestID(t *testing.T) {
	fl := &FeatureList{
		Features: []Feature{
			{ID: 3, Name: "c", Status: StatusPending},
			{ID: 1, Name: "a", Status: StatusDone},
			{ID: 2, Name: "b", Status: StatusPending},
		},
	}
	got := fl.NextPending()
	if got == nil || got.ID != 2 {
		t.Errorf("NextPending = %+v, want feature id=2", got)
	}
}

func TestNextPending_NilWhenAllNonPending(t *testing.T) {
	fl := &FeatureList{
		Features: []Feature{{ID: 1, Status: StatusDone}, {ID: 2, Status: StatusInProgress}},
	}
	if fl.NextPending() != nil {
		t.Error("expected nil when no pending features")
	}
}

func TestSetStatus_LegalTransitions(t *testing.T) {
	fl := &FeatureList{
		Project:  "x",
		Rules:    DefaultRules(),
		Features: []Feature{{ID: 1, Name: "a", Status: StatusPending}},
	}
	if err := fl.SetStatus(1, StatusInProgress, "agent-1", "2026-05-13T00:00:00Z"); err != nil {
		t.Fatalf("pending→in_progress: %v", err)
	}
	if err := fl.SetStatus(1, StatusDone, "agent-1", "2026-05-13T01:00:00Z"); err != nil {
		t.Fatalf("in_progress→done: %v", err)
	}
}

func TestSetStatus_RejectsIllegalTransition(t *testing.T) {
	fl := &FeatureList{
		Project:  "x",
		Rules:    DefaultRules(),
		Features: []Feature{{ID: 1, Name: "a", Status: StatusDone}},
	}
	if err := fl.SetStatus(1, StatusInProgress, "", ""); err == nil {
		t.Error("done→in_progress should be illegal")
	}
}

func TestSetStatus_OneFeatureAtATime(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusInProgress},
			{ID: 2, Name: "b", Status: StatusPending},
		},
	}
	if err := fl.SetStatus(2, StatusInProgress, "", ""); err == nil {
		t.Error("expected error: cannot start while another in_progress")
	}
}

func TestSetStatus_OwnerAndTimestampUpdate(t *testing.T) {
	fl := &FeatureList{
		Project:  "x",
		Rules:    DefaultRules(),
		Features: []Feature{{ID: 1, Name: "a", Status: StatusPending}},
	}
	_ = fl.SetStatus(1, StatusInProgress, "claude", "2026-05-13T00:00:00Z")
	if fl.Features[0].OwnerAgent != "claude" {
		t.Errorf("OwnerAgent = %q, want claude", fl.Features[0].OwnerAgent)
	}
	if fl.Features[0].UpdatedAt != "2026-05-13T00:00:00Z" {
		t.Errorf("UpdatedAt missing")
	}
}

func TestSaveAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	original := &FeatureList{
		Project:     "korva",
		Description: "test",
		Rules:       DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Title: "First", Status: StatusPending},
			{ID: 2, Name: "b", Title: "Second", Status: StatusDone, Acceptance: []string{"works"}},
		},
	}
	if err := SaveFeatureList(dir, original); err != nil {
		t.Fatalf("save: %v", err)
	}
	// File must exist with expected name.
	if _, err := os.Stat(filepath.Join(dir, FeatureListPath)); err != nil {
		t.Fatalf("file not created: %v", err)
	}
	loaded, err := LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Project != "korva" {
		t.Errorf("Project = %q", loaded.Project)
	}
	if len(loaded.Features) != 2 {
		t.Errorf("features = %d, want 2", len(loaded.Features))
	}
	if loaded.Features[1].Acceptance[0] != "works" {
		t.Errorf("Acceptance not preserved: %+v", loaded.Features[1].Acceptance)
	}
}

func TestCountByStatus(t *testing.T) {
	fl := &FeatureList{
		Features: []Feature{
			{ID: 1, Status: StatusPending},
			{ID: 2, Status: StatusInProgress},
			{ID: 3, Status: StatusDone},
			{ID: 4, Status: StatusDone},
			{ID: 5, Status: StatusBlocked},
		},
	}
	c := fl.CountByStatus()
	if c.Pending != 1 || c.InProgress != 1 || c.Done != 2 || c.Blocked != 1 || c.Total != 5 {
		t.Errorf("counts wrong: %+v", c)
	}
}
