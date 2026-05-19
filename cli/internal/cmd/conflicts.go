package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
)

// Phase 2 — operator CLI for the conflict judgment workflow.
//
//   korva conflicts list   [--status pending|judged|orphaned|ignored]
//                          [--project NAME] [--limit N]
//   korva conflicts show   <id>
//   korva conflicts judge  <id> --relation <verb> [--reason text]
//                                [--confidence 0..1] [--evidence text]
//   korva conflicts ignore <id> [--reason text]
//
// All subcommands talk to the admin REST API; admin.key is required.

var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "List, inspect, and resolve auto-detected observation conflicts",
	Long: `Vault auto-detects candidate conflicts whenever an agent saves a new
observation that overlaps with existing knowledge in the same project. This
command surfaces those pending judgments so an operator can confirm, refine,
or dismiss them without dropping into the database.`,
}

var conflictsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List conflicts (default: pending judgments)",
	RunE:  runConflictsList,
}

var conflictsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single conflict with both observations",
	Args:  cobra.ExactArgs(1),
	RunE:  runConflictsShow,
}

var conflictsJudgeCmd = &cobra.Command{
	Use:   "judge <id>",
	Short: "Record the verdict for a pending judgment",
	Args:  cobra.ExactArgs(1),
	RunE:  runConflictsJudge,
}

var conflictsIgnoreCmd = &cobra.Command{
	Use:   "ignore <id>",
	Short: "Dismiss a pending judgment as not-a-conflict",
	Args:  cobra.ExactArgs(1),
	RunE:  runConflictsIgnore,
}

func init() {
	conflictsCmd.AddCommand(conflictsListCmd, conflictsShowCmd, conflictsJudgeCmd, conflictsIgnoreCmd)
	conflictsListCmd.Flags().String("status", "pending", "Status filter: pending | judged | orphaned | ignored")
	conflictsListCmd.Flags().String("project", "", "Restrict to one project (default: all)")
	conflictsListCmd.Flags().Int("limit", 50, "Maximum rows to return")

	conflictsJudgeCmd.Flags().String("relation", "", "Verdict verb: supersedes | conflicts_with | related | compatible | scoped")
	conflictsJudgeCmd.Flags().String("reason", "", "Short rationale shown in audit history")
	conflictsJudgeCmd.Flags().String("evidence", "", "Long-form evidence (e.g. LLM reasoning)")
	conflictsJudgeCmd.Flags().Float64("confidence", 1.0, "Confidence in [0, 1]")
	conflictsJudgeCmd.Flags().String("session-id", "", "Optional session ID for audit trail")
	_ = conflictsJudgeCmd.MarkFlagRequired("relation")

	conflictsIgnoreCmd.Flags().String("reason", "", "Why this candidate is not actually a conflict")
	conflictsIgnoreCmd.Flags().String("session-id", "", "Optional session ID for audit trail")
}

// ── list ────────────────────────────────────────────────────────────────────

type conflictListResponse struct {
	Conflicts []conflictRow `json:"conflicts"`
	Count     int           `json:"count"`
	Status    string        `json:"status"`
	Project   string        `json:"project,omitempty"`
}

type conflictRow struct {
	ID             string  `json:"id"`
	SourceID       string  `json:"source_id"`
	TargetID       string  `json:"target_id"`
	Relation       string  `json:"relation"`
	Project        string  `json:"project"`
	JudgmentStatus string  `json:"judgment_status"`
	Confidence     float64 `json:"confidence"`
	Reason         string  `json:"reason,omitempty"`
	CreatedAt      string  `json:"created_at"`
	JudgedAt       *string `json:"judged_at,omitempty"`
}

func runConflictsList(cmd *cobra.Command, _ []string) error {
	status, _ := cmd.Flags().GetString("status")
	project, _ := cmd.Flags().GetString("project")
	limit, _ := cmd.Flags().GetInt("limit")

	q := url.Values{}
	q.Set("status", status)
	if project != "" {
		q.Set("project", project)
	}
	q.Set("limit", strconv.Itoa(limit))

	var resp conflictListResponse
	if err := adminGetJSON("/admin/conflicts?"+q.Encode(), &resp); err != nil {
		return err
	}

	fmt.Printf("Conflicts (status=%s", resp.Status)
	if resp.Project != "" {
		fmt.Printf(", project=%s", resp.Project)
	}
	fmt.Printf(") — %d row(s)\n", resp.Count)
	fmt.Println(strings.Repeat("─", 33))
	if len(resp.Conflicts) == 0 {
		fmt.Println("  (none)")
		return nil
	}
	for _, c := range resp.Conflicts {
		relation := c.Relation
		if relation == "" {
			relation = "—"
		}
		fmt.Printf("  %s\n", c.ID)
		fmt.Printf("    %s ↔ %s  (%s)\n", short(c.SourceID), short(c.TargetID), c.Project)
		fmt.Printf("    status=%s relation=%s confidence=%.2f\n", c.JudgmentStatus, relation, c.Confidence)
		if c.Reason != "" {
			fmt.Printf("    reason: %s\n", c.Reason)
		}
		fmt.Println()
	}
	return nil
}

