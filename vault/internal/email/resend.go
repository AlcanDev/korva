package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// resendAPIEndpoint is the Resend v1 send endpoint.
// Exposed as a var (not const) so tests can redirect to an httptest server.
var resendAPIEndpoint = "https://api.resend.com/emails"

// resendMailer sends email via the Resend HTTP API.
// https://resend.com/docs/api-reference/emails/send-email
type resendMailer struct {
	apiKey   string
	from     string
	fromName string

	// httpClient is nil in production (uses a fresh client per call).
	// Tests inject a custom client to redirect requests to an httptest server.
	httpClient *http.Client
}

func (m *resendMailer) Configured() bool { return true }

func (m *resendMailer) Send(ctx context.Context, msg Message) error {
	fromAddr := m.from
	if m.fromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", m.fromName, m.from)
	}

	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html,omitempty"`
		Text    string   `json:"text,omitempty"`
	}{
		From:    fromAddr,
		To:      []string{msg.To},
		Subject: msg.Subject,
		HTML:    msg.HTML,
		Text:    msg.Text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := m.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("email: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var apiErr struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&apiErr) //nolint:errcheck — best-effort decode
		if apiErr.Message != "" {
			return fmt.Errorf("email: resend %d: %s", resp.StatusCode, apiErr.Message)
		}
		return fmt.Errorf("email: resend returned status %d", resp.StatusCode)
	}

	return nil
}
