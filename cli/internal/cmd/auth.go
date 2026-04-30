package cmd

import (
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

func init() {
	authCmd.AddCommand(authRedeemCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}

// vaultBase returns the local vault base URL from config (default port 7437).
func vaultBase() string {
	cfg, err := config.Load(mustPaths().ConfigFile)
	if err == nil && cfg.Vault.Port > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", cfg.Vault.Port)
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
