package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// These tests cover the parts of `korva projects` that don't require a live
// Vault: command wiring, flag declarations, and required-flag enforcement.
// The HTTP path is exercised by the admin handler tests in
// vault/internal/api/projects_test.go.

func TestProjectsCmd_HasAllSubcommands(t *testing.T) {
	want := map[string]bool{
		"list":        false,
		"suggest":     false,
		"consolidate": false,
		"prune":       false,
	}
	for _, c := range projectsCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("projects subcommand %q not registered", name)
		}
	}
}

func TestProjectsConsolidate_RequiresCanonicalAndSource(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"canonical is required", "canonical"},
		{"source is required", "source"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := projectsConsolidateCmd.Flags().Lookup(tc.flag)
			if f == nil {
				t.Fatalf("flag %q not declared", tc.flag)
			}
			ann := f.Annotations[cobra.BashCompOneRequiredFlag]
			if len(ann) == 0 || ann[0] != "true" {
				t.Errorf("flag %q is not marked required (annotations=%v)", tc.flag, f.Annotations)
			}
		})
	}
}

func TestProjectsPrune_HasApplyFlagDefaultFalse(t *testing.T) {
	f := projectsPruneCmd.Flags().Lookup("apply")
	if f == nil {
		t.Fatal("apply flag not declared")
	}
	if f.DefValue != "false" {
		t.Errorf("apply default = %q, want \"false\" (prune must be dry-run by default)", f.DefValue)
	}
}

func TestRunProjectsConsolidate_ErrorsWithoutFlags(t *testing.T) {
	// Build a throwaway cobra.Command with the same flag layout but no admin
	// key configured. We expect early-exit validation, not a network call.
	c := &cobra.Command{}
	c.Flags().String("canonical", "", "")
	c.Flags().StringSlice("source", nil, "")

	if err := runProjectsConsolidate(c, nil); err == nil {
		t.Error("expected error when --canonical is empty")
	}

	_ = c.Flags().Set("canonical", "alpha")
	if err := runProjectsConsolidate(c, nil); err == nil {
		t.Error("expected error when --source is empty")
	}
}
