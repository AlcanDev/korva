package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/store"
)

// captureMailer records every Send call so tests can assert on what was
// emailed without spinning up Resend. Configured() returns true so handler
// code-paths that gate on it (e.g. dispatching) run.
type captureMailer struct {
	mu   sync.Mutex
	msgs []email.Message
}

func (m *captureMailer) Configured() bool { return true }
func (m *captureMailer) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return nil
}
func (m *captureMailer) last() (email.Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.msgs) == 0 {
		return email.Message{}, false
	}
	return m.msgs[len(m.msgs)-1], true
}
func (m *captureMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.msgs)
}

// otpTestEnv wires an in-memory store with one team + one member +
// captureMailer. The handlers we test (request/verify) don't go through
// the full router so license / admin middleware is irrelevant here.
type otpTestEnv struct {
	store  *store.Store
	mailer *captureMailer
	teamID string
	email  string
}

func newOTPTestEnv(t *testing.T) *otpTestEnv {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	db := s.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	teamID := "team-otp-001"
	if _, err := db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		teamID, "OTP Co", "owner@otp.co", now); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		"member-001", teamID, "alice@otp.co", "member", now); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return &otpTestEnv{
		store:  s,
		mailer: &captureMailer{},
		teamID: teamID,
		email:  "alice@otp.co",
	}
}

// doRequest fires /auth/otp/request with a JSON body and returns the recorder.
func (e *otpTestEnv) doRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/auth/otp/request", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	authOTPRequest(e.store, e.mailer)(rec, r)
	return rec
}

// doVerify fires /auth/otp/verify with a JSON body and returns the recorder.
func (e *otpTestEnv) doVerify(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/auth/otp/verify", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	authOTPVerify(e.store)(rec, r)
	return rec
}

// extractEmailedCode pulls the 6-digit code out of the most-recent OTPMessage.
// Fails the test when there's no message or the format changes.
func (e *otpTestEnv) extractEmailedCode(t *testing.T) string {
	t.Helper()
	msg, ok := e.mailer.last()
	if !ok {
		t.Fatal("no OTP email was sent")
	}
	// The plain-text body contains the code on its own indented line.
	// Pull the first 6-digit run.
	for _, line := range strings.Split(msg.Text, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == otpCodeDigits {
			ok := true
			for _, r := range trimmed {
				if r < '0' || r > '9' {
					ok = false
					break
				}
			}
			if ok {
				return trimmed
			}
		}
	}
	t.Fatalf("no 6-digit code found in email text:\n%s", msg.Text)
	return ""
}

func TestAuthOTPRequest_SendsEmailForKnownMember(t *testing.T) {
	env := newOTPTestEnv(t)
	rec := env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if env.mailer.count() != 1 {
		t.Errorf("emails sent = %d, want 1", env.mailer.count())
	}
	code := env.extractEmailedCode(t)
	if len(code) != otpCodeDigits {
		t.Errorf("code length = %d", len(code))
	}
}

func TestAuthOTPRequest_UnknownEmailStillReturns204(t *testing.T) {
	// Anti-enumeration: an attacker can't tell whether the address exists.
	env := newOTPTestEnv(t)
	rec := env.doRequest(t, `{"email":"ghost@nowhere.test"}`)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (no leak)", rec.Code)
	}
	if env.mailer.count() != 0 {
		t.Errorf("emails sent = %d, want 0", env.mailer.count())
	}
}

func TestAuthOTPRequest_MalformedBodyRejected(t *testing.T) {
	env := newOTPTestEnv(t)
	for _, body := range []string{
		``,
		`{}`,
		`{"email":""}`,
		`{"email":"not-an-email"}`,
		`{this is not json`,
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			rec := env.doRequest(t, body)
			if rec.Code < 400 {
				t.Errorf("body %q → status %d, want 4xx", body, rec.Code)
			}
		})
	}
}

func TestAuthOTPRequest_InvalidatesPreviousPendingCodes(t *testing.T) {
	env := newOTPTestEnv(t)
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	firstCode := env.extractEmailedCode(t)

	// Second request: first code should now be invalid.
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	secondCode := env.extractEmailedCode(t)

	if firstCode == secondCode {
		t.Fatal("expected a different code on the second request")
	}

	// Verifying with the first (now-invalidated) code must fail.
	rec := env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":%q}`, env.email, firstCode))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("verify-with-stale-code status = %d, want 401", rec.Code)
	}
}

