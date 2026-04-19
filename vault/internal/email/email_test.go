package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- NoopMailer ---

func TestNoopMailer_NotConfigured(t *testing.T) {
	m := &noopMailer{}
	if m.Configured() {
		t.Fatal("noop mailer should not be configured")
	}
}

func TestNoopMailer_SendSucceeds(t *testing.T) {
	m := &noopMailer{}
	err := m.Send(context.Background(), Message{To: "a@b.com", Subject: "test"})
	if err != nil {
		t.Fatalf("noop mailer Send should never error, got: %v", err)
	}
}

// --- NewFromEnv ---

func TestNewFromEnv_NoVars_ReturnsNoop(t *testing.T) {
	// Environment variables are intentionally not set in unit tests.
	m := NewFromEnv()
	if m.Configured() {
		t.Skip("KORVA_EMAIL_API_KEY and KORVA_EMAIL_FROM are set in this environment — skipping")
	}
}

// --- resendMailer ---

// roundTripFunc lets tests intercept HTTP requests without changing the endpoint URL.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestResendMailer(srv *httptest.Server) *resendMailer {
	return &resendMailer{
		apiKey:   "test-key-abc",
		from:     "noreply@acme.com",
		fromName: "Acme Team",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				// Redirect all requests to the test server.
				req2 := req.Clone(req.Context())
				req2.URL.Scheme = "http"
				req2.URL.Host = srv.Listener.Addr().String()
				req2.URL.Path = "/emails"
				return http.DefaultTransport.RoundTrip(req2)
			}),
		},
	}
}

func TestResendMailer_Configured(t *testing.T) {
	m := &resendMailer{apiKey: "k", from: "a@b.com"}
	if !m.Configured() {
		t.Fatal("resendMailer should be configured")
	}
}

func TestResendMailer_Send_Success(t *testing.T) {
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key-abc" {
			t.Errorf("unexpected Authorization: %q", auth)
		}
		json.NewDecoder(r.Body).Decode(&received) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "email-id-123"}) //nolint:errcheck
	}))
	defer srv.Close()

	m := newTestResendMailer(srv)
	err := m.Send(context.Background(), Message{
		To:      "alice@corp.com",
		Subject: "Test invite",
		HTML:    "<p>Hello</p>",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if received["subject"] != "Test invite" {
		t.Errorf("subject mismatch: %v", received["subject"])
	}
	// Sender should be formatted as "Name <address>"
	if got, _ := received["from"].(string); !strings.Contains(got, "Acme Team") {
		t.Errorf("from field missing name: %q", got)
	}
}

func TestResendMailer_Send_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"name":    "validation_error",
			"message": "invalid from address",
		})
	}))
	defer srv.Close()

	m := newTestResendMailer(srv)
	err := m.Send(context.Background(), Message{To: "x@y.com", Subject: "Test"})
	if err == nil {
		t.Fatal("expected error on 422, got nil")
	}
	if !strings.Contains(err.Error(), "invalid from address") {
		t.Errorf("error should contain API message, got: %v", err)
	}
}

func TestResendMailer_Send_NetworkError(t *testing.T) {
	m := &resendMailer{
		apiKey: "k",
		from:   "a@b.com",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, &networkErr{"connection refused"}
			}),
		},
	}
	err := m.Send(context.Background(), Message{To: "x@y.com", Subject: "Test"})
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}
}

type networkErr struct{ msg string }

func (e *networkErr) Error() string { return e.msg }

// --- Templates ---

func TestInviteMessage_Fields(t *testing.T) {
	msg := InviteMessage("alice@corp.com", "Acme Corp", "secrettoken42", "2026-12-31T00:00:00Z")

	if msg.To != "alice@corp.com" {
		t.Errorf("To: want %q, got %q", "alice@corp.com", msg.To)
	}
	if msg.Subject == "" {
		t.Error("Subject must not be empty")
	}
	if !strings.Contains(msg.Subject, "Acme Corp") {
		t.Errorf("Subject should mention team name, got: %q", msg.Subject)
	}
	if !strings.Contains(msg.Text, "secrettoken42") {
		t.Error("plain-text body must contain the token")
	}
	if !strings.Contains(msg.HTML, "secrettoken42") {
		t.Error("HTML body must contain the token")
	}
	if !strings.Contains(msg.Text, "Acme Corp") {
		t.Error("plain-text body must contain the team name")
	}
	if !strings.Contains(msg.HTML, "Acme Corp") {
		t.Error("HTML body must contain the team name")
	}
}

func TestInviteMessage_ExpiryFormatting(t *testing.T) {
	msg := InviteMessage("a@b.com", "Team", "tok", "2026-07-04T12:00:00Z")
	// Should render as a human date, not raw RFC3339
	if strings.Contains(msg.Text, "T12:00:00Z") {
		t.Error("plain-text expiry should be formatted, not raw RFC3339")
	}
}

func TestInviteMessage_BadExpiry(t *testing.T) {
	// Should not panic on unparsable timestamp
	msg := InviteMessage("a@b.com", "Team", "tok", "not-a-date")
	if msg.To == "" {
		t.Fatal("message should be built even with bad expiry")
	}
}

func TestFormatExpiry_Valid(t *testing.T) {
	got := formatExpiry("2026-01-15T00:00:00Z")
	if got == "" || got == "2026-01-15T00:00:00Z" {
		t.Errorf("expected human date, got: %q", got)
	}
}

func TestFormatExpiry_Invalid(t *testing.T) {
	raw := "bad-date"
	if got := formatExpiry(raw); got != raw {
		t.Errorf("invalid date should return raw value, got: %q", got)
	}
}
