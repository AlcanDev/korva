package privacy

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

// Filter removes sensitive information from text before storing it in Vault.
// It handles:
//   - Common secret key=value patterns (password=xxx, token=xxx)
//   - Bearer tokens in Authorization headers
//   - Content wrapped in <private>...</private> tags
//   - Configurable extra patterns from korva.config.json
//
// Internally delegates to FilterReport (Phase 9.1) and drops the report.
// Use FilterReport directly when you need redaction telemetry.
func Filter(text string, extraPatterns []string) string {
	out, _ := FilterReport(text, extraPatterns)
	return out
}

// ContainsSensitiveData returns true if text contains patterns that should be redacted.
// Used to warn users before saving observations.
func ContainsSensitiveData(text string) bool {
	for _, p := range builtinPatternsTyped {
		if p.Re.MatchString(text) {
			return true
		}
	}
	if privateTagPattern.MatchString(text) {
		return true
	}
	if bearerPattern.MatchString(text) {
		return true
	}
	return false
}

// ContainsSensitiveData reuses the typed pattern list so adding a category
// to one path automatically widens the other.
//
// bearerPattern matches Authorization: Bearer <token>
var bearerPattern = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)

// privateTagPattern matches <private>...</private> (including multiline)
var privateTagPattern = regexp.MustCompile(`(?is)<private>.*?</private>`)

// compileExtraPattern builds a regex for a custom keyword pattern.
// For keyword "mySecret" it matches: mySecret=xxx, mySecret: xxx
func compileExtraPattern(keyword string) (*regexp.Regexp, error) {
	escaped := regexp.QuoteMeta(keyword)
	pattern := `(?i)(` + escaped + `\s*[:=]\s*)\S+`
	return regexp.Compile(pattern)
}

// StripPrivateTags removes <private>...</private> tags and their contents,
// returning only the surrounding text.
func StripPrivateTags(text string) string {
	return strings.TrimSpace(privateTagPattern.ReplaceAllString(text, ""))
}
