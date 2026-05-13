package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// otpFakeVault stands in for the live vault during CLI tests. It records the
// last request body for assertions, returns 204 on /request and a configurable
// payload on /verify so we can exercise both success and error paths.
type otpFakeVault struct {
	server         *httptest.Server
	requestCalls   atomic.Int32
	verifyCalls    atomic.Int32
	lastReqEmail   string
	lastVerEmail   string
	lastVerCode    string
	verifyStatus   int
	verifyResponse map[string]string
	requestStatus  int
}

func newOTPFakeVault(t *testing.T) *otpFakeVault {
	t.Helper()
	v := &otpFakeVault{
		verifyStatus: http.StatusOK,
		verifyResponse: map[string]string{
			"session_token": "session-tok-xyz",
			"email":         "alice@otp.co",
			"team":          "OTP Co",
			"team_id":       "team-1",
			"expires_at":    "2030-01-01T00:00:00Z",
		},
		requestStatus: http.StatusNoContent,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/otp/request", func(w http.ResponseWriter, r *http.Request) {
		v.requestCalls.Add(1)
		var body struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		v.lastReqEmail = body.Email
		w.WriteHeader(v.requestStatus)
	})
	mux.HandleFunc("/auth/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		v.verifyCalls.Add(1)
		var body struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		v.lastVerEmail = body.Email
		v.lastVerCode = body.Code
		w.WriteHeader(v.verifyStatus)
		if v.verifyStatus == http.StatusOK {
			_ = json.NewEncoder(w).Encode(v.verifyResponse)
		} else {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wrong code"})
		}
	})
	v.server = httptest.NewServer(mux)
	t.Cleanup(v.server.Close)
	return v
}

// runLoginHelper invokes doAuthLogin against the fake vault using `code`
// as the preset OTP. Returns the recorded output + final token file content.
func runLoginHelper(t *testing.T, v *otpFakeVault, email, code string) (string, string, error) {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "session.token")
	var out bytes.Buffer
	err := doAuthLogin(context.Background(), v.server.URL, tokenPath, email, code,
		loginIO{in: strings.NewReader(""), out: &out})
	saved := ""
	if data, e := os.ReadFile(tokenPath); e == nil {
		saved = strings.TrimSpace(string(data))
	}
	return out.String(), saved, err
}

func TestDoAuthLogin_HappyPathWithPresetCode(t *testing.T) {
	v := newOTPFakeVault(t)
	out, savedToken, err := runLoginHelper(t, v, "alice@otp.co", "482917")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if savedToken != "session-tok-xyz" {
		t.Errorf("saved token = %q, want session-tok-xyz", savedToken)
	}
	if v.requestCalls.Load() != 1 {
		t.Errorf("request calls = %d, want 1", v.requestCalls.Load())
	}
	if v.verifyCalls.Load() != 1 {
		t.Errorf("verify calls = %d, want 1", v.verifyCalls.Load())
	}
	if v.lastVerCode != "482917" {
		t.Errorf("server received code = %q", v.lastVerCode)
	}
	if !strings.Contains(out, "Authenticated as alice@otp.co") {
		t.Errorf("output missing success line: %s", out)
	}
}

func TestDoAuthLogin_InteractivePrompt(t *testing.T) {
	v := newOTPFakeVault(t)
	tokenPath := filepath.Join(t.TempDir(), "session.token")
	var out bytes.Buffer
	in := strings.NewReader("123456\n")

	err := doAuthLogin(context.Background(), v.server.URL, tokenPath, "alice@otp.co", "",
		loginIO{in: in, out: &out, prompt: "Code: "})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if v.lastVerCode != "123456" {
		t.Errorf("code from prompt = %q, want 123456", v.lastVerCode)
	}
	if !strings.Contains(out.String(), "Code: ") {
		t.Errorf("prompt not printed: %s", out.String())
	}
}

func TestDoAuthLogin_RequestRateLimited(t *testing.T) {
	v := newOTPFakeVault(t)
	v.requestStatus = http.StatusTooManyRequests
	_, _, err := runLoginHelper(t, v, "alice@otp.co", "111111")
	if err == nil || !strings.Contains(err.Error(), "too many login attempts") {
		t.Errorf("expected rate-limit error, got %v", err)
	}
	if v.verifyCalls.Load() != 0 {
		t.Errorf("verify must not run when /request fails, got %d calls", v.verifyCalls.Load())
	}
}

func TestDoAuthLogin_VerifyRejectsWrongCode(t *testing.T) {
	v := newOTPFakeVault(t)
	v.verifyStatus = http.StatusUnauthorized
	_, savedToken, err := runLoginHelper(t, v, "alice@otp.co", "000000")
	if err == nil || !strings.Contains(err.Error(), "wrong code") {
		t.Errorf("expected verify failure, got %v", err)
	}
	if savedToken != "" {
		t.Errorf("no token should be written on failure, got %q", savedToken)
	}
}

func TestDoAuthLogin_EmptyCodeIsRejected(t *testing.T) {
	v := newOTPFakeVault(t)
	tokenPath := filepath.Join(t.TempDir(), "session.token")
	var out bytes.Buffer
	// Interactive prompt that returns immediately with no code.
	err := doAuthLogin(context.Background(), v.server.URL, tokenPath, "alice@otp.co", "",
		loginIO{in: strings.NewReader("\n"), out: &out})
	if err == nil || !strings.Contains(err.Error(), "code is required") {
		t.Errorf("expected 'code is required', got %v", err)
	}
	if v.verifyCalls.Load() != 0 {
		t.Errorf("verify must not run when code is empty")
	}
}

func TestDoAuthLogin_VaultUnreachable(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "session.token")
	var out bytes.Buffer
	// Point at a port we know nothing's listening on.
	err := doAuthLogin(context.Background(), "http://127.0.0.1:1", tokenPath, "alice@otp.co", "111111",
		loginIO{in: strings.NewReader(""), out: &out})
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "vault unreachable") {
		t.Errorf("error should mention vault unreachable, got %v", err)
	}
}

func TestRunAuthLogin_RequiresEmail(t *testing.T) {
	authLoginOpts.Email = ""
	t.Cleanup(func() {
		authLoginOpts = struct {
			Email string
			Code  string
		}{}
	})

	err := runAuthLogin(authLoginCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--email") {
		t.Errorf("expected --email error, got %v", err)
	}
}

func TestRunAuthLogin_RejectsInvalidEmail(t *testing.T) {
	authLoginOpts.Email = "not-an-email"
	t.Cleanup(func() {
		authLoginOpts = struct {
			Email string
			Code  string
		}{}
	})

	err := runAuthLogin(authLoginCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "valid address") {
		t.Errorf("expected invalid-email error, got %v", err)
	}
}

// Sanity test: the fake server sees the email arg passed to doAuthLogin.
func TestDoAuthLogin_ForwardsEmailToBothEndpoints(t *testing.T) {
	v := newOTPFakeVault(t)
	_, _, err := runLoginHelper(t, v, "bob@example.com", "654321")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if v.lastReqEmail != "bob@example.com" {
		t.Errorf("/request email = %q", v.lastReqEmail)
	}
	if v.lastVerEmail != "bob@example.com" {
		t.Errorf("/verify email = %q", v.lastVerEmail)
	}
}

// Compile-time check: loginIO.in must satisfy io.Reader (so tests can pass a buffer).
var _ io.Reader = strings.NewReader("")
