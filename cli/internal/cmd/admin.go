package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin operations (requires admin.key on this machine)",
}

var adminInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate the admin key for this machine",
	RunE:  runAdminInit,
}

var adminRotateCmd = &cobra.Command{
	Use:   "rotate-key",
	Short: "Rotate the admin key (requires current key)",
	RunE:  runAdminRotate,
}

var (
	adminOwner string
	adminForce bool
)

func init() {
	adminCmd.AddCommand(adminInitCmd)
	adminCmd.AddCommand(adminRotateCmd)
	adminInitCmd.Flags().StringVar(&adminOwner, "owner", "", "Admin owner email (required)")
	adminInitCmd.Flags().BoolVar(&adminForce, "force", false, "Overwrite existing admin key")
	adminInitCmd.MarkFlagRequired("owner")
}

func runAdminInit(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	if err := paths.EnsureAll(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	cfg, err := admin.Generate(paths.AdminKey, adminOwner, adminForce)
	if err != nil {
		if err == admin.ErrKeyExists {
			return fmt.Errorf("admin.key already exists — use --force to overwrite, or 'korva admin rotate-key' to rotate")
		}
		return fmt.Errorf("generating admin key: %w", err)
	}

	fmt.Printf("\nAdmin key generated for %s (version %d)\n", cfg.Owner, cfg.Version)
	fmt.Printf("Stored at: %s (permissions: 0600)\n", paths.AdminKey)
	fmt.Println("\nThis file is your admin credential — keep it secure.")
	fmt.Println("It is NEVER synced to Git or shared with other machines.")
	return nil
}

func runAdminRotate(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	fmt.Print("Enter current admin key: ")
	keyBytes, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return fmt.Errorf("reading admin key: %w", err)
	}
	fmt.Println("")

	currentKey := string(keyBytes)
	cfg, err := admin.Rotate(paths.AdminKey, currentKey)
	if err != nil {
		return fmt.Errorf("rotating key: %w", err)
	}

	fmt.Printf("Admin key rotated (now version %d)\n", cfg.Version)
	fmt.Println("Update any services that use the old admin key.")
	return nil
}
