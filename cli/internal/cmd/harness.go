package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/harness"
)

// harnessCmd is the parent for every `korva harness <sub>` command.
// Harness Engineering is Korva's bootstrap for repos that want to be
// worked on by autonomous AI agents: AGENTS.md, init.sh, a state
// machine in feature_list.json, progress/, docs/, CHECKPOINTS.md.
// The sub-commands are deliberately small — each maps to one verb the
// state machine understands (init, start, done, block) plus a few
// read-only views (status, list, next).
var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "Bootstrap and manage the agent-ready harness for any repo",
	Long: `Harness Engineering — turn any repo into a place an AI agent can work
autonomously and verifiably.

  korva harness init           Materialize AGENTS.md, init.sh, feature_list.json, docs/, progress/
  korva harness status         Show backlog counts + currently in_progress feature
  korva harness list           Print every feature with its status
  korva harness next           Show the next pending feature
  korva harness start <id>     Set a feature to in_progress
  korva harness done <id>      Set a feature to done
  korva harness block <id>     Set a feature to blocked
  korva harness add            Append a new feature to the backlog`,
}

// --- init ------------------------------------------------------------------

type harnessInitFlags struct {
	Root          string
	Project       string
	Description   string
	Stack         string
	WithSubagents bool
	Overwrite     bool
}

var harnessInitOpts harnessInitFlags

var harnessInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Lay down the harness files in the current directory",
	RunE:  runHarnessInit,
}

func runHarnessInit(_ *cobra.Command, _ []string) error {
	root := harnessInitOpts.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	project := harnessInitOpts.Project
	if project == "" {
		project = filepath.Base(abs)
	}

	stack := harness.Stack(strings.ToLower(strings.TrimSpace(harnessInitOpts.Stack)))
	if stack == "" {
		stack = detectStack(abs)
	}
	if !isKnownStack(stack) {
		return fmt.Errorf("unknown stack %q — pick one of: %s", stack, joinStacks())
	}

	written, err := harness.Generate(harness.InitOptions{
		Root:          abs,
		Project:       project,
		Description:   harnessInitOpts.Description,
		Stack:         stack,
		WithSubagents: harnessInitOpts.WithSubagents,
		Overwrite:     harnessInitOpts.Overwrite,
	})
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Harness initialized for %q (stack: %s)", project, stack))
	for _, f := range written {
		fmt.Printf("    + %s\n", f)
	}
	if len(written) == 0 {
		printInfo("No new files written — harness already present (use --overwrite to force).")
	}
	printInfo("Next: review feature_list.json and run ./init.sh")
	return nil
}

// --- status ----------------------------------------------------------------

var harnessStatusJSON bool

var harnessStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show backlog counts + currently in_progress feature",
	RunE:  runHarnessStatus,
}

// statusPayload is what `harness status --json` emits. The wire shape
// is deliberately small so `init.sh` (or any other script) can pipe it
// into jq without parsing prose.
type statusPayload struct {
	Project    string         `json:"project"`
	Counts     harness.Counts `json:"counts"`
	InProgress *featureWire   `json:"in_progress,omitempty"`
	NextID     int            `json:"next_pending_id,omitempty"`
}

type featureWire struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func runHarnessStatus(_ *cobra.Command, _ []string) error {
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	payload := statusPayload{
		Project: fl.Project,
		Counts:  fl.CountByStatus(),
	}
	if cur := fl.CurrentInProgress(); cur != nil {
		payload.InProgress = &featureWire{ID: cur.ID, Name: cur.Name, Title: cur.Title}
	}
	if next := fl.NextPending(); next != nil {
		payload.NextID = next.ID
	}

	if harnessStatusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	c := payload.Counts
	fmt.Printf("Project:    %s\n", fl.Project)
	if fl.Description != "" {
		fmt.Printf("            %s\n", fl.Description)
	}
	fmt.Println()
	fmt.Printf("  pending:     %d\n", c.Pending)
	fmt.Printf("  in_progress: %d\n", c.InProgress)
	fmt.Printf("  done:        %d\n", c.Done)
	fmt.Printf("  blocked:     %d\n", c.Blocked)
	fmt.Printf("  ─────────────────\n")
	fmt.Printf("  total:       %d\n", c.Total)
	fmt.Println()
	if payload.InProgress != nil {
		fmt.Printf("In progress: #%d %s — %s\n", payload.InProgress.ID, payload.InProgress.Name, payload.InProgress.Title)
	} else {
		fmt.Println("In progress: (none)")
	}
	if payload.NextID > 0 {
		fmt.Printf("Next pending: #%d  (run 'korva harness next' to see acceptance criteria)\n", payload.NextID)
	} else {
		fmt.Println("Next pending: (backlog clear)")
	}
	return nil
}

