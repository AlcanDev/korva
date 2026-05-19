package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with your organization's Korva for Teams",
}

var authRedeemCmd = &cobra.Command{
	Use:   "redeem <invite-token>",
	Short: "Redeem a one-time invite token to activate your session",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthRedeem,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session and organization",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the current session",
	RunE:  runAuthLogout,
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the current session token (for editor MCP config)",
	Long: `Print the plaintext session token stored at ~/.korva/session.token.

Useful for wiring the remote MCP endpoint into your editor:
  curl -H "Authorization: Bearer $(korva auth token)" https://mcp.korva.dev/mcp

Exits non-zero with a hint when no session is active.`,
	RunE: runAuthToken,
}

// authLoginOpts captures the flags for `korva auth login`. The email comes
// from --email; the code is normally typed interactively (so it can be
// scripted via `--code` for tests / automation).
var authLoginOpts struct {
	Email string
	Code  string
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with email — a one-time code is sent to your inbox",
	Long: `Sign in to Korva for Teams by email. Requires that an admin has already
added your address to the team (run 'korva teams invite' once per member).

Flow:
  1. Vault emails a 6-digit code to the address you provide.
  2. Enter the code at the prompt (or pass it with --code for automation).
  3. A 30-day session token is saved at ~/.korva/session.token.

This is the way to re-authenticate after a session expires or when
installing the CLI on a new machine — no admin in the loop.`,
	RunE: runAuthLogin,
}

func init() {
	authCmd.AddCommand(authRedeemCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authTokenCmd)

	authLoginCmd.Flags().StringVarP(&authLoginOpts.Email, "email", "e", "", "email registered by your team admin (required)")
	authLoginCmd.Flags().StringVar(&authLoginOpts.Code, "code", "", "skip the prompt and pass the OTP directly (for scripts/tests)")
}

// vaultBase returns the vault base URL the CLI should target. Resolution
// order, highest priority first:
//  1. KORVA_VAULT_ENDPOINT env var (e.g. "https://api.korva.dev").
//  2. vault.endpoint in ~/.korva/config.json (set via `korva config set`).
//  3. http://127.0.0.1:<vault.port> when vault.port is configured.
//  4. http://127.0.0.1:7437 (local default).
//
// The trailing slash is stripped so callers can safely concatenate paths
// like vaultBase()+"/auth/redeem".
func vaultBase() string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv("KORVA_VAULT_ENDPOINT")), "/"); v != "" {
		return v
	}
	cfg, err := config.Load(mustPaths().ConfigFile)
	if err == nil {
		if e := strings.TrimRight(strings.TrimSpace(cfg.Vault.Endpoint), "/"); e != "" {
			return e
		}
		if cfg.Vault.Port > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", cfg.Vault.Port)
		}
	}
	return "http://127.0.0.1:7437"
}

// mustPaths returns platform paths and exits on error.
func mustPaths() *config.Paths {
	paths, err := config.PlatformPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ cannot resolve paths: %v\n", err)
		os.Exit(1)
	}
	return paths
}

func runAuthToken(cmd *cobra.Command, _ []string) error {
	path := mustPaths().SessionTokenFile
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no session token at %s — run `korva auth login` or `korva auth redeem <token>` first", path)
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return fmt.Errorf("session token file is empty (%s)", path)
	}
	fmt.Fprintln(cmd.OutOrStdout(), token)
	return nil
}

