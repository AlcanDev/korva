package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/profile"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Korva for Teams — manage team profile and members",
}

// ── sub-commands ──────────────────────────────────────────────────────────────

var teamsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync team profile: check Beacon first, fall back to Git",
	RunE:  runTeamsSync,
}

var teamsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active team profile source (beacon or git)",
	RunE:  runTeamsStatus,
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all teams (admin key required)",
	RunE:  runTeamsList,
}

var teamsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new team (admin key required)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsCreate,
}

var teamsMembersCmd = &cobra.Command{
	Use:   "members <team-id>",
	Short: "List members of a team (admin key required)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsMembers,
}

var teamsInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Generate a one-time invite token for a team member (admin key required)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsInvite,
}

var teamsAddMemberCmd = &cobra.Command{
	Use:   "add-member <email>",
	Short: "Add a member directly to a team without invite flow (admin key required)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsAddMember,
}

var teamsRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member <member-id>",
	Short: "Remove a member from a team by their member ID (admin key required)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsRemoveMember,
}

// ── flag vars ─────────────────────────────────────────────────────────────────

var (
	teamsSyncGitMirror      string
	teamsCreateOwner        string
	teamsCreateLicenseID    string
	teamsInviteTeamID       string
	teamsAddMemberTeamID    string
	teamsAddMemberRole      string
	teamsRemoveMemberTeamID string
)

func init() {
	teamsCmd.AddCommand(teamsSyncCmd)
	teamsCmd.AddCommand(teamsStatusCmd)
	teamsCmd.AddCommand(teamsListCmd)
	teamsCmd.AddCommand(teamsCreateCmd)
	teamsCmd.AddCommand(teamsMembersCmd)
	teamsCmd.AddCommand(teamsInviteCmd)
	teamsCmd.AddCommand(teamsAddMemberCmd)
	teamsCmd.AddCommand(teamsRemoveMemberCmd)

	teamsSyncCmd.Flags().StringVar(&teamsSyncGitMirror, "git-mirror", "",
		"If set, export the active profile to this Git repo URL after sync")

	teamsCreateCmd.Flags().StringVar(&teamsCreateOwner, "owner", "", "Owner email")
	teamsCreateCmd.Flags().StringVar(&teamsCreateLicenseID, "license-id", "", "License key ID (optional)")

	teamsInviteCmd.Flags().StringVar(&teamsInviteTeamID, "team", "", "Team ID (required)")
	_ = teamsInviteCmd.MarkFlagRequired("team")

	teamsAddMemberCmd.Flags().StringVar(&teamsAddMemberTeamID, "team", "", "Team ID (required)")
	teamsAddMemberCmd.Flags().StringVar(&teamsAddMemberRole, "role", "member", "Role: member or admin")
	_ = teamsAddMemberCmd.MarkFlagRequired("team")

	teamsRemoveMemberCmd.Flags().StringVar(&teamsRemoveMemberTeamID, "team", "", "Team ID (required)")
	_ = teamsRemoveMemberCmd.MarkFlagRequired("team")
}

// ── admin API helper ──────────────────────────────────────────────────────────

// adminDo performs an HTTP request against the local vault admin API.
// It sets Content-Type and X-Admin-Key on every request.
func adminDo(ctx context.Context, method, path string, body any) (*http.Response, error) {
	base := vaultBase()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	paths := mustPaths()
	if key := readAdminKey(paths.AdminKey); key != "" {
		req.Header.Set("X-Admin-Key", key)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// adminJSON performs adminDo and decodes the JSON response into dst.
// Body reads are capped at 4 MiB to prevent unbounded memory use.
func adminJSON(ctx context.Context, method, path string, body any, dst any) error {
	resp, err := adminDo(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("vault unreachable — is it running? (korva vault start): %w", err)
	}
	defer resp.Body.Close()
	const maxBody = 4 << 20 // 4 MiB
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errResp)
		if errResp.Error != "" {
			return fmt.Errorf("vault: %s (HTTP %d)", errResp.Error, resp.StatusCode)
		}
		return fmt.Errorf("vault returned HTTP %d", resp.StatusCode)
	}
	if dst != nil {
		return json.Unmarshal(raw, dst)
	}
	return nil
}