// --- list ------------------------------------------------------------------

var harnessListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print every feature with its status",
	RunE:  runHarnessList,
}

func runHarnessList(_ *cobra.Command, _ []string) error {
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	for _, f := range fl.Features {
		marker := statusMarker(f.Status)
		title := f.Title
		if title == "" {
			title = f.Name
		}
		fmt.Printf("  %s  #%-3d  %-12s  %s\n", marker, f.ID, string(f.Status), title)
	}
	return nil
}

// --- next ------------------------------------------------------------------

var harnessNextJSON bool

var harnessNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the next pending feature with its acceptance criteria",
	RunE:  runHarnessNext,
}

func runHarnessNext(_ *cobra.Command, _ []string) error {
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	next := fl.NextPending()
	if next == nil {
		if harnessNextJSON {
			_, _ = fmt.Fprintln(os.Stdout, "null")
		} else {
			printInfo("No pending features — backlog clear.")
		}
		return nil
	}
	if harnessNextJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(next)
	}
	fmt.Printf("#%d  %s — %s\n", next.ID, next.Name, next.Title)
	if next.Description != "" {
		fmt.Println()
		fmt.Println(next.Description)
	}
	if len(next.Acceptance) > 0 {
		fmt.Println()
		fmt.Println("Acceptance:")
		for _, a := range next.Acceptance {
			fmt.Printf("  - %s\n", a)
		}
	}
	fmt.Println()
	fmt.Printf("Start it with:  korva harness start %d\n", next.ID)
	return nil
}

// --- start / done / block (transition commands) ----------------------------

var harnessStartCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Set a feature to in_progress",
	Args:  cobra.ExactArgs(1),
	RunE:  transitionRunner(harness.StatusInProgress),
}

var harnessDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Set a feature to done",
	Args:  cobra.ExactArgs(1),
	RunE:  transitionRunner(harness.StatusDone),
}

var harnessBlockCmd = &cobra.Command{
	Use:   "block <id>",
	Short: "Set a feature to blocked",
	Args:  cobra.ExactArgs(1),
	RunE:  transitionRunner(harness.StatusBlocked),
}

var harnessReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Return a blocked / in_progress feature to pending",
	Args:  cobra.ExactArgs(1),
	RunE:  transitionRunner(harness.StatusPending),
}

// transitionRunner is the shared body for start / done / block / reopen.
// Each command differs only by target status, so the runner closes over it.
func transitionRunner(target harness.FeatureStatus) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("id must be an integer, got %q", args[0])
		}
		fl, err := harness.LoadFeatureList(".")
		if err != nil {
			return err
		}
		owner, _ := cmd.Flags().GetString("agent")
		if owner == "" {
			owner = defaultAgentName()
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if err := fl.SetStatus(id, target, owner, now); err != nil {
			return err
		}
		if err := harness.SaveFeatureList(".", fl); err != nil {
			return err
		}
		f := fl.FindByID(id)
		printSuccess(fmt.Sprintf("Feature #%d (%s) → %s", id, f.Name, target))
		return nil
	}
}

// --- add -------------------------------------------------------------------

type harnessAddFlags struct {
	Name        string
	Title       string
	Description string
	Acceptance  []string
}

var harnessAddOpts harnessAddFlags

var harnessAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Append a new feature to the backlog",
	RunE:  runHarnessAdd,
}

