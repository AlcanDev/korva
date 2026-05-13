package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestLookupPrice_KnownAndUnknownIDs(t *testing.T) {
	tests := []struct {
		id          string
		wantInput   float64
		wantFamily  string
	}{
		{"claude-3-5-sonnet-20240620", 3.0, "Anthropic Claude 3.5 Sonnet"},
		{"claude-opus-4-20251022", 15.0, "Anthropic Claude Opus"},
		{"gpt-4o-2024-08-06", 2.5, "OpenAI GPT-4o"},
		{"gpt-4o-mini", 0.15, "OpenAI GPT-4o mini"},
		{"gemini-2.0-flash-001", 0.10, "Google Gemini 2.0 Flash"},
		{"some-totally-new-model", 3.0, "(unknown model)"}, // defaultPrice
		{"", 3.0, "(unknown model)"},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got := lookupPrice(tc.id)
			if got.InputPer1M != tc.wantInput {
				t.Errorf("InputPer1M = %v, want %v", got.InputPer1M, tc.wantInput)
			}
			if got.Family != tc.wantFamily {
				t.Errorf("Family = %q, want %q", got.Family, tc.wantFamily)
			}
		})
	}
}

func TestComputeCostUSD_AppliesAllThreeRates(t *testing.T) {
	p := modelPrice{InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.3}
	// 1M input, 1M output, 1M cache_read.
	got := computeCostUSD(p, 1_000_000, 1_000_000, 1_000_000)
	want := 3.0 + 15.0 + 0.3
	if got != want {
		t.Errorf("cost = %v, want %v", got, want)
	}
}

func TestAdminCostSummary_AggregatesByModelAndProject(t *testing.T) {
	s := newAPITestStore(t)
	// Seed two interactions: one Sonnet on korva, one GPT-4o on vault-mcp.
	// We poke the store directly because the public Save path doesn't expose
	// every field, and this keeps the test focused on the aggregator.
	insertInteraction(t, s, "korva", "claude-3-5-sonnet", 100_000, 20_000, 80_000)
	insertInteraction(t, s, "vault-mcp", "gpt-4o", 50_000, 10_000, 40_000)

	req := httptest.NewRequest(http.MethodGet, "/admin/cost/summary?days=7", nil)
	rec := httptest.NewRecorder()
	adminCostSummary(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp CostSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WindowDays != 7 {
		t.Errorf("WindowDays = %d, want 7", resp.WindowDays)
	}
	if resp.InteractionsCnt != 2 {
		t.Errorf("InteractionsCnt = %d, want 2", resp.InteractionsCnt)
	}
	// Total input = 100k + 50k = 150k; output = 30k; cache_read = 120k.
	if resp.InputTokens != 150_000 {
		t.Errorf("InputTokens = %d, want 150000", resp.InputTokens)
	}
	if resp.OutputTokens != 30_000 {
		t.Errorf("OutputTokens = %d, want 30000", resp.OutputTokens)
	}
	if resp.CacheRead != 120_000 {
		t.Errorf("CacheRead = %d, want 120000", resp.CacheRead)
	}

	// Sonnet input 100k @ $3/M = $0.30 ; output 20k @ $15/M = $0.30 ;
	// cache_read 80k @ $0.30/M = $0.024 → total 0.624
	// GPT-4o input 50k @ $2.5/M = $0.125 ; output 10k @ $10/M = $0.10 ;
	// cache_read 40k @ $1.25/M = $0.05 → total 0.275
	// Grand total ≈ 0.899
	if resp.TotalUSD < 0.85 || resp.TotalUSD > 0.95 {
		t.Errorf("TotalUSD ≈ %v, expected ~0.899", resp.TotalUSD)
	}

	if len(resp.ByModel) != 2 {
		t.Errorf("ByModel buckets = %d, want 2", len(resp.ByModel))
	}
	if len(resp.ByProject) != 2 {
		t.Errorf("ByProject buckets = %d, want 2", len(resp.ByProject))
	}

	// Savings: Sonnet cache_read 80k * (3.0 - 0.3) / 1M = 0.216
	//          GPT-4o cache_read 40k * (2.5 - 1.25) / 1M = 0.05
	// → ≈ 0.266
	if resp.SavingsUSD < 0.20 || resp.SavingsUSD > 0.35 {
		t.Errorf("SavingsUSD ≈ %v, expected ~0.266", resp.SavingsUSD)
	}
}

func TestAdminCostSummary_ClampsDaysParam(t *testing.T) {
	s := newAPITestStore(t)
	tests := []struct {
		in   string
		want int
	}{
		{"", 30},     // default
		{"7", 7},     // explicit
		{"1000", 365}, // clamped to max
		{"abc", 30},  // unparsable → default
		{"-5", 30},   // non-positive → default
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			url := "/admin/cost/summary"
			if tc.in != "" {
				url += "?days=" + tc.in
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			adminCostSummary(s)(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			var resp CostSummaryResponse
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.WindowDays != tc.want {
				t.Errorf("WindowDays = %d, want %d", resp.WindowDays, tc.want)
			}
		})
	}
}

// insertInteraction is a test helper that pokes a raw interaction row into
// the store via SaveInteraction so GetTokenStats sees it.
func insertInteraction(t *testing.T, s *store.Store, project, model string, input, output, cacheRead int64) {
	t.Helper()
	rec := store.Interaction{
		Project:       project,
		Agent:         "test",
		Model:         model,
		InputTokens:   input,
		OutputTokens:  output,
		CacheRead:     cacheRead,
		PromptExcerpt: "test",
		Status:        "ok",
	}
	if _, err := s.SaveInteraction(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
}