// ── sync / status (unchanged logic) ──────────────────────────────────────────

func runTeamsSync(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	lic, _ := license.Load(paths.LicenseFile) // nil = community tier
	mgr := profile.NewManager(paths, lic)

	adminKey := readAdminKey(paths.AdminKey)

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	beacon, err := mgr.FetchBeaconProfile(ctx, adminKey)
	if err != nil {
		printInfo(fmt.Sprintf("Beacon unreachable (%v), falling back to Git", err))
	}

	if beacon != nil {
		printSuccess(fmt.Sprintf("Beacon profile active — team: %v", beacon.Team["name"]))
		if teamsSyncGitMirror != "" {
			printInfo("Git mirror export not yet implemented in this version")
		}
		return nil
	}

	profileID, err := mgr.ActiveProfileID()
	if err != nil || profileID == "" {
		return fmt.Errorf("no active team profile — run 'korva init --profile <url>' or configure via Beacon panel")
	}

	printInfo(fmt.Sprintf("Syncing Git profile '%s'…", profileID))
	baseCfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if _, err := mgr.Sync(profileID, baseCfg); err != nil {
		return fmt.Errorf("git sync failed: %w", err)
	}
	printSuccess(fmt.Sprintf("Team profile '%s' synced from Git", profileID))
	return nil
}

func runTeamsStatus(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	lic, _ := license.Load(paths.LicenseFile)
	mgr := profile.NewManager(paths, lic)
	adminKey := readAdminKey(paths.AdminKey)

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	beacon, _ := mgr.FetchBeaconProfile(ctx, adminKey)
	if beacon != nil {
		fmt.Printf("  Source  : beacon (local vault)\n")
		if name, ok := beacon.Team["name"].(string); ok {
			fmt.Printf("  Team    : %s\n", name)
		}
		fmt.Printf("  Members : %d\n", len(beacon.Members))
		return nil
	}

	profileID, _ := mgr.ActiveProfileID()
	if profileID == "" {
		printInfo("No team profile configured (Community tier)")
		return nil
	}
	fmt.Printf("  Source     : git\n")
	fmt.Printf("  Profile ID : %s\n", profileID)
	return nil
}

// ── list ──────────────────────────────────────────────────────────────────────

func runTeamsList(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	var result struct {
		Teams []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Owner     string `json:"owner"`
			LicenseID string `json:"license_id"`
			CreatedAt string `json:"created_at"`
		} `json:"teams"`
	}
	if err := adminJSON(ctx, http.MethodGet, "/admin/teams", nil, &result); err != nil {
		return err
	}

	if len(result.Teams) == 0 {
		printInfo("No teams yet — use 'korva teams create <name>'")
		return nil
	}

	fmt.Printf("  %-16s  %-24s  %-26s  %s\n", "ID", "NAME", "OWNER", "CREATED")
	fmt.Printf("  %s\n", strings.Repeat("─", 82))
	for _, t := range result.Teams {
		owner := t.Owner
		if owner == "" {
			owner = "—"
		}
		fmt.Printf("  %-16s  %-24s  %-26s  %s\n",
			t.ID, truncate(t.Name, 24), truncate(owner, 26), formatTeamDate(t.CreatedAt))
	}
	return nil
}

// ── create ────────────────────────────────────────────────────────────────────

func runTeamsCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	body := map[string]string{
		"name":       args[0],
		"owner":      teamsCreateOwner,
		"license_id": teamsCreateLicenseID,
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := adminJSON(ctx, http.MethodPost, "/admin/teams", body, &result); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Team created — ID: %s", result.ID))
	printInfo(fmt.Sprintf("Next: add members with 'korva teams add-member <email> --team %s'", result.ID))
	return nil
}

