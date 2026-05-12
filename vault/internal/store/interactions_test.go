package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSaveInteraction_HappyPath(t *testing.T) {
	s := newTestStore(t)

	in := Interaction{
		Project:         "korva",
		Agent:           "claude",
		Model:           "claude-opus-4-7",
		PromptExcerpt:   "Implementa endpoint POST /admin/system-status",
		ResponseExcerpt: "Listo. He creado el handler.",
		InputTokens:     8200,
		OutputTokens:    950,
		CacheRead:       6100,
		CacheCreation:   120,
		DurationMs:      1234,
		ToolCalls:       json.RawMessage(`[{"name":"Read"}]`),
	}

	id, err := s.SaveInteraction(in)
	if err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if id == "" {
		t.Fatal("SaveInteraction() returned empty id")
	}

	got, err := s.GetInteraction(id)
	if err != nil {
		t.Fatalf("GetInteraction() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetInteraction() returned nil")
	}
	if got.InputTokens != 8200 || got.OutputTokens != 950 || got.CacheRead != 6100 {
		t.Errorf("token totals mismatch: %+v", got)
	}
	if got.Estimated {
		t.Error("Estimated should be false when usage was provided")
	}
	if got.Status != "ok" {
		t.Errorf("default Status = %q, want %q", got.Status, "ok")
	}
}

