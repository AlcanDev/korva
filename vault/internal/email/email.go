// Package email provides a thin transactional email abstraction for Korva Vault.
//
// Configuration is read exclusively from environment variables so that API keys
// are never stored in korva.config.json or the database.
//
//	KORVA_EMAIL_PROVIDER   resend (default when API key is set)
//	KORVA_EMAIL_API_KEY    Resend API key  (re_...)
//	KORVA_EMAIL_FROM       sender address  (noreply@yourcompany.com)
//	KORVA_EMAIL_FROM_NAME  sender display name (optional)
//
// When KORVA_EMAIL_API_KEY or KORVA_EMAIL_FROM are not set, NewFromEnv returns a
// NoopMailer that silently succeeds. Email delivery is always best-effort — a
// send failure never blocks the HTTP response.
package email

import (
	"context"
	"os"
	"strings"
)

// Mailer is the single interface for sending transactional messages.
type Mailer interface {
	// Send dispatches msg. Returns nil on success or when not configured.
	Send(ctx context.Context, msg Message) error
	// Configured reports whether email delivery is set up on this instance.
	Configured() bool
}

// Message is a single email to dispatch.
type Message struct {
	To      string // single recipient address
	Subject string
	HTML    string // HTML body (preferred by clients)
	Text    string // plain-text fallback
}

// NewFromEnv reads configuration from environment variables and returns the
// appropriate Mailer implementation. Returns a NoopMailer when unconfigured.
func NewFromEnv() Mailer {
	apiKey := strings.TrimSpace(os.Getenv("KORVA_EMAIL_API_KEY"))
	from := strings.TrimSpace(os.Getenv("KORVA_EMAIL_FROM"))

	if apiKey == "" || from == "" {
		return &noopMailer{}
	}

	return &resendMailer{
		apiKey:   apiKey,
		from:     from,
		fromName: strings.TrimSpace(os.Getenv("KORVA_EMAIL_FROM_NAME")),
	}
}
