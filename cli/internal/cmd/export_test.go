package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// Light wiring tests for `korva export`. The actual export work is exercised
// by store and API tests; here we just pin the command structure and the
// out-of-the-box required-flag behavior.

func TestExportCmd_HasObsidianSubcommand(t *testing.T) {
	found := false
	for _, c := range exportCmd.Commands() {
		if c.Name() == "obsidian" {
			found = true
		}
	}
	if !found {
		t.Error("export obsidian subcommand not registered")
	}
}

func TestExportObsidian_OutFlagIsRequired(t *testing.T) {
	f := exportObsidianCmd.Flags().Lookup("out")
	if f == nil {
		t.Fatal("--out flag not declared")
	}
	ann := f.Annotations[cobra.BashCompOneRequiredFlag]
	if len(ann) == 0 || ann[0] != "true" {
		t.Errorf("--out is not marked required (annotations=%v)", f.Annotations)
	}
}

func TestRunExportObsidian_ErrorsWithoutOut(t *testing.T) {
	c := &cobra.Command{}
	c.Flags().String("out", "", "")
	c.Flags().String("project", "", "")
	c.Flags().String("type", "", "")
	if err := runExportObsidian(c, nil); err == nil {
		t.Error("expected error when --out is empty")
	}
}
