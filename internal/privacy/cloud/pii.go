package cloud

import "regexp"

// piiDetector scrubs PII from text. Each pattern, when matched, is
// replaced by a placeholder; the number of replacements is reported
// so the caller can apply default-deny on suspicious payloads.
type piiDetector struct {
	patterns []piiPattern
}

type piiPattern struct {
	name        string
	re          *regexp.Regexp
	placeholder string
}

const piiRedacted = "[CLOUD_REDACTED]" //nolint:unused // kept for future configurable redaction

func defaultPIIDetector() *piiDetector {
	// Order matters: more specific patterns must run before generic ones,
	// otherwise a UUID gets shredded by the looser phone regex first.
	return &piiDetector{
		patterns: []piiPattern{
			{name: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`), placeholder: "[JWT]"},
			{name: "uuid", re: regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), placeholder: "[UUID]"},
			{name: "long_hex", re: regexp.MustCompile(`\b[A-Fa-f0-9]{32,}\b`), placeholder: "[HEX]"},
			{name: "email", re: regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`), placeholder: "[EMAIL]"},
			{name: "user_path_unix", re: regexp.MustCompile(`(?:/Users|/home)/[^/\s"']+`), placeholder: "[USER_PATH]"},
			{name: "user_path_win", re: regexp.MustCompile(`(?i)[a-z]:\\Users\\[^\\\s"']+`), placeholder: "[USER_PATH]"},
			{name: "ipv6", re: regexp.MustCompile(`\b(?:[A-Fa-f0-9]{1,4}:){7}[A-Fa-f0-9]{1,4}\b`), placeholder: "[IPV6]"},
			{name: "ipv4", re: regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\b`), placeholder: "[IPV4]"},
			// phone last + stricter: requires a country/area separator and ≥7 digits total
			{name: "phone", re: regexp.MustCompile(`(?:\+\d{1,3}[\s.\-]\d{2,4}|\(\d{2,4}\))[\s.\-]?\d{3,4}[\s.\-]?\d{3,4}\b`), placeholder: "[PHONE]"},
		},
	}
}

// scrub returns the text with PII replaced and the number of distinct
// patterns that matched (not the total replacement count).
func (d *piiDetector) scrub(text string) (string, int) {
	if text == "" {
		return "", 0
	}
	hits := 0
	out := text
	for _, p := range d.patterns {
		if p.re.MatchString(out) {
			out = p.re.ReplaceAllString(out, p.placeholder)
			hits++
		}
	}
	return out, hits
}