func TestSaveInteraction_RequiresProjectAndAgent(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name string
		in   Interaction
	}{
		{"missing project", Interaction{Agent: "claude"}},
		{"missing agent", Interaction{Project: "korva"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.SaveInteraction(tc.in); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestSaveInteraction_EstimatedFallback(t *testing.T) {
	s := newTestStore(t)

	in := Interaction{
		Project:         "korva",
		Agent:           "cursor",
		Model:           "claude-sonnet-4-6",
		PromptExcerpt:   strings.Repeat("a", 400),
		ResponseExcerpt: strings.Repeat("b", 200),
	}

	id, err := s.SaveInteraction(in)
	if err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	got, _ := s.GetInteraction(id)
	if !got.Estimated {
		t.Error("Estimated should be true when no usage was reported")
	}
	if got.InputTokens != 100 { // 400 / 4
		t.Errorf("estimated input_tokens = %d, want 100", got.InputTokens)
	}
	if got.OutputTokens != 50 { // 200 / 4
		t.Errorf("estimated output_tokens = %d, want 50", got.OutputTokens)
	}
}

func TestSaveInteraction_PrivacyFilter(t *testing.T) {
	s := newTestStore(t)

	in := Interaction{
		Project:         "korva",
		Agent:           "claude",
		PromptExcerpt:   "I configured password=hunter2 for the env",
		ResponseExcerpt: "I see token=abc123xyz in the request",
	}
	id, err := s.SaveInteraction(in)
	if err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	got, _ := s.GetInteraction(id)
	if strings.Contains(got.PromptExcerpt, "hunter2") {
		t.Errorf("prompt excerpt was not redacted: %q", got.PromptExcerpt)
	}
	if strings.Contains(got.ResponseExcerpt, "abc123xyz") {
		t.Errorf("response excerpt was not redacted: %q", got.ResponseExcerpt)
	}
}

func TestSaveInteraction_ExcerptTruncation(t *testing.T) {
	s := newTestStore(t)

	huge := strings.Repeat("x", excerptMaxBytes*2)
	in := Interaction{
		Project:       "korva",
		Agent:         "claude",
		PromptExcerpt: huge,
	}
	id, err := s.SaveInteraction(in)
	if err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	got, _ := s.GetInteraction(id)
	if len(got.PromptExcerpt) > excerptMaxBytes {
		t.Errorf("PromptExcerpt len = %d, want <= %d", len(got.PromptExcerpt), excerptMaxBytes)
	}
}

func TestSaveInteraction_RejectsInvalidToolCalls(t *testing.T) {
	s := newTestStore(t)

	in := Interaction{
		Project:   "korva",
		Agent:     "claude",
		ToolCalls: json.RawMessage(`{not-valid-json}`),
	}
	if _, err := s.SaveInteraction(in); err == nil {
		t.Error("expected error for invalid tool_calls JSON")
	}
}

func TestListInteractions_FiltersAndPagination(t *testing.T) {
	s := newTestStore(t)

	seed := []Interaction{
		{Project: "korva", Agent: "claude", Model: "opus", InputTokens: 100, OutputTokens: 50},
		{Project: "korva", Agent: "claude", Model: "sonnet", InputTokens: 200, OutputTokens: 80},
		{Project: "other", Agent: "cursor", Model: "opus", InputTokens: 300, OutputTokens: 120},
		{Project: "other", Agent: "cursor", Model: "haiku", Status: "error", InputTokens: 50, OutputTokens: 10},
	}
	for _, in := range seed {
		if _, err := s.SaveInteraction(in); err != nil {
			t.Fatalf("seed Save() error = %v", err)
		}
	}

	tests := []struct {
		name    string
		filters InteractionFilters
		want    int
	}{
		{"all", InteractionFilters{}, 4},
		{"by project", InteractionFilters{Project: "korva"}, 2},
		{"by agent", InteractionFilters{Agent: "cursor"}, 2},
		{"by model", InteractionFilters{Model: "opus"}, 2},
		{"by status", InteractionFilters{Status: "error"}, 1},
		{"limit", InteractionFilters{Limit: 2}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ListInteractions(tc.filters)
			if err != nil {
				t.Fatalf("ListInteractions() error = %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestListInteractions_FTSQuery(t *testing.T) {
	s := newTestStore(t)

	seed := []Interaction{
		{Project: "korva", Agent: "claude", PromptExcerpt: "implementa el endpoint observatory"},
		{Project: "korva", Agent: "claude", PromptExcerpt: "agrega test para handler"},
		{Project: "korva", Agent: "claude", PromptExcerpt: "corrige bug en sentinel"},
	}
	for _, in := range seed {
		if _, err := s.SaveInteraction(in); err != nil {
			t.Fatalf("seed Save() error = %v", err)
		}
	}

	got, err := s.ListInteractions(InteractionFilters{Query: "observatory"})
	if err != nil {
		t.Fatalf("ListInteractions() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("FTS results len = %d, want 1; got=%v", len(got), got)
	}
}

func TestCountInteractions(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		if _, err := s.SaveInteraction(Interaction{Project: "korva", Agent: "claude"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := s.SaveInteraction(Interaction{Project: "other", Agent: "claude"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	total, err := s.CountInteractions(InteractionFilters{})
	if err != nil {
		t.Fatalf("CountInteractions() error = %v", err)
	}
	if total != 8 {
		t.Errorf("total = %d, want 8", total)
	}

	scoped, _ := s.CountInteractions(InteractionFilters{Project: "korva"})
	if scoped != 5 {
		t.Errorf("scoped count = %d, want 5", scoped)
	}

	if fts, _ := s.CountInteractions(InteractionFilters{Query: "anything"}); fts != -1 {
		t.Errorf("FTS query should return -1, got %d", fts)
	}
}

func TestGetTokenStats_AggregationAndCacheHit(t *testing.T) {
	s := newTestStore(t)

	seed := []Interaction{
		{Project: "korva", Agent: "claude", Model: "opus", InputTokens: 1000, OutputTokens: 200, CacheRead: 500},
		{Project: "korva", Agent: "claude", Model: "opus", InputTokens: 800, OutputTokens: 100, CacheRead: 200},
		{Project: "other", Agent: "cursor", Model: "sonnet", InputTokens: 400, OutputTokens: 50, CacheRead: 0},
	}
	for _, in := range seed {
		if _, err := s.SaveInteraction(in); err != nil {
			t.Fatalf("seed error = %v", err)
		}
	}

	stats, err := s.GetTokenStats(time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("GetTokenStats() error = %v", err)
	}

	if stats.InputTokens != 2200 {
		t.Errorf("InputTokens = %d, want 2200", stats.InputTokens)
	}
	if stats.OutputTokens != 350 {
		t.Errorf("OutputTokens = %d, want 350", stats.OutputTokens)
	}
	if stats.CacheRead != 700 {
		t.Errorf("CacheRead = %d, want 700", stats.CacheRead)
	}
	if stats.InteractionsCount != 3 {
		t.Errorf("InteractionsCount = %d, want 3", stats.InteractionsCount)
	}

	// CacheHitPct = 700 / (2200 + 700) ≈ 0.241
	wantPct := 700.0 / (2200.0 + 700.0)
	if stats.CacheHitPct < wantPct-0.001 || stats.CacheHitPct > wantPct+0.001 {
		t.Errorf("CacheHitPct = %f, want ~%f", stats.CacheHitPct, wantPct)
	}

	if got := stats.ByModel["opus"]; got.Count != 2 || got.InputTokens != 1800 {
		t.Errorf("ByModel[opus] = %+v, want Count=2 Input=1800", got)
	}
	if got := stats.ByProject["korva"]; got.Count != 2 || got.InputTokens != 1800 {
		t.Errorf("ByProject[korva] = %+v, want Count=2 Input=1800", got)
	}
}

func TestPurgeInteractionsOlderThan(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC()
	old, recent := now.AddDate(0, 0, -45), now.AddDate(0, 0, -1)
	for _, ts := range []time.Time{old, old, recent, recent, recent} {
		if _, err := s.SaveInteraction(Interaction{
			Project: "korva", Agent: "claude", CreatedAt: ts,
		}); err != nil {
			t.Fatalf("seed Save() error = %v", err)
		}
	}

	cutoff := now.AddDate(0, 0, -30)
	removed, err := s.PurgeInteractionsOlderThan(cutoff)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	remaining, _ := s.CountInteractions(InteractionFilters{})
	if remaining != 3 {
		t.Errorf("remaining = %d, want 3", remaining)
	}
}
