package email

import (
	"fmt"
	"time"
)

// InviteMessage builds the transactional email sent to a new team member when
// an admin creates an invite token via POST /admin/teams/{id}/invites.
//
// The plaintext token is embedded directly in the message because it is a
// one-time secret — the recipient runs `korva auth redeem <token>` once.
func InviteMessage(to, teamName, token, expiresAt string) Message {
	expiry := formatExpiry(expiresAt)
	cmd := fmt.Sprintf("korva auth redeem %s", token)

	text := fmt.Sprintf(
		"You've been invited to join %s on Korva for Teams.\n\n"+
			"Run this command in your terminal to activate your session:\n\n"+
			"    %s\n\n"+
			"This invite expires on %s.\n\n"+
			"Don't have Korva CLI? Install it:\n"+
			"    curl -fsSL https://korva.dev/install | bash\n\n"+
			"Not expecting this email? You can safely ignore it.\n",
		teamName, cmd, expiry)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#0d1117;font-family:ui-sans-serif,system-ui,-apple-system,sans-serif">
  <table width="100%%" cellpadding="0" cellspacing="0" style="padding:40px 16px">
    <tr><td align="center">
      <table width="100%%" cellpadding="0" cellspacing="0" style="max-width:520px">

        <!-- Header -->
        <tr><td style="padding-bottom:24px">
          <span style="display:inline-flex;align-items:center;gap:8px">
            <span style="display:inline-block;width:28px;height:28px;border-radius:6px;background:#f0883e30;border:1px solid #f0883e50;text-align:center;line-height:28px;font-size:14px">⚡</span>
            <span style="color:#e6edf3;font-weight:600;font-size:15px">Korva</span>
          </span>
        </td></tr>

        <!-- Card -->
        <tr><td style="background:#161b22;border:1px solid #21262d;border-radius:12px;padding:32px">
          <h1 style="margin:0 0 8px;color:#e6edf3;font-size:20px;font-weight:600">
            You've been invited to %s
          </h1>
          <p style="margin:0 0 24px;color:#8b949e;font-size:14px;line-height:1.5">
            Your team is using Korva to share architecture decisions,
            patterns and context across AI sessions.
          </p>

          <p style="margin:0 0 8px;color:#8b949e;font-size:13px">
            Run this command in your terminal:
          </p>
          <div style="background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:14px 16px;font-family:ui-monospace,SFMono-Regular,monospace;font-size:13px;color:#58a6ff;word-break:break-all">
            %s
          </div>

          <p style="margin:20px 0 0;color:#484f58;font-size:12px;line-height:1.5">
            This invite expires <strong style="color:#8b949e">%s</strong>.
            Don't have Korva CLI?
            <a href="https://korva.dev/install" style="color:#388bfd;text-decoration:none">Install it here</a>.
          </p>
        </td></tr>

        <!-- Footer -->
        <tr><td style="padding-top:20px;text-align:center">
          <p style="margin:0;color:#484f58;font-size:11px">
            Not expecting this email? You can safely ignore it.<br>
            Sent by <a href="https://korva.dev" style="color:#388bfd;text-decoration:none">Korva for Teams</a>
          </p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>`, teamName, cmd, expiry)

	return Message{
		To:      to,
		Subject: fmt.Sprintf("You've been invited to %s on Korva", teamName),
		HTML:    html,
		Text:    text,
	}
}

// formatExpiry parses an RFC3339 timestamp and returns a human-readable date.
// Returns the raw string on parse failure so the caller always has a value.
func formatExpiry(expiresAt string) string {
	if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		return t.Local().Format("January 2, 2006")
	}
	return expiresAt
}
