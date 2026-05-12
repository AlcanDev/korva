package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveRepairMode(t *testing.T) {
	mk := func() *cobra.Command {
		c := &cobra.Command{}
		c.Flags().Bool("plan", false, "")
		c.Flags().Bool("dry-run", false, "")
		c.Flags().Bool("apply", false, "")
		return c
	}

	tests := []struct {
		name    string
		flags   map[string]string
		want    string
		wantErr bool
	}{
		{"default is plan", nil, "plan", false},
		{"plan flag", map[string]string{"plan": "true"}, "plan", false},
		{"dry-run flag", map[string]string{"dry-run": "true"}, "dry_run", false},
		{"apply flag", map[string]string{"apply": "true"}, "apply", false},
		{"plan + apply errors", map[string]string{"plan": "true", "apply": "true"}, "", true},
		{"all three error", map[string]string{"plan": "true", "dry-run": "true", "apply": "true"}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := mk()
			for k, v := range tc.flags {
				if err := c.Flags().Set(k, v); err != nil {
					t.Fatalf("set %s: %v", k, err)
				}
			}
			got, err := resolveRepairMode(c)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got mode=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