// ── members ───────────────────────────────────────────────────────────────────

func runTeamsMembers(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	teamID := args[0]
	var result struct {
		Members []struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Role      string `json:"role"`
			CreatedAt string `json:"created_at"`
		} `json:"members"`
	}
	if err := adminJSON(ctx, http.MethodGet, "/admin/teams/"+url.PathEscape(teamID)+"/members", nil, &result); err != nil {
		return err
	}

	if len(result.Members) == 0 {
		printInfo("No members yet — use 'korva teams add-member <email> --team " + teamID + "'")
		return nil
	}

	fmt.Printf("  %-16s  %-30s  %-8s  %s\n", "MEMBER ID", "EMAIL", "ROLE", "JOINED")
	fmt.Printf("  %s\n", strings.Repeat("─", 74))
	for _, m := range result.Members {
		fmt.Printf("  %-16s  %-30s  %-8s  %s\n",
			m.ID, truncate(m.Email, 30), m.Role, formatTeamDate(m.CreatedAt))
	}
	return nil
}

// ── invite ────────────────────────────────────────────────────────────────────

func runTeamsInvite(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	email := args[0]
	body := map[string]string{"email": email}

	var result struct {
		ID        string `json:"id"`
		Token     string `json:"token"`
		Email     string `json:"email"`
		ExpiresAt string `json:"expires_at"`
		EmailSent bool   `json:"email_sent"`
		Note      string `json:"note"`
	}
	path := "/admin/teams/" + url.PathEscape(teamsInviteTeamID) + "/invites"
	if err := adminJSON(ctx, http.MethodPost, path, body, &result); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  ✓ Invite created for %s\n", result.Email)
	if result.EmailSent {
		fmt.Printf("  📧 Invite email sent automatically\n")
	} else {
		fmt.Println()
		fmt.Printf("  Token (shown once — share securely):\n")
		fmt.Printf("\n    %s\n\n", result.Token)
		fmt.Printf("  The developer can redeem it with:\n")
		fmt.Printf("\n    korva auth redeem %s\n\n", result.Token)
	}
	fmt.Printf("  Expires : %s\n", formatTeamDate(result.ExpiresAt))
	if result.Note != "" {
		printInfo(result.Note)
	}
	return nil
}

// ── add-member ────────────────────────────────────────────────────────────────

func runTeamsAddMember(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	body := map[string]string{
		"email": args[0],
		"role":  teamsAddMemberRole,
	}
	var result struct {
		ID string `json:"id"`
	}
	path := "/admin/teams/" + url.PathEscape(teamsAddMemberTeamID) + "/members"
	if err := adminJSON(ctx, http.MethodPost, path, body, &result); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Member added — ID: %s  email: %s  role: %s",
		result.ID, args[0], teamsAddMemberRole))
	printInfo(fmt.Sprintf("Generate their session token: 'korva teams invite %s --team %s'",
		args[0], teamsAddMemberTeamID))
	return nil
}

// ── remove-member ─────────────────────────────────────────────────────────────

func runTeamsRemoveMember(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	memberID := args[0]
	path := "/admin/teams/" + url.PathEscape(teamsRemoveMemberTeamID) + "/members/" + url.PathEscape(memberID)
	var result struct {
		Status string `json:"status"`
	}
	if err := adminJSON(ctx, http.MethodDelete, path, nil, &result); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Member %s removed from team %s", memberID, teamsRemoveMemberTeamID))
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// readAdminKey reads the admin key value from the key file.
func readAdminKey(keyPath string) string {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	var kf admin.AdminConfig
	if err := json.Unmarshal(data, &kf); err != nil {
		return ""
	}
	return kf.Key
}

func formatTeamDate(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return s
}

func truncate(s string, max int) string {
	if max <= 1 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
