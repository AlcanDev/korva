package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Phase 4 — operator CLI for project hygiene.
//
//   korva projects list                              tabular view of every project
//   korva projects suggest                           propose merges for variant names
//   korva projects consolidate --canonical X         merge one or more sources into a canonical
//                              --source A --source B
//   korva projects prune       [--apply]             clean up sessions for empty projects
//
// All subcommands hit the admin REST API and require admin.key. Defaults err
// on the safe side: `prune` is a dry-run unless --apply is set; `consolidate`
// refuses to operate without at least one --source.

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Inspect, consolidate, and clean up project namespaces",
	Long: `Korva tracks every observation, session, and prompt under a project
name. Over time a single team can accumulate variants (alpha vs Alpha, my-project
vs my_project) and orphan sessions from abandoned MCP runs.

The "projects" command is the operator's swiss-army knife for that drift:
inspect the full inventory, surface merge candidates, fold variants into a
canonical name, and prune empty projects whose only signal was a stray session.`,
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every project with observation + session counts",
	RunE:  runProjectsList,
}

var projectsSuggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Propose merges for projects that share a normalized name",
	RunE:  runProjectsSuggest,
}

var projectsConsolidateCmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Merge one or more source projects into a canonical name",
	RunE:  runProjectsConsolidate,
}

var projectsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Find (and optionally delete) sessions for projects with zero observations",
	RunE:  runProjectsPrune,
}

func init() {
	projectsCmd.AddCommand(projectsListCmd, projectsSuggestCmd, projectsConsolidateCmd, projectsPruneCmd)

	projectsConsolidateCmd.Flags().String("canonical", "", "Target project name to keep (required)")
	projectsConsolidateCmd.Flags().StringSlice("source", nil, "Source project(s) to fold into --canonical (repeatable)")
	_ = projectsConsolidateCmd.MarkFlagRequired("canonical")
	_ = projectsConsolidateCmd.MarkFlagRequired("source")

	projectsPruneCmd.Flags().Bool("apply", false, "Actually delete the orphan sessions (otherwise: dry-run)")
}

// ── list ────────────────────────────────────────────────────────────────────

type projectsListResponse struct {
	Projects []projectStatsRow `json:"projects"`
	Count    int               `json:"count"`
}

type projectStatsRow struct {
	Name             string `json:"name"`
	ObservationCount int    `json:"observation_count"`
	SessionCount     int    `json:"session_count"`
}

func runProjectsList(_ *cobra.Command, _ []string) error {
	var resp projectsListResponse
	if err := adminGetJSON("/admin/projects", &resp); err != nil {
		return err
	}
	fmt.Printf("Projects (%d total)\n", resp.Count)
	fmt.Println(strings.Repeat("─", 33))
	if len(resp.Projects) == 0 {
		fmt.Println("  (none — vault has no observations or sessions yet)")
		return nil
	}
	fmt.Printf("  %-32s %12s %12s\n", "NAME", "OBSERVATIONS", "SESSIONS")
	for _, p := range resp.Projects {
		name := p.Name
		if len(name) > 32 {
			name = name[:31] + "…"
		}
		fmt.Printf("  %-32s %12d %12d\n", name, p.ObservationCount, p.SessionCount)
	}
	return nil
}

// ── suggest ─────────────────────────────────────────────────────────────────

type suggestResponse struct {
	Proposals []consolidationProposal `json:"proposals"`
	Count     int                     `json:"count"`
}

type consolidationProposal struct {
	Canonical string            `json:"canonical"`
	Variants  []projectStatsRow `json:"variants"`
}

