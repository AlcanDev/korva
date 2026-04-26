package store

import (
	"fmt"
	"strings"
	"time"
)

// BuildReasoningHint generates a plain-language explanation of why an
// observation is relevant to a given search query, using only stored metadata.
//
// No NLP or external APIs are required — the hint is derived from:
//   - Observation type (decisions and patterns are high-confidence signals)
//   - Age (recent observations are more likely to still be accurate)
//   - Tags that match the query terms
//   - Phase annotations in tags (e.g. "sdd:apply" is actionable context)
//
// The hint is intentionally short (1-2 sentences) so it adds context without
// consuming excessive LLM tokens. Empty string means no notable reason.
func BuildReasoningHint(obs Observation, query string) string {
	var parts []string

	// Type signal: decisions and patterns carry high confidence.
	switch obs.Type {
	case TypeDecision:
		parts = append(parts, "Architectural decision")
	case TypePattern:
		parts = append(parts, "Reusable pattern")
	case TypeAntiPattern:
		parts = append(parts, "Anti-pattern to avoid")
	case TypeBugfix:
		parts = append(parts, "Bug fix record")
	case TypeLearning:
		parts = append(parts, "Learning note")
	}

	// Age signal: observations older than 6 months may be stale.
	age := time.Since(obs.CreatedAt)
	switch {
	case age < 7*24*time.Hour:
		parts = append(parts, "saved this week")
	case age < 30*24*time.Hour:
		parts = append(parts, "saved this month")
	case age > 180*24*time.Hour:
		parts = append(parts, fmt.Sprintf("saved %.0f months ago — verify still current", age.Hours()/720))
	}

	// Tag signal: SDD phase tags are especially useful for agents.
	for _, tag := range obs.Tags {
		if strings.HasPrefix(tag, "sdd:") {
			phase := strings.TrimPrefix(tag, "sdd:")
			parts = append(parts, fmt.Sprintf("from SDD %s phase", phase))
			break
		}
	}

	// Query match signal: tags that overlap with query terms reinforce relevance.
	if query != "" {
		lq := strings.ToLower(query)
		var matchedTags []string
		for _, tag := range obs.Tags {
			if strings.Contains(lq, strings.ToLower(tag)) || strings.Contains(strings.ToLower(tag), lq) {
				matchedTags = append(matchedTags, tag)
			}
		}
		if len(matchedTags) > 0 {
			parts = append(parts, fmt.Sprintf("tagged: %s", strings.Join(matchedTags, ", ")))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}
