//go:build e2e

// Package e2e runs full end-to-end smoke tests against the real korva +
// korva-vault binaries. Enabled with `go test -tags e2e ./e2e/...` so it
// stays out of the default `go test ./...` matrix (it builds binaries and
// listens on a TCP port).
//
// What this covers, end-to-end:
//
//  1. `korva init --admin` provisions a fresh HOME
//  2. `korva-vault -mode=http` listens and serves
//  3. The REST API ingests observations via /api/v1/observations
//  4. /admin/projects lists what we just inserted
//  5. /admin/projects/suggestions surfaces name variants
//  6. /admin/projects/consolidate merges variants
//  7. /admin/projects/prune (dry-run) reports empty projects
//  8. /admin/export/obsidian writes a markdown vault to disk
//  9. /status responds (Hive reason-code surface in JSON)
//
// Anything that touches the local filesystem uses t.TempDir() / a temp HOME
// so the test is hermetic and safe for CI.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestEndToEnd_FullLifecycle is one big sequence — splitting it into multiple
// tests would force re-building the binaries and re-launching the vault
// 5+ times, which is wasted CI time. We keep it monolithic but use t.Run
// subtests for readability.
func TestEndToEnd_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e smoke skipped in -short mode")
	}

	// 0. Build the two binaries we need into a temp dir.
	binDir := t.TempDir()
	korvaBin := filepath.Join(binDir, "korva")
	vaultBin := filepath.Join(binDir, "korva-vault")
	mustBuild(t, korvaBin, "github.com/alcandev/korva/cli/cmd/korva")
	mustBuild(t, vaultBin, "github.com/alcandev/korva/vault/cmd/korva-vault")

	// 1. Isolated HOME for the whole run. KORVA_HIVE_DISABLE keeps the
	// worker quiet — we exercise its status surface separately.
	home := t.TempDir()
	env := append(os.Environ(),
		"HOME="+home,
		"KORVA_HIVE_DISABLE=1",
	)

	t.Run("init", func(t *testing.T) {
		out, err := runCmd(env, korvaBin, "init", "--admin", "--owner", "smoke@e2e.test")
		if err != nil {
			t.Fatalf("init failed: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(home, ".korva", "admin.key")); err != nil {
			t.Fatalf("admin.key missing: %v", err)
		}
	})

	adminKey := mustReadAdminKey(t, filepath.Join(home, ".korva", "admin.key"))
	port := mustFreePort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 2. Spin up the vault as a child process and tear it down on test end.
	vaultCmd := exec.Command(vaultBin, "-mode=http", "-port="+strconv.Itoa(port))
	vaultCmd.Env = env
	vaultLog := &bytes.Buffer{}
	vaultCmd.Stdout = vaultLog
	vaultCmd.Stderr = vaultLog
	if err := vaultCmd.Start(); err != nil {
		t.Fatalf("vault start: %v", err)
	}
	t.Cleanup(func() {
		_ = vaultCmd.Process.Signal(os.Interrupt)
		_, _ = vaultCmd.Process.Wait()
	})
	waitForReady(t, baseURL+"/v1/health", 5*time.Second, vaultLog)

	c := apiClient{base: baseURL, adminKey: adminKey, t: t}

	t.Run("save observations", func(t *testing.T) {
		ids := []string{}
		for _, obs := range []map[string]any{
			{"project": "korva", "type": "decision", "title": "Adopt ULID", "content": "Sortable + URL-safe.", "topic_key": "adopt-ulid"},
			{"project": "korva", "type": "pattern", "title": "Outbox", "content": "Decouples cloud failures.", "topic_key": "outbox"},
			{"project": "vault-mcp", "type": "decision", "title": "stdio transport", "content": "JSON-RPC over stdio.", "topic_key": "stdio-transport"},
		} {
			var resp map[string]string
			c.postJSON(t, "/api/v1/observations", obs, &resp, http.StatusCreated, /*admin=*/ false)
			if resp["id"] == "" {
				t.Errorf("save returned empty id: %+v", resp)
			}
			ids = append(ids, resp["id"])
		}
		if len(ids) != 3 {
			t.Fatalf("got %d ids, want 3", len(ids))
		}
	})

	t.Run("/admin/projects list", func(t *testing.T) {
		var resp struct {
			Projects []map[string]any `json:"projects"`
			Count    int              `json:"count"`
		}
		c.getJSON(t, "/admin/projects", &resp)
		if resp.Count != 2 {
			t.Errorf("project count = %d, want 2", resp.Count)
		}
	})

	t.Run("/admin/projects/suggestions surfaces variants", func(t *testing.T) {
		// Add a name-variant of "korva".
		variant := map[string]any{"project": "Korva", "type": "decision", "title": "Doc style", "content": "y"}
		var saved map[string]string
		c.postJSON(t, "/api/v1/observations", variant, &saved, http.StatusCreated, false)

		var resp struct {
			Proposals []map[string]any `json:"proposals"`
			Count     int              `json:"count"`
		}
		c.getJSON(t, "/admin/projects/suggestions", &resp)
		if resp.Count != 1 {
			t.Errorf("suggestions count = %d, want 1", resp.Count)
		}
	})

	t.Run("/admin/projects/consolidate merges variants", func(t *testing.T) {
		body := map[string]any{"canonical": "korva", "sources": []string{"Korva"}}
		var resp map[string]any
		c.postJSON(t, "/admin/projects/consolidate", body, &resp, http.StatusOK, true)
		if resp["status"] != "merged" {
			t.Errorf("status = %v, want merged", resp["status"])
		}
	})

	t.Run("/admin/projects/prune dry-run", func(t *testing.T) {
		var resp map[string]any
		c.postJSON(t, "/admin/projects/prune", map[string]any{}, &resp, http.StatusOK, true)
		if dr, _ := resp["dry_run"].(bool); !dr {
			t.Errorf("expected dry_run=true, got %+v", resp)
		}
	})

	t.Run("/admin/export/obsidian writes notes", func(t *testing.T) {
		exportDir := t.TempDir()
		body := map[string]any{"out": exportDir}
		var resp map[string]any
		c.postJSON(t, "/admin/export/obsidian", body, &resp, http.StatusOK, true)

		fileCount, _ := resp["file_count"].(float64)
		if int(fileCount) < 3 {
			t.Errorf("file_count = %v, want >= 3", resp["file_count"])
		}
		// Spot-check a couple of files we know should be on disk.
		mustExist := []string{
			"README.md",
			"korva/decision/adopt-ulid.md",
			"vault-mcp/decision/stdio-transport.md",
		}
		for _, rel := range mustExist {
			if _, err := os.Stat(filepath.Join(exportDir, rel)); err != nil {
				t.Errorf("export missing %s: %v", rel, err)
			}
		}
	})

	t.Run("/api/v1/hive/status carries reason field", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/hive/status")
		if err != nil {
			t.Fatalf("hive/status: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("hive/status: status %d: %s", resp.StatusCode, body)
		}
		// Worker is disabled via KORVA_HIVE_DISABLE so the JSON should pin
		// phase=disabled + reason=sync_paused. The point of the smoke is to
		// catch regressions where either key disappears from the wire.
		if !strings.Contains(string(body), `"phase"`) {
			t.Errorf("hive/status missing phase key: %s", body)
		}
		if !strings.Contains(string(body), `"reason"`) {
			t.Errorf("hive/status missing reason key: %s", body)
		}
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

type apiClient struct {
	base     string
	adminKey string
	t        *testing.T
}

func (c apiClient) getJSON(t *testing.T, path string, out any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, c.base+path, nil)
	req.Header.Set("X-Admin-Key", c.adminKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", path, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decode %s: %v\nbody=%s", path, err, body)
	}
}

func (c apiClient) postJSON(t *testing.T, path string, body, out any, wantStatus int, admin bool) {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if admin {
		req.Header.Set("X-Admin-Key", c.adminKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s: status %d, want %d: %s", path, resp.StatusCode, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode %s: %v\nbody=%s", path, err, raw)
		}
	}
}

func mustBuild(t *testing.T, outBin, pkg string) {
	t.Helper()
	// Build from the repo root; t.TempDir() gives us an isolated output path.
	cmd := exec.Command("go", "build", "-o", outBin, pkg)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func mustReadAdminKey(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin key: %v", err)
	}
	var k struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &k); err != nil {
		t.Fatalf("parse admin key: %v", err)
	}
	if k.Key == "" {
		t.Fatalf("admin key is empty")
	}
	return k.Key
}

func mustFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForReady(t *testing.T, url string, timeout time.Duration, log *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("vault never became ready at %s after %v\nvault log:\n%s", url, timeout, log)
}

func runCmd(env []string, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}