func TestAuthOTPRequest_RateLimitTrips(t *testing.T) {
	env := newOTPTestEnv(t)
	// First N within the window should succeed.
	for i := 0; i < otpMaxIssuePerHour; i++ {
		rec := env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d unexpectedly failed: %d", i, rec.Code)
		}
	}
	// (N+1)-th must trip the limit.
	rec := env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status after limit = %d, want 429", rec.Code)
	}
}

func TestAuthOTPVerify_HappyPathMintsSession(t *testing.T) {
	env := newOTPTestEnv(t)
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	code := env.extractEmailedCode(t)

	rec := env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":%q}`, env.email, code))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["email"] != env.email {
		t.Errorf("response email = %q", resp["email"])
	}
	if resp["session_token"] == "" {
		t.Error("session_token missing")
	}
	if resp["team_id"] != env.teamID {
		t.Errorf("team_id = %q", resp["team_id"])
	}

	// The session token is persisted (hash matches) so /auth/me would work.
	sessionHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resp["session_token"])))
	var found int
	if err := env.store.DB().QueryRow(
		`SELECT COUNT(*) FROM member_sessions WHERE token_hash=?`, sessionHash).Scan(&found); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if found != 1 {
		t.Errorf("session count = %d, want 1", found)
	}

	// The OTP row is consumed.
	var consumed int
	_ = env.store.DB().QueryRow(
		`SELECT COUNT(*) FROM auth_otp_codes WHERE email=? AND consumed_at IS NOT NULL`,
		env.email).Scan(&consumed)
	if consumed != 1 {
		t.Errorf("consumed OTP count = %d, want 1", consumed)
	}
}

func TestAuthOTPVerify_WrongCode(t *testing.T) {
	env := newOTPTestEnv(t)
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))

	rec := env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":"000000"}`, env.email))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthOTPVerify_AttemptLimit_BurnsCode(t *testing.T) {
	env := newOTPTestEnv(t)
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	correctCode := env.extractEmailedCode(t)

	// Burn the attempt budget with wrong codes.
	for i := 0; i < otpMaxAttempts; i++ {
		env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":"000000"}`, env.email))
	}

	// Now even the correct code should fail — the row was burned on the
	// last wrong attempt.
	rec := env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":%q}`, env.email, correctCode))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status after burn = %d, want 401", rec.Code)
	}
}

func TestAuthOTPVerify_ExpiredCode(t *testing.T) {
	env := newOTPTestEnv(t)
	env.doRequest(t, fmt.Sprintf(`{"email":%q}`, env.email))
	code := env.extractEmailedCode(t)

	// Move the row's expiry into the past directly in the DB to avoid
	// real-time waits.
	if _, err := env.store.DB().Exec(
		`UPDATE auth_otp_codes SET expires_at=? WHERE email=?`,
		time.Now().UTC().Add(-1*time.Minute).Format(time.RFC3339), env.email); err != nil {
		t.Fatalf("force expiry: %v", err)
	}

	rec := env.doVerify(t, fmt.Sprintf(`{"email":%q,"code":%q}`, env.email, code))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthOTPVerify_MissingBodyFields(t *testing.T) {
	env := newOTPTestEnv(t)
	for _, body := range []string{
		`{}`,
		`{"email":"alice@otp.co"}`,
		`{"code":"123456"}`,
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			rec := env.doVerify(t, body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestNewOTPCode_AlwaysCorrectLength(t *testing.T) {
	for i := 0; i < 200; i++ {
		code, err := newOTPCode(otpCodeDigits)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(code) != otpCodeDigits {
			t.Errorf("len(%q) = %d", code, len(code))
		}
		// Strictly numeric.
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Errorf("non-digit in %q", code)
			}
		}
	}
}

func TestNewOTPCode_RejectsInvalidDigitCount(t *testing.T) {
	for _, n := range []int{0, -1, 13, 100} {
		if _, err := newOTPCode(n); err == nil {
			t.Errorf("newOTPCode(%d) should have errored", n)
		}
	}
}
