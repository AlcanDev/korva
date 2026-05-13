package harness

import (
	"os"
	"path/filepath"
	"strings"
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
			{ID: 6, Status: StatusSpecReady},
		},
	}
	c := fl.CountByStatus()
	if c.Pending != 1 || c.SpecReady != 1 || c.InProgress != 1 || c.Done != 2 || c.Blocked != 1 || c.Total != 6 {
		t.Errorf("counts wrong: %+v", c)
	}
}

// ───────────────────────── Phase 13.1 — SDD-mode tests ─────────────────────────

func TestSDDRules_OptsInToSpecGate(t *testing.T) {
	d := DefaultRules()
	if d.RequireApprovedSpecToImplement {
		t.Error("DefaultRules must not require approved spec")
	}
	s := SDDRules()
	if !s.RequireApprovedSpecToImplement {
		t.Error("SDDRules must require approved spec")
	}
	// SDD rules still inherit the other invariants.
	if !s.OneFeatureAtATime || !s.RequireTestsToClose {
		t.Errorf("SDDRules dropped a base invariant: %+v", s)
	}
}

func TestValidate_AcceptsSpecReadyStatus(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   SDDRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusSpecReady, SDD: true},
		},
	}
	if err := Validate(fl); err != nil {
		t.Errorf("spec_ready must validate: %v", err)
	}
}

func TestLegalTransition_StandardFeature(t *testing.T) {
	cases := []struct {
		from, to FeatureStatus
		want     bool
	}{
		{StatusPending, StatusInProgress, true},
		{StatusInProgress, StatusDone, true},
		{StatusInProgress, StatusBlocked, true},
		{StatusBlocked, StatusPending, true},
		{StatusDone, StatusDone, true},
		{StatusDone, StatusInProgress, false},
		{StatusPending, StatusSpecReady, false},
		{StatusPending, StatusDone, false},
		{StatusSpecReady, StatusPending, true},
		{StatusSpecReady, StatusInProgress, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if got := legalTransition(tc.from, tc.to, false); got != tc.want {
				t.Errorf("legalTransition(%s→%s, std) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestLegalTransition_SDDFeature(t *testing.T) {
	cases := []struct {
		from, to FeatureStatus
		want     bool
	}{
		{StatusPending, StatusSpecReady, true},
		{StatusSpecReady, StatusInProgress, true},
		{StatusInProgress, StatusDone, true},
		{StatusSpecReady, StatusPending, true},
		{StatusInProgress, StatusSpecReady, true},
		{StatusBlocked, StatusSpecReady, true},
		{StatusPending, StatusInProgress, false},
		{StatusPending, StatusDone, false},
		{StatusSpecReady, StatusDone, false},
		{StatusDone, StatusInProgress, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if got := legalTransition(tc.from, tc.to, true); got != tc.want {
				t.Errorf("legalTransition(%s→%s, sdd) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestSetStatus_SDDGate_BlocksPendingToInProgress(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   SDDRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusPending, SDD: true},
		},
	}
	err := fl.SetStatus(1, StatusInProgress, "", "")
	if err == nil {
		t.Fatal("expected SDD gate to block pending→in_progress")
	}
	if !strings.Contains(err.Error(), "spec_ready") && !strings.Contains(err.Error(), "harness ready") {
		t.Errorf("error should mention the missing step: %v", err)
	}
}

func TestSetStatus_SDDHappyPath(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   SDDRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusPending, SDD: true},
		},
	}
	if err := fl.SetStatus(1, StatusSpecReady, "spec_author", "2026-05-13T10:00:00Z"); err != nil {
		t.Fatalf("pending→spec_ready: %v", err)
	}
	if err := fl.SetStatus(1, StatusInProgress, "implementer", "2026-05-13T11:00:00Z"); err != nil {
		t.Fatalf("spec_ready→in_progress: %v", err)
	}
	if err := fl.SetStatus(1, StatusDone, "reviewer", "2026-05-13T12:00:00Z"); err != nil {
		t.Fatalf("in_progress→done: %v", err)
	}
}

func TestSetStatus_NonSDDIgnoresSpecReady(t *testing.T) {
	fl := &FeatureList{
		Project: "x",
		Rules:   DefaultRules(),
		Features: []Feature{
			{ID: 1, Name: "a", Status: StatusPending},
		},
	}
	if err := fl.SetStatus(1, StatusInProgress, "", ""); err != nil {
		t.Fatalf("non-SDD pending→in_progress should work: %v", err)
	}
	fl.Features[0].Status = StatusPending
	if err := fl.SetStatus(1, StatusSpecReady, "", ""); err == nil {
		t.Error("non-SDD pending→spec_ready must be illegal")
	}
}

func TestNextSpecReady_PicksLowestID(t *testing.T) {
	fl := &FeatureList{
		Features: []Feature{
			{ID: 3, Status: StatusSpecReady},
			{ID: 1, Status: StatusInProgress},
			{ID: 2, Status: StatusSpecReady},
		},
	}
	got := fl.NextSpecReady()
	if got == nil || got.ID != 2 {
		t.Errorf("NextSpecReady = %+v, want id=2", got)
	}
}

func TestNextSpecReady_NilWhenNone(t *testing.T) {
	fl := &FeatureList{Features: []Feature{{ID: 1, Status: StatusPending}}}
	if fl.NextSpecReady() != nil {
		t.Error("expected nil when no spec_ready features")
	}
}

func TestSaveAndLoad_PreservesSDDField(t *testing.T) {
	dir := t.TempDir()
	original := &FeatureList{
		Project: "korva",
		Rules:   SDDRules(),
		Features: []Feature{
			{ID: 1, Name: "spec_one", Title: "First", Status: StatusPending, SDD: true},
			{ID: 2, Name: "plain", Title: "Second", Status: StatusPending, SDD: false},
		},
	}
	if err := SaveFeatureList(dir, original); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFeatureList(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.Rules.RequireApprovedSpecToImplement {
		t.Error("SDD rule lost on roundtrip")
	}
	if !loaded.Features[0].SDD {
		t.Error("sdd:true lost on roundtrip")
	}
	if loaded.Features[1].SDD {
		t.Error("sdd:false leaked as true on roundtrip")
	}
}
