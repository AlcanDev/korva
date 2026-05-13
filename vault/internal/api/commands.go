package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Phase 7 — One-click commands endpoint.
//
//   POST /admin/commands/run   body: {"command": "<slug>"}
//
// Operators can trigger a curated set of `korva` CLI commands from the
// Beacon dashboard. The endpoint is restricted on every axis we can think of:
//
//   1. Admin auth (router middleware) — same gate as every other /admin route.
//   2. Localhost-only host check — refuses when r.Host doesn't resolve to
//      127.0.0.1 / ::1 / "localhost". A vault exposed on a LAN won't let
//      remote callers spawn local processes.
//   3. Hard whitelist — the JSON body carries a slug (e.g. "status"), not
//      a raw command string. We never interpolate user input into the argv;
//      args are built from constants below.
//   4. Hard timeout (10 s) — any command that runs longer is killed.
//   5. Capped output (256 KiB total) — protects the response writer.
//
// The response shape pins exit_code, stdout, stderr, duration_ms — enough
// for the dashboard to render a terminal-like card without follow-up calls.

// allowedCommand describes one whitelisted invocation.
type allowedCommand struct {
	Slug        string   // wire key — what the client sends
	Description string   // human-readable label (also returned by the listing)
	Argv        []string // exact args; never tainted by client input
}

// commandsWhitelist is the only source of truth for what can run.
// Add a new entry here AND on the frontend before any new button appears.
var commandsWhitelist = []allowedCommand{
	{Slug: "status", Description: "Show running services and last sync", Argv: []string{"korva", "status"}},
	{Slug: "doctor", Description: "Run health checks", Argv: []string{"korva", "doctor"}},
	{Slug: "hive-status", Description: "Show Hive worker phase + reason", Argv: []string{"korva", "hive", "status"}},
	{Slug: "projects-list", Description: "List projects with counts", Argv: []string{"korva", "projects", "list"}},
	{Slug: "projects-suggest", Description: "Propose project consolidations", Argv: []string{"korva", "projects", "suggest"}},
	{Slug: "config-show", Description: "Print the resolved config", Argv: []string{"korva", "config", "show"}},
	{Slug: "version", Description: "Print the korva version", Argv: []string{"korva", "--version"}},
}

// commandRunRequest is the wire shape.
type commandRunRequest struct {
	Command string `json:"command"`
}

// commandRunResponse is what we ship back.
type commandRunResponse struct {
	Slug       string `json:"slug"`
	Argv       string `json:"argv"` // joined for display (operator-readable)
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMS int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
	Truncated  bool   `json:"truncated"` // true if output exceeded the cap
}

// commandListResponse exposes the whitelist so the UI can build buttons
// without duplicating the list on the frontend.
type commandListResponse struct {
	Commands  []commandListEntry `json:"commands"`
	LocalOnly bool               `json:"local_only"` // true when this vault accepts the run endpoint
}

type commandListEntry struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Argv        string `json:"argv"` // joined, for tooltip
}

// adminListCommands handles GET /admin/commands.
func adminListCommands() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := make([]commandListEntry, 0, len(commandsWhitelist))
		for _, c := range commandsWhitelist {
			out = append(out, commandListEntry{
				Slug:        c.Slug,
				Description: c.Description,
				Argv:        strings.Join(c.Argv, " "),
			})
		}
		writeJSON(w, http.StatusOK, commandListResponse{
			Commands:  out,
			LocalOnly: isLocalRequest(r),
		})
	}
}

// adminRunCommand handles POST /admin/commands/run.
func adminRunCommand() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLocalRequest(r) {
			writeError(w, http.StatusForbidden,
				"command runner is local-only; expose vault on 127.0.0.1 to use it")
			return
		}
		var req commandRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cmd, ok := findCommand(req.Command)
		if !ok {
			writeError(w, http.StatusBadRequest, "unknown command slug")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		start := time.Now()
		stdout, stderr, exitCode, timedOut, truncated, err := runCommand(ctx, cmd.Argv)
		duration := time.Since(start)
		if err != nil && exitCode == 0 && !timedOut {
			// Process-level error (e.g. binary not found) — surface in stderr.
			stderr = err.Error()
			exitCode = -1
		}

		// Publish to SSE so the activity feed reflects what just ran.
		PublishEvent(Event{
			Kind:  EventCommandRun,
			Title: cmd.Slug,
			Meta: map[string]any{
				"argv":        strings.Join(cmd.Argv, " "),
				"exit_code":   exitCode,
				"duration_ms": duration.Milliseconds(),
				"timed_out":   timedOut,
			},
		})
		writeJSON(w, http.StatusOK, commandRunResponse{
			Slug:       cmd.Slug,
			Argv:       strings.Join(cmd.Argv, " "),
			ExitCode:   exitCode,
			Stdout:     stdout,
			Stderr:     stderr,
			DurationMS: duration.Milliseconds(),
			TimedOut:   timedOut,
			Truncated:  truncated,
		})
	}
}

// findCommand looks up a slug in the whitelist; returns false if unknown.
func findCommand(slug string) (allowedCommand, bool) {
	slug = strings.TrimSpace(slug)
	for _, c := range commandsWhitelist {
		if c.Slug == slug {
			return c, true
		}
	}
	return allowedCommand{}, false
}

// outputCap is the per-stream byte ceiling. Keeps response sizes predictable
// even when a command floods stdout.
const outputCap = 256 * 1024 // 256 KiB

// runCommand spawns the process, waits with the parent context timeout, and
// returns captured stdout/stderr (each capped to outputCap bytes).
func runCommand(ctx context.Context, argv []string) (stdout, stderr string, exitCode int, timedOut bool, truncated bool, err error) {
	if len(argv) == 0 {
		return "", "", -1, false, false, errors.New("empty argv")
	}
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	stdoutB, stderrB := &cappedWriter{cap: outputCap}, &cappedWriter{cap: outputCap}
	c.Stdout = stdoutB
	c.Stderr = stderrB
	runErr := c.Run()
	timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	truncated = stdoutB.truncated || stderrB.truncated

	if runErr != nil {
		// exec.ExitError carries the exit status when the process actually ran.
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return stdoutB.String(), stderrB.String(), ee.ExitCode(), timedOut, truncated, nil
		}
		// Anything else (binary missing, permission denied, …) bubbles up.
		return stdoutB.String(), stderrB.String(), -1, timedOut, truncated, runErr
	}
	return stdoutB.String(), stderrB.String(), 0, timedOut, truncated, nil
}

// cappedWriter is a write sink with a hard byte limit. After the cap is hit
// it silently drops everything else but flips the `truncated` flag so the
// caller can surface "output was clipped" in the UI.
type cappedWriter struct {
	cap       int
	buf       []byte
	truncated bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.truncated {
		return len(p), nil
	}
	remaining := w.cap - len(w.buf)
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf = append(w.buf, p[:remaining]...)
		w.truncated = true
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *cappedWriter) String() string { return string(w.buf) }

// isLocalRequest returns true when the request's apparent host is loopback.
// Uses net.SplitHostPort so IPv4, IPv6 ([::1]:7437), and host-only ("::1",
// "localhost") variants all classify correctly.
func isLocalRequest(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	// Fall through for any other loopback variant (127.0.0.2, ::ffff:127.0.0.1).
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
