package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var obsCmd = &cobra.Command{
	Use:   "obs",
	Short: "Browse and search vault observations from the terminal",
}

var obsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent observations",
	Long: `List the most recent observations in the vault.
Use --project, --type, --limit, and --offset to filter and paginate.`,
	RunE: runObsList,
}

var obsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search observations",
	Args:  cobra.ExactArgs(1),
	RunE:  runObsSearch,
}

var obsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a single observation by ULID",
	Args:  cobra.ExactArgs(1),
	RunE:  runObsGet,
}

func init() {
	obsCmd.AddCommand(obsListCmd)
	obsCmd.AddCommand(obsSearchCmd)
	obsCmd.AddCommand(obsGetCmd)

	obsListCmd.Flags().String("project", "", "Filter by project name")
	obsListCmd.Flags().String("type", "", "Filter by type (decision, pattern, bugfix, learning, context, antipattern, task)")
	obsListCmd.Flags().Int("limit", 20, "Max results (1-200)")
	obsListCmd.Flags().Int("offset", 0, "Skip first N results")

	obsSearchCmd.Flags().String("project", "", "Filter by project name")
	obsSearchCmd.Flags().String("type", "", "Filter by type")
	obsSearchCmd.Flags().Int("limit", 20, "Max results (1-200)")
	obsSearchCmd.Flags().Bool("cloud", false, "Include Hive community results alongside local ones")
}

// --- handlers ---

func runObsList(cmd *cobra.Command, _ []string) error {
	paths := mustPaths()
	port := vaultPort(paths)

	project, _ := cmd.Flags().GetString("project")
	obsType, _ := cmd.Flags().GetString("type")
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")

	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	if obsType != "" {
		params.Set("type", obsType)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))

	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/search?%s", port, params.Encode())
	return doObsSearch(apiURL, "")
}

func runObsSearch(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	port := vaultPort(paths)

	project, _ := cmd.Flags().GetString("project")
	obsType, _ := cmd.Flags().GetString("type")
	limit, _ := cmd.Flags().GetInt("limit")
	cloud, _ := cmd.Flags().GetBool("cloud")

	params := url.Values{}
	params.Set("q", args[0])
	if project != "" {
		params.Set("project", project)
	}
	if obsType != "" {
		params.Set("type", obsType)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	if cloud {
		params.Set("cloud", "1")
	}

	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/search?%s", port, params.Encode())
	return doObsSearch(apiURL, args[0])
}

func runObsGet(_ *cobra.Command, args []string) error {
	paths := mustPaths()
	port := vaultPort(paths)

	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/observations/%s", port, args[0])

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("vault unreachable — is it running? (korva vault start): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("observation %q not found", args[0])
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var obs obsRow
	if err := json.NewDecoder(resp.Body).Decode(&obs); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	printObsDetail(obs)
	return nil
}

// --- internals ---

type obsRow struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Team      string   `json:"team"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Author    string   `json:"author"`
	CreatedAt string   `json:"created_at"`
	Source    string   `json:"source"` // local | hive (search only)
}

type obsSearchResp struct {
	Results []obsRow `json:"results"`
	Count   int      `json:"count"`
	Total   *int     `json:"total"` // nil for FTS searches
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

func doObsSearch(apiURL, query string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("vault unreachable — is it running? (korva vault start): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vault returned %d", resp.StatusCode)
	}

	var result obsSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(result.Results) == 0 {
		printInfo("No observations found")
		return nil
	}

	// Header
	if query != "" {
		fmt.Printf("  Search: %q  ·  %d result(s)\n\n", query, result.Count)
	} else {
		if result.Total != nil {
			fmt.Printf("  %d observation(s)  [offset %d, limit %d, total %d]\n\n",
				result.Count, result.Offset, result.Limit, *result.Total)
		} else {
			fmt.Printf("  %d observation(s)\n\n", result.Count)
		}
	}

	// Rows
	for _, obs := range result.Results {
		printObsSummary(obs)
	}
	return nil
}

func printObsSummary(obs obsRow) {
	typeLabel := padRight(obs.Type, 11)
	project := obs.Project
	if project == "" {
		project = "(no project)"
	}
	ts := formatObsDate(obs.CreatedAt)
	sourceTag := ""
	if obs.Source == "hive" {
		sourceTag = "  " + dimText("[hive]")
	}
	fmt.Printf("  %s  %-18s  %s%s\n", typeLabel, padRight(project, 18), ts, sourceTag)
	fmt.Printf("    %s\n", obs.Title)
	if obs.ID != "" {
		fmt.Printf("    %s\n", dimText(obs.ID))
	}
	fmt.Println()
}

func printObsDetail(obs obsRow) {
	fmt.Printf("  ID       : %s\n", obs.ID)
	fmt.Printf("  Title    : %s\n", obs.Title)
	fmt.Printf("  Type     : %s\n", obs.Type)
	fmt.Printf("  Project  : %s\n", obs.Project)
	if obs.Team != "" {
		fmt.Printf("  Team     : %s\n", obs.Team)
	}
	if obs.Author != "" {
		fmt.Printf("  Author   : %s\n", obs.Author)
	}
	fmt.Printf("  Created  : %s\n", formatObsDate(obs.CreatedAt))
	if len(obs.Tags) > 0 {
		fmt.Printf("  Tags     : %s\n", strings.Join(obs.Tags, ", "))
	}
	fmt.Println()
	fmt.Println("  " + strings.ReplaceAll(obs.Content, "\n", "\n  "))
}

func formatObsDate(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return s
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func dimText(s string) string {
	// ANSI dim — degrades gracefully in non-ANSI terminals.
	return "\033[2m" + s + "\033[0m"
}
