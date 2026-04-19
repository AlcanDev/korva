package email

import "context"

// noopMailer is a no-operation Mailer returned when email is not configured.
// All Send calls succeed silently so callers need no nil-checks.
type noopMailer struct{}

func (n *noopMailer) Send(_ context.Context, _ Message) error { return nil }
func (n *noopMailer) Configured() bool                        { return false }