func runAuthRedeem(cmd *cobra.Command, args []string) error {
	token := strings.TrimSpace(args[0])
	if len(token) < 16 {
		return fmt.Errorf("token looks too short — paste the full invite token")
	}

	paths := mustPaths()
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{"token": token})
	req, err := http.NewRequestWithContext(ctx, "POST", vaultBase()+"/auth/redeem",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable — is korva-vault running? (%w)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var e map[string]string
		json.Unmarshal(raw, &e)
		if msg := e["error"]; msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("redeem failed (status %d)", resp.StatusCode)
	}

	var result struct {
		SessionToken string `json:"session_token"`
		Email        string `json:"email"`
		Team         string `json:"team"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}

	// Persist session token at ~/.korva/session.token (mode 0600)
	if err := os.WriteFile(paths.SessionTokenFile, []byte(result.SessionToken+"\n"), 0600); err != nil {
		return fmt.Errorf("saving session token: %w", err)
	}

	printSuccess(fmt.Sprintf("Authenticated as %s", result.Email))
	printInfo(fmt.Sprintf("Organization : %s", result.Team))
	if result.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, result.ExpiresAt); err == nil {
			printInfo(fmt.Sprintf("Session valid until %s", t.Local().Format("2006-01-02")))
		}
	}
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	sessionToken, err := readSessionToken(paths.SessionTokenFile)
	if err != nil {
		printInfo("No active session — run 'korva auth redeem <invite-token>'")
		return nil
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", vaultBase()+"/auth/me", nil)
	req.Header.Set("X-Session-Token", sessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		printInfo("Session expired or invalid — run 'korva auth redeem <invite-token>'")
		os.Remove(paths.SessionTokenFile)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status check failed (status %d)", resp.StatusCode)
	}

	var me struct {
		Email     string `json:"email"`
		Team      string `json:"team"`
		Role      string `json:"role"`
		ExpiresAt string `json:"expires_at"`
	}
	json.Unmarshal(raw, &me)

	fmt.Printf("  ✓ Authenticated\n")
	fmt.Printf("  Email        : %s\n", me.Email)
	fmt.Printf("  Organization : %s\n", me.Team)
	fmt.Printf("  Role         : %s\n", me.Role)
	if t, err := time.Parse(time.RFC3339, me.ExpiresAt); err == nil {
		fmt.Printf("  Valid until  : %s\n", t.Local().Format("2006-01-02"))
		remaining := time.Until(t)
		if remaining < 7*24*time.Hour && remaining > 0 {
			days := int(remaining.Hours() / 24)
			fmt.Printf("\n  ⚠  Session expires in %d day(s). Run 'korva auth redeem' with a new invite to renew.\n", days)
		}
	}
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	sessionToken, err := readSessionToken(paths.SessionTokenFile)
	if err != nil {
		printInfo("No active session")
		return nil
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "DELETE", vaultBase()+"/auth/session", nil)
	req.Header.Set("X-Session-Token", sessionToken)
	if resp, _ := http.DefaultClient.Do(req); resp != nil { // best-effort server logout
		_ = resp.Body.Close()
	}

	os.Remove(paths.SessionTokenFile)
	printSuccess("Logged out")
	return nil
}

// readSessionToken reads ~/.korva/session.token.
// Returns an error when no session is saved.
func readSessionToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// loginIO is the test seam for `korva auth login`. Production paths use
// os.Stdin / os.Stdout / os.Stderr; tests inject buffers so they can drive
// the interactive prompt without a TTY.
type loginIO struct {
	in     io.Reader
	out    io.Writer
	prompt string // shown before the code is read; empty = no prompt printed
}

func defaultLoginIO() loginIO {
	return loginIO{in: os.Stdin, out: os.Stdout, prompt: "Enter the 6-digit code: "}
}

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	email := strings.ToLower(strings.TrimSpace(authLoginOpts.Email))
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("--email is required and must be a valid address")
	}
	return doAuthLogin(cmd.Context(), vaultBase(), mustPaths().SessionTokenFile,
		email, authLoginOpts.Code, defaultLoginIO())
}

// doAuthLogin is the test-friendly core of `korva auth login`. It runs the
// two-step OTP exchange (request → verify), persists the resulting session
// token at `tokenPath`, and reports progress to `lio.out`.
//
// When `presetCode` is non-empty (e.g. CI / scripted use) the interactive
// prompt is skipped entirely. Otherwise the function reads one line from
// `lio.in` after printing `lio.prompt`.
func doAuthLogin(ctx context.Context, baseURL, tokenPath, email, presetCode string, lio loginIO) error {
	// Step 1: request the code.
	ctxReq, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]string{"email": email})
	req, err := http.NewRequestWithContext(ctxReq, http.MethodPost, baseURL+"/auth/otp/request",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable — is korva-vault running? (%w)", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("too many login attempts — wait an hour before trying again")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("OTP request failed (status %d)", resp.StatusCode)
	}

	fmt.Fprintf(lio.out, "  → Code sent to %s. Check your inbox.\n", email)

	// Step 2: read the code (interactive or preset).
	code := strings.TrimSpace(presetCode)
	if code == "" {
		if lio.prompt != "" {
			fmt.Fprint(lio.out, lio.prompt)
		}
		line, err := bufio.NewReader(lio.in).ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("reading code: %w", err)
		}
		code = strings.TrimSpace(line)
	}
	if code == "" {
		return fmt.Errorf("code is required")
	}

	// Step 3: verify.
	ctxVer, cancelVer := context.WithTimeout(ctx, 10*time.Second)
	defer cancelVer()
	verifyBody, _ := json.Marshal(map[string]string{"email": email, "code": code})
	req2, err := http.NewRequestWithContext(ctxVer, http.MethodPost, baseURL+"/auth/otp/verify",
		bytes.NewReader(verifyBody))
	if err != nil {
		return err
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp2.Body.Close()
	raw, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		var e map[string]string
		_ = json.Unmarshal(raw, &e)
		if msg := e["error"]; msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("OTP verify failed (status %d)", resp2.StatusCode)
	}

	var result struct {
		SessionToken string `json:"session_token"`
		Email        string `json:"email"`
		Team         string `json:"team"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}
	if err := os.WriteFile(tokenPath, []byte(result.SessionToken+"\n"), 0600); err != nil {
		return fmt.Errorf("saving session token: %w", err)
	}

	fmt.Fprintf(lio.out, "  ✓ Authenticated as %s\n", result.Email)
	if result.Team != "" {
		fmt.Fprintf(lio.out, "  → Organization : %s\n", result.Team)
	}
	if t, err := time.Parse(time.RFC3339, result.ExpiresAt); err == nil {
		fmt.Fprintf(lio.out, "  → Session valid until %s\n", t.Local().Format("2006-01-02"))
	}
	return nil
}
