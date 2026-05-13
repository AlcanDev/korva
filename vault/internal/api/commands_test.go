package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Phase 7 — coverage for the one-click command endpoint. We pin three
// guarantees that matter most for safety: (1) the endpoint refuses non-local
// requests, (2) the whitelist is the only command surface, (3) timeouts +
// output caps are enforced.

func TestAdminListCommands_ReturnsWhitelist(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/commands", nil)
	req.Host = "127.0.0.1:7437"
	rec := httptest.NewRecorder()
	adminListCommands()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp commandListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Commands) < 5 {
		t.Errorf("expected at least 5 whitelisted commands, got %d", len(resp.Commands))
	}
	if !resp.LocalOnly {
		t.Error("expected local_only=true when Host is 127.0.0.1")
	}
}

func TestAdminRunCommand_RejectsRemoteRequests(t *testing.T) {
	body, _ := json.Marshal(commandRunRequest{Command: "status"})
	req := httptest.NewRequest(http.MethodPost, "/admin/commands/run", bytes.NewReader(body))
	req.Host = "vault.example.com:7437"
	rec := httptest.NewRecorder()
	adminRunCommand()(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAdminRunCommand_RejectsUnknownSlug(t *testing.T) {
	body, _ := json.Marshal(commandRunRequest{Command: "rm-rf-slash"})
	req := httptest.NewRequest(http.MethodPost, "/admin/commands/run", bytes.NewReader(body))
	req.Host = "127.0.0.1:7437"
	rec := httptest.NewRecorder()
	adminRunCommand()(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAdminRunCommand_RejectsMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/commands/run", strings.NewReader("not-json"))
	req.Host = "127.0.0.1:7437"
	rec := httptest.NewRecorder()
	adminRunCommand()(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// runCommand-level tests. These exercise the helper directly — no HTTP — so
// we can use stable cross-platform binaries (echo, sleep) without dragging
// the real korva CLI into the test path.

func TestRunCommand_CapturesStdoutAndExitCode(t *testing.T) {
	stdout, stderr, code, timedOut, truncated, err := runCommand(
		context.Background(), []string{"echo", "hello"},
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.TrimSpace(stdout) != "hello" {
		t.Errorf("stdout = %q, want hello", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if timedOut || truncated {
		t.Errorf("flags wrong: timedOut=%v truncated=%v", timedOut, truncated)
	}
}

func TestRunCommand_HonorsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, _, timedOut, _, _ := runCommand(ctx, []string{"sleep", "5"})
	if !timedOut {
		t.Error("expected timedOut=true after 100ms deadline against sleep 5")
	}
}

func TestCappedWriter_TruncatesAtCap(t *testing.T) {
	w := &cappedWriter{cap: 10}
	// Write 15 bytes — must truncate after 10.
	_, _ = w.Write([]byte("0123456789abcde"))
	if w.String() != "0123456789" {
		t.Errorf("buffer = %q, want first 10 bytes", w.String())
	}
	if !w.truncated {
		t.Error("expected truncated=true")
	}
	// Further writes are silently dropped.
	_, _ = w.Write([]byte("more"))
	if w.String() != "0123456789" {
		t.Errorf("post-truncation write polluted buffer: %q", w.String())
	}
}

func TestIsLocalRequest(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:7437", true},
		{"localhost:7437", true},
		{"[::1]:7437", true},
		{"127.0.0.1", true},
		{"localhost", true},
		{"::1", true},
		{"vault.example.com:7437", false},
		{"10.0.0.5:7437", false},
		{"192.168.1.10", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tc.host
			if got := isLocalRequest(r); got != tc.want {
				t.Errorf("isLocalRequest(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
