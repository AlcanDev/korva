package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Phase 5 — `korva export` umbrella command.
//
//   korva export obsidian --out DIR [--project NAME] [--type TYPE]
//
// The Obsidian subcommand drives the matching admin endpoint, which calls
// store.ExportObsidian to render the vault as a directory of markdown notes
// (frontmatter + wikilinks). Useful for browsing your knowledge graph in any
// markdown-native tool — Obsidian, Logseq, Foam, plain VSCode.

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the vault into other formats (Obsidian, …)",
	Long: `Export your Korva vault into formats other tools understand. The
inaugural target is Obsidian — every observation becomes a markdown file with
YAML frontmatter and [[wikilinks]] for related observations. Re-running over
the same directory is safe; notes are rewritten from the live store state.`,
}

var exportObsidianCmd = &cobra.Command{
	Use:   "obsidian",
	Short: "Render the vault as Obsidian-flavored markdown",
	RunE:  runExportObsidian,
}

func init() {
	exportCmd.AddCommand(exportObsidianCmd)
	exportObsidianCmd.Flags().String("out", "", "Output directory (required)")
	exportObsidianCmd.Flags().String("project", "", "Restrict to a single project")
	exportObsidianCmd.Flags().String("type", "", "Restrict to a single observation type")
	_ = exportObsidianCmd.MarkFlagRequired("out")
}

type obsidianExportResponse struct {
	OutDir       string         `json:"out_dir"`
	FileCount    int            `json:"file_count"`
	ProjectCount int            `json:"project_count"`
	ByProject    map[string]int `json:"by_project"`
	ByType       map[string]int `json:"by_type"`
	GeneratedAt  string         `json:"generated_at"`
}

func runExportObsidian(cmd *cobra.Command, _ []string) error {
	out, _ := cmd.Flags().GetString("out")
	project, _ := cmd.Flags().GetString("project")
	obsType, _ := cmd.Flags().GetString("type")
	if out == "" {
		return fmt.Errorf("--out is required")
	}

	body, _ := json.Marshal(map[string]any{
		"out":     out,
		"project": project,
		"type":    obsType,
	})

	var resp obsidianExportResponse
	if err := adminPostJSON("/admin/export/obsidian", body, &resp); err != nil {
		return err
	}

	fmt.Printf("Obsidian export written to %s\n", resp.OutDir)
	fmt.Printf("  %d note(s) across %d project(s)\n", resp.FileCount, resp.ProjectCount)
	if len(resp.ByType) > 0 {
		fmt.Println("  by type:")
		for t, n := range resp.ByType {
			fmt.Printf("    %-12s %d\n", t, n)
		}
	}
	fmt.Println()
	fmt.Println("  Open the output directory in Obsidian (File → Open vault → choose folder)")
	fmt.Println("  and follow the wikilinks under each project's _index.md.")
	return nil
}
