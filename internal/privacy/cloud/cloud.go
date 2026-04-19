// Package cloud implements the second-layer privacy filter applied to
// observations BEFORE they are sent to the Korva Hive (community cloud).
//
// It is stricter than internal/privacy.Filter and follows a default-deny
// posture: when in doubt, the observation is rejected and never leaves the
// user's machine. Identifiers (project/team/author) are anonymized with an
// HMAC-SHA256 keyed on the install salt — stable per-install but not
// reversible without the salt.
//
// The chain is:
//
//	internal/privacy.Filter   →  redacts secrets in-place (always applied at Save)
//	internal/privacy/cloud    →  decides whether the redacted observation may go to cloud
package cloud

import (
	"strings"
	"time"
)

// Input is the observation payload as it lives in the local Vault
// (already passed through internal/privacy.Filter).
type Input struct {
	ID        string
	Type      string
	Title     string
	Content   string
	Tags      []string
	Project   string
	Team      string
	Author    string
	CreatedAt time.Time
}

// Output is the anonymized payload safe to send to the Korva Hive.
// It deliberately omits session_id, country, and any raw identifier.
type Output struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	ProjectHash string    `json:"project_hash,omitempty"`
	TeamHash    string    `json:"team_hash,omitempty"`
	AuthorHash  string    `json:"author_hash,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Decision is the outcome of Filter.Process.
type Decision int

const (
	// Accept means the Output is safe to send to Hive.
	Accept Decision = iota
	// Reject means the observation must NOT leave the local machine.
	Reject
)

// Hard-blocked types: enterprise-only content that must never reach Hive,
// regardless of allowlist configuration.
var hardBlockedTypes = map[string]bool{
	"skill":      true,
	"credential": true,
	"incident":   true,
	"private":    true,
}

// Maximum content length post-filter. Anything bigger is rejected
// to limit blast radius of leaks and keep Hive batches small.
const defaultMaxContent = 8 * 1024

// Filter decides whether observations may be sent to Hive.
type Filter struct {
	allowed     map[string]bool
	salt        []byte
	maxContent  int
	piiDetector *piiDetector
}

// New builds a Filter.
//
//	allowedTypes  — observation types eligible for Hive (e.g. ["pattern","decision","learning"])
//	installSalt   — opaque per-installation salt (use install.id from internal/identity)
func New(allowedTypes []string, installSalt string) *Filter {
	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" && !hardBlockedTypes[t] {
			allowed[t] = true
		}
	}
	return &Filter{
		allowed:     allowed,
		salt:        []byte(installSalt),
		maxContent:  defaultMaxContent,
		piiDetector: defaultPIIDetector(),
	}
}

// Process evaluates an Input and returns either an anonymized Output ready
// for Hive (Decision = Accept) or the rejection reason (Decision = Reject).
//
// The reason is human-readable and suitable for the cloud_outbox.last_error
// column, but it must NEVER include the offending content.
func (f *Filter) Process(in Input) (Output, Decision, string) {
	t := strings.ToLower(strings.TrimSpace(in.Type))

	if hardBlockedTypes[t] {
		return Output{}, Reject, "type is hard-blocked from cloud sync: " + t
	}
	if len(f.allowed) > 0 && !f.allowed[t] {
		return Output{}, Reject, "type not in cloud allowlist: " + t
	}
	if strings.TrimSpace(in.Content) == "" {
		return Output{}, Reject, "empty content"
	}
	if containsPrivateMarker(in.Title) || containsPrivateMarker(in.Content) {
		return Output{}, Reject, "content carries unresolved <private> marker"
	}
	// Reject oversized payloads BEFORE scrubbing, so a maliciously crafted
	// blob (e.g. all-hex) cannot be collapsed by PII placeholders into a
	// passing tiny string.
	if len(in.Content) > f.maxContent || len(in.Title) > f.maxContent {
		return Output{}, Reject, "content exceeds cloud size cap"
	}

	cleanedTitle, titleHits := f.piiDetector.scrub(in.Title)
	cleanedContent, contentHits := f.piiDetector.scrub(in.Content)

	// Default deny: redaction succeeded but the original carried 2+ PII matches —
	// treat as risky and bail. One isolated match is acceptable (regex false-positives are common).
	if titleHits+contentHits >= 2 {
		return Output{}, Reject, "content tripped multiple PII patterns"
	}

	out := Output{
		ID:          in.ID,
		Type:        t,
		Title:       cleanedTitle,
		Content:     cleanedContent,
		Tags:        sanitizeTags(in.Tags),
		ProjectHash: hashField(in.Project, f.salt),
		TeamHash:    hashField(in.Team, f.salt),
		AuthorHash:  hashField(in.Author, f.salt),
		CreatedAt:   in.CreatedAt.UTC(),
	}
	return out, Accept, ""
}

func containsPrivateMarker(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<private>") || strings.Contains(lower, "</private>")
}

func sanitizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t == "" || len(t) > 64 {
			continue
		}
		out = append(out, t)
	}
	return out
}
