package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/version"
)

var rootCmd = &cobra.Command{
	Use:     "korva",
	Short:   "Korva — AI ecosystem for enterprise development teams",
	Version: version.String(),
	// CompletionOptions fine-tunes the auto-generated `korva completion` sub-command.
	CompletionOptions: cobra.CompletionOptions{
		// Keep the completion command visible in `korva --help`.
		HiddenDefaultCmd: false,
	},
	// PersistentPreRun fires before every sub-command and triggers a
	// non-blocking background update check (at most once every 24 h).
	// The hint is printed to stderr so it never interferes with command output.
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		go CheckUpdateHint()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(loreCmd)
	rootCmd.AddCommand(sentinelCmd)
	rootCmd.AddCommand(adminCmd)
	rootCmd.AddCommand(hiveCmd)
	rootCmd.AddCommand(licenseCmd)
	rootCmd.AddCommand(teamsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(vaultCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(obsCmd)
	rootCmd.AddCommand(skillsCmd)
}

func printSuccess(msg string) {
	fmt.Fprintf(os.Stdout, "  ✓ %s\n", msg)
}

func printInfo(msg string) {
	fmt.Fprintf(os.Stdout, "  → %s\n", msg)
}

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "  ✗ %s\n", msg)
}
