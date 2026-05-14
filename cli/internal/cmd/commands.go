package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available Korva commands by category",
	Run:   runCommands,
}

func runCommands(_ *cobra.Command, _ []string) {
	type entry struct {
		name string
		desc string
	}
	type section struct {
		title   string
		entries []entry
	}

	sections := []section{
		{
			"Setup & Configuration",
			[]entry{
				{"korva init", "Initialize Korva in the current project (creates korva.config.json)"},
				{"korva setup", "Configure VS Code, Cursor, and Claude Code to use Vault MCP"},
				{"korva setup --global", "Configure editors globally (no workspace files written)"},
				{"korva setup --local", "Write workspace-level .vscode/mcp.json for the current project"},
				{"korva config list", "Show merged configuration (global + local)"},
				{"korva config list --global", "Show global config only (~/.korva/config.json)"},
				{"korva config get <key>", "Get a config value, e.g. 'vault.port' or 'project'"},
				{"korva config set <key> <value>", "Set a config value in local config (use --global for global)"},
				{"korva doctor", "Diagnose the Korva installation and report any issues"},
			},
		},
		{
			"Vault (Knowledge Memory)",
			[]entry{
				{"korva vault start", "Start the Vault MCP + REST server in the background"},
				{"korva vault stop", "Stop the running Vault server"},
				{"korva vault status", "Show whether the Vault server is running"},
				{"korva vault logs", "Tail live Vault server logs"},
				{"korva obs list", "List recent observations stored in the Vault"},
				{"korva obs search <query>", "Full-text search across observations"},
				{"korva obs get <id>", "Show a specific observation by ID"},
			},
		},
		{
			"Lore (Knowledge Scrolls)",
			[]entry{
				{"korva lore list", "List available knowledge scrolls"},
				{"korva lore show <name>", "Display a scroll's content"},
				{"korva lore install <name>", "Install a public scroll from the Korva registry"},
				{"korva lore new <name>", "Create a new private scroll"},
			},
		},
		{
			"Sentinel (Code Quality)",
			[]entry{
				{"korva sentinel install", "Install pre-commit hooks in the current repo"},
				{"korva sentinel uninstall", "Remove Korva pre-commit hooks"},
				{"korva sentinel run", "Run sentinel validation manually on staged files"},
			},
		},
		{
			"Teams & Auth",
			[]entry{
				{"korva auth login --email <x>", "Sign in by email — a one-time code is mailed to you"},
				{"korva auth redeem <token>", "Redeem an admin-issued invite token (first-time setup)"},
				{"korva auth logout", "Sign out of Korva Teams"},
				{"korva auth status", "Show current authentication status"},
				{"korva teams list", "List teams you belong to"},
				{"korva teams invite <email>", "Admin: invite a member by email"},
				{"korva hive enable", "Enable contribution to the Korva Hive community brain"},
				{"korva hive disable", "Opt out of the Hive"},
				{"korva skills list", "List team skills available for the current project"},
			},
		},
		{
			"Harness Engineering (autonomous agents)",
			[]entry{
				{"korva harness init", "Lay down AGENTS.md, init.sh, feature_list.json, docs/, progress/ (--sdd for spec-driven)"},
				{"korva harness status", "Show backlog counts + currently in_progress feature"},
				{"korva harness list", "Print every feature with its status"},
				{"korva harness next", "Show the next pending feature + acceptance criteria"},
				{"korva harness start <id>", "Move a feature to in_progress"},
				{"korva harness done <id>", "Move a feature to done"},
				{"korva harness block <id>", "Mark a feature as blocked"},
				{"korva harness reopen <id>", "Return a feature to pending"},
				{"korva harness add", "Append a new feature (--name, --title, --accept, --sdd)"},
				{"korva harness spec <id>", "SDD: materialize specs/<feature>/{requirements,design,tasks}.md"},
				{"korva harness ready <id>", "SDD: mark a feature's spec as ready for human approval (pending → spec_ready)"},
			},
		},
		{
			"Maintenance",
			[]entry{
				{"korva sync", "Sync Vault observations to/from a Git remote"},
				{"korva license activate <key>", "Activate a Korva for Teams license"},
				{"korva update", "Check for a newer version of Korva and upgrade if available"},
				{"korva admin", "Administrative operations (requires admin.key)"},
			},
		},
		{
			"Global flags (all commands)",
			[]entry{
				{"--help, -h", "Show help for any command"},
				{"--version, -v", "Print the Korva version"},
				{"korva completion <shell>", "Generate shell completion for bash, zsh, fish, or powershell"},
			},
		},
	}

	fmt.Println("Korva — available commands")
	fmt.Println()

	for _, sec := range sections {
		fmt.Printf("  %s\n", sec.title)
		fmt.Printf("  %s\n", repeatStr("─", len(sec.title)))
		for _, e := range sec.entries {
			fmt.Printf("    %-40s %s\n", e.name, e.desc)
		}
		fmt.Println()
	}

	fmt.Println("Run 'korva <command> --help' for detailed usage of any command.")
}

func repeatStr(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
