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
func Filter(text string, extraPatterns []string) string {
	// Apply built-in patterns
	result := text
	for _, p := range builtinPatterns {
		result = p.ReplaceAllString(result, "$1"+redacted)
	}

	// Apply <private>...</private> tag removal (multiline)
	result = privateTagPattern.ReplaceAllString(result, redacted)

	// Apply Bearer token removal
	result = bearerPattern.ReplaceAllString(result, "Bearer "+redacted)

	// Apply extra patterns from config
	for _, extra := range extraPatterns {
		p, err := compileExtraPattern(extra)
		if err == nil {
			result = p.ReplaceAllString(result, "$1"+redacted)
		}
	}

	return result
}

// ContainsSensitiveData returns true if text contains patterns that should be redacted.
// Used to warn users before saving observations.
func ContainsSensitiveData(text string) bool {
	for _, p := range builtinPatterns {
		if p.MatchString(text) {
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

// builtinPatterns matches common secret key=value or key: value formats.
// Each pattern has a capturing group $1 for the key name so the redaction
// shows which field was removed: "password=[REDACTED]"
var builtinPatterns = []*regexp.Regexp{
	// password=xxx, password: xxx, PASSWORD=xxx
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)\S+`),
	// passwd=xxx
	regexp.MustCompile(`(?i)(passwd\s*[:=]\s*)\S+`),
	// pwd=xxx
	regexp.MustCompile(`(?i)(pwd\s*[:=]\s*)\S+`),
	// token=xxx, TOKEN=xxx
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)\S+`),
	// secret=xxx
	regexp.MustCompile(`(?i)(secret\s*[:=]\s*)\S+`),
	// api_key=xxx, apiKey=xxx, API_KEY=xxx
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)\S+`),
	// ROLE_ID=xxx (HashiCorp Vault)
	regexp.MustCompile(`(?i)(ROLE_ID\s*[:=]\s*)\S+`),
	// SECRET_ID=xxx (HashiCorp Vault)
	regexp.MustCompile(`(?i)(SECRET_ID\s*[:=]\s*)\S+`),
	// private_key=xxx
	regexp.MustCompile(`(?i)(private[_-]?key\s*[:=]\s*)\S+`),
	// client_secret=xxx
	regexp.MustCompile(`(?i)(client[_-]?secret\s*[:=]\s*)\S+`),
}

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