// ── show ────────────────────────────────────────────────────────────────────

type conflictDetailResponse struct {
	Conflict conflictRow `json:"conflict"`
	Source   *obsSummary `json:"source"`
	Target   *obsSummary `json:"target"`
}

type obsSummary struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Project string `json:"project"`
}

func runConflictsShow(cmd *cobra.Command, args []string) error {
	id := args[0]
	var resp conflictDetailResponse
	if err := adminGetJSON("/admin/conflicts/"+url.PathEscape(id), &resp); err != nil {
		return err
	}
	c := resp.Conflict
	fmt.Printf("Conflict %s\n", c.ID)
	fmt.Println(strings.Repeat("─", 33))
	fmt.Printf("  Project:   %s\n", c.Project)
	fmt.Printf("  Status:    %s\n", c.JudgmentStatus)
	fmt.Printf("  Relation:  %s\n", or(c.Relation, "(pending)"))
	fmt.Printf("  Conf:      %.2f\n", c.Confidence)
	if c.Reason != "" {
		fmt.Printf("  Reason:    %s\n", c.Reason)
	}
	fmt.Printf("  Created:   %s\n", c.CreatedAt)
	if c.JudgedAt != nil {
		fmt.Printf("  Judged:    %s\n", *c.JudgedAt)
	}
	fmt.Println()
	printObservation("Source", resp.Source)
	fmt.Println()
	printObservation("Target", resp.Target)
	return nil
}

func printObservation(label string, o *obsSummary) {
	if o == nil {
		fmt.Printf("%s: (missing)\n", label)
		return
	}
	fmt.Printf("%s [%s] %s\n", label, o.Type, o.Title)
	fmt.Printf("  %s\n", truncateForCLI(o.Content, 280))
}

func truncateForCLI(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ── judge ───────────────────────────────────────────────────────────────────

func runConflictsJudge(cmd *cobra.Command, args []string) error {
	id := args[0]
	relation, _ := cmd.Flags().GetString("relation")
	reason, _ := cmd.Flags().GetString("reason")
	evidence, _ := cmd.Flags().GetString("evidence")
	confidence, _ := cmd.Flags().GetFloat64("confidence")
	sessID, _ := cmd.Flags().GetString("session-id")

	body, _ := json.Marshal(map[string]any{
		"relation":        relation,
		"reason":          reason,
		"evidence":        evidence,
		"confidence":      confidence,
		"marked_by_actor": "user",
		"marked_by_kind":  "manual",
		"session_id":      sessID,
	})

	var resp conflictRow
	if err := adminPostJSON("/admin/conflicts/"+url.PathEscape(id)+"/judge", body, &resp); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Conflict %s judged as %q (confidence %.2f)", resp.ID, resp.Relation, resp.Confidence))
	return nil
}

// ── ignore ──────────────────────────────────────────────────────────────────

func runConflictsIgnore(cmd *cobra.Command, args []string) error {
	id := args[0]
	reason, _ := cmd.Flags().GetString("reason")
	sessID, _ := cmd.Flags().GetString("session-id")

	body, _ := json.Marshal(map[string]any{"reason": reason, "session_id": sessID})
	var resp map[string]string
	if err := adminPostJSON("/admin/conflicts/"+url.PathEscape(id)+"/ignore", body, &resp); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Conflict %s ignored: %s", id, reason))
	return nil
}

// ── HTTP helpers ────────────────────────────────────────────────────────────
// Lightweight, shared by the four subcommands. Mirrors doctor.go's fetch
// helper but generalised so future admin-API CLIs can reuse.

func adminGetJSON(path string, out any) error {
	return adminCall(http.MethodGet, path, nil, out)
}

func adminPostJSON(path string, body []byte, out any) error {
	return adminCall(http.MethodPost, path, body, out)
}

func adminCall(method, path string, body []byte, out any) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	key, err := admin.Load(paths.AdminKey)
	if err != nil {
		return fmt.Errorf("admin key required — run `korva init --admin` first")
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, vaultBase()+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", key.Key)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("vault returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// ── tiny utility helpers ────────────────────────────────────────────────────

func short(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