func runProjectsSuggest(_ *cobra.Command, _ []string) error {
	var resp suggestResponse
	if err := adminGetJSON("/admin/projects/suggestions", &resp); err != nil {
		return err
	}
	fmt.Printf("Consolidation suggestions — %d group(s)\n", resp.Count)
	fmt.Println(strings.Repeat("─", 33))
	if len(resp.Proposals) == 0 {
		fmt.Println("  (no variants found — every project has a unique normalized name)")
		return nil
	}
	for _, p := range resp.Proposals {
		fmt.Printf("  canonical → %s\n", p.Canonical)
		sources := make([]string, 0, len(p.Variants)-1)
		for _, v := range p.Variants {
			if v.Name == p.Canonical {
				continue
			}
			sources = append(sources, fmt.Sprintf("%s (%d obs)", v.Name, v.ObservationCount))
		}
		fmt.Printf("    sources: %s\n", strings.Join(sources, ", "))
		fmt.Printf("    suggested: korva projects consolidate --canonical %s", p.Canonical)
		for _, v := range p.Variants {
			if v.Name == p.Canonical {
				continue
			}
			fmt.Printf(" --source %s", v.Name)
		}
		fmt.Println()
		fmt.Println()
	}
	return nil
}

// ── consolidate ─────────────────────────────────────────────────────────────

type consolidateResponse struct {
	Status              string   `json:"status"`
	Canonical           string   `json:"canonical"`
	Sources             []string `json:"sources"`
	ObservationsUpdated int64    `json:"observations_updated"`
	SessionsUpdated     int64    `json:"sessions_updated"`
	PromptsUpdated      int64    `json:"prompts_updated"`
}

func runProjectsConsolidate(cmd *cobra.Command, _ []string) error {
	canonical, _ := cmd.Flags().GetString("canonical")
	sources, _ := cmd.Flags().GetStringSlice("source")
	if canonical == "" {
		return fmt.Errorf("--canonical is required")
	}
	if len(sources) == 0 {
		return fmt.Errorf("at least one --source is required")
	}

	body, _ := json.Marshal(map[string]any{
		"canonical": canonical,
		"sources":   sources,
	})

	var resp consolidateResponse
	if err := adminPostJSON("/admin/projects/consolidate", body, &resp); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Merged %d source(s) into %q", len(resp.Sources), resp.Canonical))
	fmt.Printf("    observations updated: %d\n", resp.ObservationsUpdated)
	fmt.Printf("    sessions updated:     %d\n", resp.SessionsUpdated)
	fmt.Printf("    prompts updated:      %d\n", resp.PromptsUpdated)
	return nil
}

// ── prune ───────────────────────────────────────────────────────────────────

type pruneResponse struct {
	Empty []struct {
		Project      string `json:"project"`
		SessionCount int    `json:"session_count"`
		PromptCount  int    `json:"prompt_count"`
	} `json:"empty"`
	SessionsRemoved int64 `json:"sessions_removed"`
	PromptsRemoved  int64 `json:"prompts_removed"`
	DryRun          bool  `json:"dry_run"`
}

func runProjectsPrune(cmd *cobra.Command, _ []string) error {
	apply, _ := cmd.Flags().GetBool("apply")
	body, _ := json.Marshal(map[string]any{"apply": apply})

	var resp pruneResponse
	if err := adminPostJSON("/admin/projects/prune", body, &resp); err != nil {
		return err
	}

	mode := "dry-run"
	if !resp.DryRun {
		mode = "applied"
	}
	fmt.Printf("Prune empty projects (%s) — %d candidate(s)\n", mode, len(resp.Empty))
	fmt.Println(strings.Repeat("─", 33))
	if len(resp.Empty) == 0 {
		fmt.Println("  (nothing to prune — every project with sessions also has observations)")
		return nil
	}
	fmt.Printf("  %-32s %12s %12s\n", "PROJECT", "SESSIONS", "PROMPTS")
	for _, e := range resp.Empty {
		name := e.Project
		if len(name) > 32 {
			name = name[:31] + "…"
		}
		fmt.Printf("  %-32s %12d %12d\n", name, e.SessionCount, e.PromptCount)
	}
	fmt.Println()
	if resp.DryRun {
		fmt.Println("  (dry-run — re-run with --apply to actually delete)")
	} else {
		printSuccess(fmt.Sprintf("Removed %d session(s), %d prompt(s)", resp.SessionsRemoved, resp.PromptsRemoved))
	}
	return nil
}