func runHarnessAdd(_ *cobra.Command, _ []string) error {
	if strings.TrimSpace(harnessAddOpts.Name) == "" {
		return fmt.Errorf("--name is required")
	}
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	nextID := 1
	for _, f := range fl.Features {
		if f.ID >= nextID {
			nextID = f.ID + 1
		}
	}
	title := harnessAddOpts.Title
	if title == "" {
		title = harnessAddOpts.Name
	}
	fl.Features = append(fl.Features, harness.Feature{
		ID:          nextID,
		Name:        harnessAddOpts.Name,
		Title:       title,
		Description: harnessAddOpts.Description,
		Acceptance:  harnessAddOpts.Acceptance,
		Status:      harness.StatusPending,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err := harness.SaveFeatureList(".", fl); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Feature #%d (%s) added to backlog", nextID, harnessAddOpts.Name))
	return nil
}

// --- helpers ---------------------------------------------------------------

// statusMarker returns a single-character status indicator used by `list`.
func statusMarker(s harness.FeatureStatus) string {
	switch s {
	case harness.StatusDone:
		return "✓"
	case harness.StatusInProgress:
		return "►"
	case harness.StatusBlocked:
		return "✗"
	default:
		return "·"
	}
}

// detectStack inspects the project root for telltale manifest files and
// picks the best matching preset. Falls back to generic when nothing
// matches — leaving the operator free to override with `--stack`.
func detectStack(root string) harness.Stack {
	if fileExists(filepath.Join(root, "go.mod")) || fileExists(filepath.Join(root, "go.work")) {
		return harness.StackGo
	}
	if fileExists(filepath.Join(root, "package.json")) || fileExists(filepath.Join(root, "tsconfig.json")) {
		return harness.StackTS
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "requirements.txt")) {
		return harness.StackPython
	}
	return harness.StackGeneric
}

func isKnownStack(s harness.Stack) bool {
	for _, k := range harness.AllStacks {
		if s == k {
			return true
		}
	}
	return false
}

func joinStacks() string {
	parts := make([]string, 0, len(harness.AllStacks))
	for _, s := range harness.AllStacks {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}

// (fileExists lives in init.go — shared across the cmd package.)

// defaultAgentName returns the value of $KORVA_AGENT, or "cli" when
// unset. Agents that want their name recorded in feature_list.json can
// either pass --agent or export KORVA_AGENT before invoking the CLI.
func defaultAgentName() string {
	if v := strings.TrimSpace(os.Getenv("KORVA_AGENT")); v != "" {
		return v
	}
	return "cli"
}

func init() {
	// init flags
	harnessInitCmd.Flags().StringVar(&harnessInitOpts.Root, "root", ".", "target directory")
	harnessInitCmd.Flags().StringVarP(&harnessInitOpts.Project, "project", "p", "", "project name (defaults to directory basename)")
	harnessInitCmd.Flags().StringVarP(&harnessInitOpts.Description, "description", "d", "", "short blurb for AGENTS.md and feature_list.json")
	harnessInitCmd.Flags().StringVarP(&harnessInitOpts.Stack, "stack", "s", "", "stack preset: "+joinStacks()+" (auto-detect when empty)")
	harnessInitCmd.Flags().BoolVar(&harnessInitOpts.WithSubagents, "with-subagents", true, "also install .claude/agents/{leader,implementer,reviewer}.md")
	harnessInitCmd.Flags().BoolVarP(&harnessInitOpts.Overwrite, "overwrite", "f", false, "replace existing harness files")

	// shared transition flag
	for _, c := range []*cobra.Command{harnessStartCmd, harnessDoneCmd, harnessBlockCmd, harnessReopenCmd} {
		c.Flags().String("agent", "", "agent name to record as owner (defaults to $KORVA_AGENT or 'cli')")
	}

	// status / next flags
	harnessStatusCmd.Flags().BoolVar(&harnessStatusJSON, "json", false, "emit machine-readable JSON")
	harnessNextCmd.Flags().BoolVar(&harnessNextJSON, "json", false, "emit machine-readable JSON")

	// add flags
	harnessAddCmd.Flags().StringVar(&harnessAddOpts.Name, "name", "", "short slug (required)")
	harnessAddCmd.Flags().StringVar(&harnessAddOpts.Title, "title", "", "human-readable title (defaults to name)")
	harnessAddCmd.Flags().StringVar(&harnessAddOpts.Description, "description", "", "longer description")
	harnessAddCmd.Flags().StringSliceVar(&harnessAddOpts.Acceptance, "accept", nil, "acceptance criterion (repeatable)")

	harnessCmd.AddCommand(harnessInitCmd)
	harnessCmd.AddCommand(harnessStatusCmd)
	harnessCmd.AddCommand(harnessListCmd)
	harnessCmd.AddCommand(harnessNextCmd)
	harnessCmd.AddCommand(harnessStartCmd)
	harnessCmd.AddCommand(harnessDoneCmd)
	harnessCmd.AddCommand(harnessBlockCmd)
	harnessCmd.AddCommand(harnessReopenCmd)
	harnessCmd.AddCommand(harnessAddCmd)
}
