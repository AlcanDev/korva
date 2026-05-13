package privacy

import (
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// Phase 9.1 — Instrumented Filter + global redaction counters.
//
// Filter() (the legacy entry point) stays unchanged for callers that just
// want the redacted string. FilterReport() returns both the string and a
// structured RedactionReport so the dashboard can show "we removed X
// passwords, Y tokens, Z bearer headers" in real time.
//
// A package-level counter tallies every redaction process-wide. Anything
// that wants to render the cumulative numbers reads RedactionStats(); the
// counters are atomic so the read path never blocks the filter hot loop.

// RedactionType classifies what kind of secret was removed. Each pattern
// in the filter maps to one of these so the dashboard groups redactions
// by category instead of dumping a regex string.
type RedactionType string

const (
	RedactionPassword     RedactionType = "password"
	RedactionToken        RedactionType = "token"
	RedactionSecret       RedactionType = "secret"
	RedactionAPIKey       RedactionType = "api_key"
	RedactionPrivateKey   RedactionType = "private_key"
	RedactionClientSecret RedactionType = "client_secret"
	RedactionRoleID       RedactionType = "vault_role_id"
	RedactionSecretID     RedactionType = "vault_secret_id"
	RedactionBearer       RedactionType = "bearer_token"
	RedactionPrivateTag   RedactionType = "private_tag"
	RedactionCustom       RedactionType = "custom_keyword"
)

// RedactionEntry describes one redaction event.
type RedactionEntry struct {
	Type RedactionType `json:"type"`
	// CharsRemoved counts characters dropped (sum of all match lengths
	// for this type in the input). Useful for the "we hid 4.2 KiB of
	// secrets this month" headline.
	CharsRemoved int `json:"chars_removed"`
}

// RedactionReport is what FilterReport returns alongside the redacted text.
type RedactionReport struct {
	// Entries lists every redaction type that fired and how many chars
	// each removed. Aggregated per-type so the JSON is small.
	Entries []RedactionEntry `json:"entries"`
	// TotalCharsRemoved is the sum of CharsRemoved across Entries.
	TotalCharsRemoved int `json:"total_chars_removed"`
}

// builtinPatternsTyped is the same list as builtinPatterns but each pattern
// carries its classification. Keeping them as two slices would risk drift;
// instead we expose this typed slice and derive `builtinPatterns` from it.
var builtinPatternsTyped = []struct {
	Type RedactionType
	Re   *regexp.Regexp
}{
	{RedactionPassword, regexp.MustCompile(`(?i)(password\s*[:=]\s*)\S+`)},
	{RedactionPassword, regexp.MustCompile(`(?i)(passwd\s*[:=]\s*)\S+`)},
	{RedactionPassword, regexp.MustCompile(`(?i)(pwd\s*[:=]\s*)\S+`)},
	{RedactionToken, regexp.MustCompile(`(?i)(token\s*[:=]\s*)\S+`)},
	{RedactionSecret, regexp.MustCompile(`(?i)(secret\s*[:=]\s*)\S+`)},
	{RedactionAPIKey, regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)\S+`)},
	{RedactionRoleID, regexp.MustCompile(`(?i)(ROLE_ID\s*[:=]\s*)\S+`)},
	{RedactionSecretID, regexp.MustCompile(`(?i)(SECRET_ID\s*[:=]\s*)\S+`)},
	{RedactionPrivateKey, regexp.MustCompile(`(?i)(private[_-]?key\s*[:=]\s*)\S+`)},
	{RedactionClientSecret, regexp.MustCompile(`(?i)(client[_-]?secret\s*[:=]\s*)\S+`)},
}

// FilterReport applies the redaction pipeline and returns both the cleaned
// text and a report of what was removed. The report's entries are
// aggregated per RedactionType (sum of chars across all matches of that
// type), so the wire payload is small.
//
// Side effect: increments the package-level counters. Callers that don't
// want telemetry should keep using the legacy Filter() entry point — which
// is now just a thin wrapper around FilterReport that drops the report.
func FilterReport(text string, extraPatterns []string) (string, RedactionReport) {
	result := text
	tally := make(map[RedactionType]int)

	// Char-cost heuristic: total length of the matched substring (key + value
	// + delimiter). The exact "value length" requires re-parsing $1; the
	// substring length is a faithful proxy of how much sensitive material
	// was scanned over, and produces positive numbers for every match.
	for _, p := range builtinPatternsTyped {
		matches := p.Re.FindAllString(result, -1)
		for _, m := range matches {
			tally[p.Type] += len(m)
		}
		result = p.Re.ReplaceAllString(result, "$1"+redacted)
	}

	// <private>…</private> tags — entire block goes.
	for _, m := range privateTagPattern.FindAllString(result, -1) {
		tally[RedactionPrivateTag] += len(m)
	}
	result = privateTagPattern.ReplaceAllString(result, redacted)

	// Bearer tokens — keep "Bearer " prefix.
	for _, m := range bearerPattern.FindAllString(result, -1) {
		tally[RedactionBearer] += len(m)
	}
	result = bearerPattern.ReplaceAllString(result, "Bearer "+redacted)

	// Custom keyword patterns from korva.config.json.
	for _, extra := range extraPatterns {
		p, err := compileExtraPattern(extra)
		if err != nil {
			continue
		}
		for _, m := range p.FindAllString(result, -1) {
			tally[RedactionCustom] += len(m)
		}
		result = p.ReplaceAllString(result, "$1"+redacted)
	}

	// Build the report + update the global counters.
	report := RedactionReport{}
	for t, chars := range tally {
		if chars > 0 {
			report.Entries = append(report.Entries, RedactionEntry{Type: t, CharsRemoved: chars})
			report.TotalCharsRemoved += chars
		}
	}
	if report.TotalCharsRemoved > 0 {
		incrementCounters(report)
	}
	return result, report
}

// ── Global counters ────────────────────────────────────────────────────────

// counters track cumulative redaction activity for the dashboard. Reads and
// writes go through atomics so the hot path never blocks.
type redactionCounters struct {
	totalEvents       atomic.Int64
	totalCharsRemoved atomic.Int64
	byType            sync.Map // RedactionType -> *atomic.Int64
	startedAt         time.Time
}

var counters = &redactionCounters{startedAt: time.Now().UTC()}

func incrementCounters(r RedactionReport) {
	counters.totalEvents.Add(1)
	counters.totalCharsRemoved.Add(int64(r.TotalCharsRemoved))
	for _, e := range r.Entries {
		v, _ := counters.byType.LoadOrStore(e.Type, &atomic.Int64{})
		v.(*atomic.Int64).Add(int64(e.CharsRemoved))
	}
}

// RedactionStatsSnapshot is the point-in-time read of the global counters.
type RedactionStatsSnapshot struct {
	TotalEvents       int64                   `json:"total_events"`
	TotalCharsRemoved int64                   `json:"total_chars_removed"`
	ByType            map[RedactionType]int64 `json:"by_type"`
	SinceUnix         int64                   `json:"since_unix"`
	Since             time.Time               `json:"since"`
}

// RedactionStats returns a snapshot suitable for serialisation. Safe to call
// concurrently with FilterReport.
func RedactionStats() RedactionStatsSnapshot {
	out := RedactionStatsSnapshot{
		TotalEvents:       counters.totalEvents.Load(),
		TotalCharsRemoved: counters.totalCharsRemoved.Load(),
		Since:             counters.startedAt,
		SinceUnix:         counters.startedAt.Unix(),
		ByType:            map[RedactionType]int64{},
	}
	counters.byType.Range(func(k, v any) bool {
		out.ByType[k.(RedactionType)] = v.(*atomic.Int64).Load()
		return true
	})
	return out
}

// ResetRedactionStats clears the counters. Used by tests; the production
// process never resets — counters are process-lifetime cumulative.
func ResetRedactionStats() {
	counters.totalEvents.Store(0)
	counters.totalCharsRemoved.Store(0)
	counters.byType.Range(func(k, _ any) bool {
		counters.byType.Delete(k)
		return true
	})
	counters.startedAt = time.Now().UTC()
}

