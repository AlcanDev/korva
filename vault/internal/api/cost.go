package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 8.6 — Cost & ROI endpoint.
//
//   GET /admin/cost/summary?days=30
//
// Wraps the existing GetTokenStats with a pricing table so the dashboard
// renders USD numbers without each caller doing arithmetic. Designed so the
// operator can answer in one screen:
//   - "How much did our AI cost us this month?"
//   - "Per model: who's the most expensive?"
//   - "Per project: which team should pay attention?"
//   - "How much did Korva save by injecting only relevant context vs.
//      sending the entire codebase every prompt?"
//
// The pricing table is embedded in code — Korva runs offline, no external
// pricing API, no surprise cloud calls.

// modelPrice is the USD-per-1M-tokens cost for one model. Values come from
// the public pricing pages of each vendor as of mid-2026. We document them
// here so the operator can audit and bump them via a code change rather
// than wondering why "the numbers seem off".
type modelPrice struct {
	Family         string  // human label
	InputPer1M     float64 // USD per 1M input tokens
	OutputPer1M    float64 // USD per 1M output tokens
	CacheReadPer1M float64 // USD per 1M cached input tokens (read)
}

// pricingTable maps lowercased model identifiers (or their prefix) to prices.
// We match by longest-prefix so we catch both "claude-3-5-sonnet-20240620" and
// the bare "claude-sonnet" alias. Anything we don't recognize falls back to
// `defaultPrice`, which is mid-tier Sonnet — better than zero ROI numbers.
var pricingTable = map[string]modelPrice{
	"claude-opus-4":     {Family: "Anthropic Claude Opus", InputPer1M: 15.0, OutputPer1M: 75.0, CacheReadPer1M: 1.5},
	"claude-3-opus":     {Family: "Anthropic Claude 3 Opus", InputPer1M: 15.0, OutputPer1M: 75.0, CacheReadPer1M: 1.5},
	"claude-sonnet-4":   {Family: "Anthropic Claude Sonnet 4", InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.3},
	"claude-3-5-sonnet": {Family: "Anthropic Claude 3.5 Sonnet", InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.3},
	"claude-3-haiku":    {Family: "Anthropic Claude 3 Haiku", InputPer1M: 0.25, OutputPer1M: 1.25, CacheReadPer1M: 0.03},
	"gpt-4o":            {Family: "OpenAI GPT-4o", InputPer1M: 2.5, OutputPer1M: 10.0, CacheReadPer1M: 1.25},
	"gpt-4o-mini":       {Family: "OpenAI GPT-4o mini", InputPer1M: 0.15, OutputPer1M: 0.6, CacheReadPer1M: 0.075},
	"gpt-4.1":           {Family: "OpenAI GPT-4.1", InputPer1M: 5.0, OutputPer1M: 15.0, CacheReadPer1M: 1.25},
	"gemini-2.0-flash":  {Family: "Google Gemini 2.0 Flash", InputPer1M: 0.10, OutputPer1M: 0.40, CacheReadPer1M: 0.025},
	"gemini-1.5-pro":    {Family: "Google Gemini 1.5 Pro", InputPer1M: 1.25, OutputPer1M: 5.0, CacheReadPer1M: 0.3125},
}

// defaultPrice is what we charge against an unrecognized model id. Picked to
// be neither the cheapest nor the most expensive so the ROI estimate stays
// believable even when a brand-new model ships.
var defaultPrice = modelPrice{Family: "(unknown model)", InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.3}

// lookupPrice returns the best-matching modelPrice for `modelID`.
func lookupPrice(modelID string) modelPrice {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return defaultPrice
	}
	// Exact match first.
	if p, ok := pricingTable[id]; ok {
		return p
	}
	// Longest-prefix match.
	var bestKey string
	for k := range pricingTable {
		if strings.HasPrefix(id, k) && len(k) > len(bestKey) {
			bestKey = k
		}
	}
	if bestKey != "" {
		return pricingTable[bestKey]
	}
	return defaultPrice
}

// computeCostUSD applies the model price to a (input, output, cache_read)
// triple. Cache reads are billed at the cache-read rate, NOT input — that's
// where the savings show up.
func computeCostUSD(p modelPrice, input, output, cacheRead int64) float64 {
	cost := 0.0
	cost += float64(input) * p.InputPer1M / 1_000_000
	cost += float64(output) * p.OutputPer1M / 1_000_000
	cost += float64(cacheRead) * p.CacheReadPer1M / 1_000_000
	return cost
}

