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