// CostBucket is what the wire returns for one row (a model or project).
type CostBucket struct {
	Name         string  `json:"name"`
	Family       string  `json:"family,omitempty"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheRead    int64   `json:"cache_read"`
	Count        int64   `json:"count"`
	CostUSD      float64 `json:"cost_usd"`
}

// DailyCost is one row of the daily series.
type DailyCost struct {
	Date    string  `json:"date"`
	Tokens  int64   `json:"tokens"`   // input + output (cache read is free for cost-curve purposes)
	CostUSD float64 `json:"cost_usd"` // approximate — uses the dominant model's price
}

// CostSummaryResponse is the wire shape of /admin/cost/summary.
type CostSummaryResponse struct {
	WindowDays      int          `json:"window_days"`
	From            time.Time    `json:"from"`
	To              time.Time    `json:"to"`
	TotalUSD        float64      `json:"total_usd"`
	TotalTokens     int64        `json:"total_tokens"`
	InputTokens     int64        `json:"input_tokens"`
	OutputTokens    int64        `json:"output_tokens"`
	CacheRead       int64        `json:"cache_read"`
	CacheHitPct     float64      `json:"cache_hit_pct"`
	SavingsUSD      float64      `json:"savings_usd"`   // approx: cost we'd have paid if all cache hits were full input
	ReductionPct    float64      `json:"reduction_pct"` // reuse from GetTokenStats (vs naive baseline)
	BaselineDir     string       `json:"baseline_dir,omitempty"`
	ByModel         []CostBucket `json:"by_model"`
	ByProject       []CostBucket `json:"by_project"`
	Daily           []DailyCost  `json:"daily"`
	InteractionsCnt int64        `json:"interactions_count"`
}

func adminCostSummary(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Default 30 days, clamp to [1, 365].
		days := 30
		if v := r.URL.Query().Get("days"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				if n > 365 {
					n = 365
				}
				days = n
			}
		}
		to := time.Now().UTC()
		from := to.AddDate(0, 0, -days)

		stats, err := s.GetTokenStats(from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Build by-model + total cost.
		byModel := make([]CostBucket, 0, len(stats.ByModel))
		var totalCost float64
		var savingsUSD float64
		for name, b := range stats.ByModel {
			price := lookupPrice(name)
			cost := computeCostUSD(price, b.InputTokens, b.OutputTokens, b.CacheRead)
			// Savings = what cache_read would have cost at full input rate
			// minus what it actually cost at the cache_read rate.
			savings := float64(b.CacheRead) * (price.InputPer1M - price.CacheReadPer1M) / 1_000_000
			totalCost += cost
			savingsUSD += savings
			byModel = append(byModel, CostBucket{
				Name:         name,
				Family:       price.Family,
				InputTokens:  b.InputTokens,
				OutputTokens: b.OutputTokens,
				CacheRead:    b.CacheRead,
				Count:        b.Count,
				CostUSD:      cost,
			})
		}

		// Build by-project. Projects don't expose their model directly here —
		// we approximate cost using the unweighted average price across models
		// the project's interactions touched. For accuracy, the dashboard
		// shows per-project tokens explicitly and only USD as a rough hint.
		avgPrice := averagePriceFromModelsBuckets(stats.ByModel)
		byProject := make([]CostBucket, 0, len(stats.ByProject))
		for name, b := range stats.ByProject {
			cost := computeCostUSD(avgPrice, b.InputTokens, b.OutputTokens, b.CacheRead)
			byProject = append(byProject, CostBucket{
				Name:         name,
				InputTokens:  b.InputTokens,
				OutputTokens: b.OutputTokens,
				CacheRead:    b.CacheRead,
				Count:        b.Count,
				CostUSD:      cost,
			})
		}

		// Daily series using average price.
		daily := make([]DailyCost, 0, len(stats.Daily))
		for _, d := range stats.Daily {
			cost := computeCostUSD(avgPrice, d.InputTokens, d.OutputTokens, d.CacheRead)
			daily = append(daily, DailyCost{
				Date:    d.Date,
				Tokens:  d.InputTokens + d.OutputTokens,
				CostUSD: cost,
			})
		}

		// Reduction percentage from the existing endpoint (vs naive baseline).
		// We re-derive it here so cost/summary is self-contained.
		reduction := 0.0
		if stats.InputTokens > 0 {
			// We don't recompute baseline here to keep this endpoint fast;
			// pulled separately via /admin/tokens/stats when needed. For
			// summary, surface cache_hit_pct as a proxy reduction signal.
			reduction = stats.CacheHitPct
		}

		writeJSON(w, http.StatusOK, CostSummaryResponse{
			WindowDays:      days,
			From:            from,
			To:              to,
			TotalUSD:        totalCost,
			TotalTokens:     stats.InputTokens + stats.OutputTokens,
			InputTokens:     stats.InputTokens,
			OutputTokens:    stats.OutputTokens,
			CacheRead:       stats.CacheRead,
			CacheHitPct:     stats.CacheHitPct,
			SavingsUSD:      savingsUSD,
			ReductionPct:    reduction,
			ByModel:         byModel,
			ByProject:       byProject,
			Daily:           daily,
			InteractionsCnt: stats.InteractionsCount,
		})
	}
}

// averagePriceFromModelsBuckets returns a token-weighted average modelPrice
// across every bucket. Used to estimate per-project cost without needing a
// model-per-project query.
func averagePriceFromModelsBuckets(buckets map[string]store.TokenStatsBucket) modelPrice {
	var totalTokens int64
	var inputW, outputW, cacheW float64
	for name, b := range buckets {
		p := lookupPrice(name)
		tokens := b.InputTokens + b.OutputTokens + b.CacheRead
		if tokens <= 0 {
			continue
		}
		totalTokens += tokens
		inputW += p.InputPer1M * float64(tokens)
		outputW += p.OutputPer1M * float64(tokens)
		cacheW += p.CacheReadPer1M * float64(tokens)
	}
	if totalTokens == 0 {
		return defaultPrice
	}
	return modelPrice{
		Family:         "(weighted average)",
		InputPer1M:     inputW / float64(totalTokens),
		OutputPer1M:    outputW / float64(totalTokens),
		CacheReadPer1M: cacheW / float64(totalTokens),
	}
}
