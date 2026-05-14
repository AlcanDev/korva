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
	Root        string
	Project     string
	Description string
	Stack       string
	Editors     string // CSV or "auto" / "none"
	SDD         bool
	Overwrite   bool
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

	editors, err := parseEditorsFlag(harnessInitOpts.Editors, abs)
	if err != nil {
		return err
	}

	written, err := harness.Generate(harness.InitOptions{
		Root:        abs,
		Project:     project,
		Description: harnessInitOpts.Description,
		Stack:       stack,
		Editors:     editors,
		SDD:         harnessInitOpts.SDD,
		Overwrite:   harnessInitOpts.Overwrite,
	})
	if err != nil {
		return err
	}

	editorLabel := "none"
	if len(editors) > 0 {
		parts := make([]string, len(editors))
		for i, e := range editors {
			parts[i] = string(e)
		}
		editorLabel = strings.Join(parts, ", ")
	}
	mode := "standard"
	if harnessInitOpts.SDD {
		mode = "SDD"
	}
	printSuccess(fmt.Sprintf("Harness initialized for %q (stack: %s, editors: %s, mode: %s)", project, stack, editorLabel, mode))
	for _, f := range written {
		fmt.Printf("    + %s\n", f)
	}
	if len(written) == 0 {
		printInfo("No new files written — harness already present (use --overwrite to force).")
	}
	if harnessInitOpts.SDD {
		printInfo("SDD mode: draft `specs/<feature>/{requirements,design,tasks}.md` then run `korva harness ready <id>` before implementing.")
	}
	printInfo("Next: review feature_list.json and run ./init.sh")
	return nil
}

// parseEditorsFlag interprets the --editors string. The accepted values are:
//
//	""        → auto-detect from `root` (DetectEditors), the default
//	"auto"    → same as empty: auto-detect
//	"none"    → install no editor rule files; only the universal layer
//	"a,b,c"   → install exactly the listed editors
//
// Returns an error if the user passes an editor name that isn't in
// harness.AllEditors, so typos surface before any files are written.
func parseEditorsFlag(raw string, root string) ([]harness.Editor, error) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "", "auto":
		return harness.DetectEditors(root), nil
	case "none":
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]harness.Editor, 0, len(parts))
	seen := make(map[harness.Editor]bool, len(parts))
	for _, p := range parts {
		name := harness.Editor(strings.ToLower(strings.TrimSpace(p)))
		if name == "" {
			continue
		}
		if !harness.IsKnownEditor(name) {
			return nil, fmt.Errorf("unknown editor %q — pick from: %s, or use 'auto'/'none'", name, joinEditors())
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out, nil
}

// joinEditors stringifies harness.AllEditors for help text + error messages.
func joinEditors() string {
	parts := make([]string, 0, len(harness.AllEditors))
	for _, e := range harness.AllEditors {
		parts = append(parts, string(e))
	}
	return strings.Join(parts, ", ")
}

// --- detect ----------------------------------------------------------------

type harnessDetectFlags struct {
	Root string
	SDD  bool
	JSON bool
}

var harnessDetectOpts harnessDetectFlags

var harnessDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Show which AI editors are wired in this repo + what `init` would install",
	Long: `Inspect a repository for editor markers (.claude, .cursor, .codex, etc.)
and print which editor templates 'korva harness init --editors auto' would
materialize. Useful before running init, and as a debug aid when auto-
detection picks the wrong editor.

  korva harness detect              # human-readable
  korva harness detect --json       # machine-readable (jq / CI scripts)
  korva harness detect --sdd        # include SDD-only files in the preview`,
	RunE: runHarnessDetect,
}

// detectionPayload is the wire shape for --json. The fields mirror the
// human output one-to-one so a script that scrapes the text view and
// one that reads JSON see the same facts.
type detectionPayload struct {
	Root         string                 `json:"root"`
	SDD          bool                   `json:"sdd"`
	Hits         []detectionHitWire     `json:"hits"`
	DefaultUsed  bool                   `json:"default_used"`
	DefaultLabel string                 `json:"default_label,omitempty"`
	CommonFiles  []string               `json:"common_files"`
	Editors      []editorPreviewPayload `json:"editors"`
}

type detectionHitWire struct {
	Editor string `json:"editor"`
	Marker string `json:"marker"`
}

type editorPreviewPayload struct {
	Editor string   `json:"editor"`
	Files  []string `json:"files"`
}

func runHarnessDetect(_ *cobra.Command, _ []string) error {
	root := harnessDetectOpts.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	hits := harness.DetectEditorsDetailed(abs)
	defaultUsed := len(hits) == 0
	editors := make([]harness.Editor, 0, len(hits))
	for _, h := range hits {
		editors = append(editors, h.Editor)
	}
	if defaultUsed {
		// Mirror DetectEditors' fallback so `init --editors auto`
		// produces the same set the preview promises.
		editors = []harness.Editor{harness.EditorClaude}
	}

	commonFiles, err := harness.CommonFiles()
	if err != nil {
		return fmt.Errorf("list common files: %w", err)
	}

	previews := make([]editorPreviewPayload, 0, len(editors))
	for _, e := range editors {
		files, err := harness.EditorFiles(e, harnessDetectOpts.SDD)
		if err != nil {
			return fmt.Errorf("list files for %s: %w", e, err)
		}
		previews = append(previews, editorPreviewPayload{Editor: string(e), Files: files})
	}

	payload := detectionPayload{
		Root:        abs,
		SDD:         harnessDetectOpts.SDD,
		Hits:        make([]detectionHitWire, 0, len(hits)),
		DefaultUsed: defaultUsed,
		CommonFiles: commonFiles,
		Editors:     previews,
	}
	for _, h := range hits {
		payload.Hits = append(payload.Hits, detectionHitWire{Editor: string(h.Editor), Marker: h.Marker})
	}
	if defaultUsed {
		payload.DefaultLabel = string(harness.EditorClaude)
	}

	if harnessDetectOpts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	// Human-readable view.
	fmt.Printf("Root:  %s\n", payload.Root)
	if payload.SDD {
		fmt.Println("Mode:  SDD preview (--sdd)")
	}
	fmt.Println()

	if defaultUsed {
		fmt.Println("Detected editors: (none — falling back to claude default)")
		fmt.Println("                  No marker matched any of:")
		for _, spec := range allEditorSpecsForHelp() {
			fmt.Printf("                    %-9s %s\n", spec.editor+":", spec.markers)
		}
	} else {
		fmt.Println("Detected editors:")
		for _, h := range hits {
			fmt.Printf("  %-9s ← %s\n", h.Editor, h.Marker)
		}
	}
	fmt.Println()

	fmt.Println("Would install (universal layer — every harness):")
	for _, f := range commonFiles {
		fmt.Printf("  + %s\n", f)
	}
	fmt.Println("  + feature_list.json")
	for _, p := range previews {
		fmt.Println()
		fmt.Printf("Would install (editor: %s):\n", p.Editor)
		if len(p.Files) == 0 {
			fmt.Println("  (no editor-specific files)")
			continue
		}
		for _, f := range p.Files {
			fmt.Printf("  + %s\n", f)
		}
	}
	fmt.Println()
	fmt.Println("Run 'korva harness init' to materialize these files.")
	return nil
}

// editorSpecForHelp is the local shape consumed by allEditorSpecsForHelp
// so we don't have to expose internal state.
type editorSpecForHelp struct {
	editor  string
	markers string
}

// allEditorSpecsForHelp returns a presentation-only view of the
// detection table for the empty-result branch. We can't import the
// unexported editorSpecs slice, so we hand-list the markers here. If
// the harness package adds a new editor and forgets to update this
// list, the help text falls out of sync — `TestDetectHelpListsAllEditors`
// in the CLI tests pins it.
func allEditorSpecsForHelp() []editorSpecForHelp {
	return []editorSpecForHelp{
		{string(harness.EditorClaude), ".claude/, CLAUDE.md"},
		{string(harness.EditorCursor), ".cursor/, .cursorrules"},
		{string(harness.EditorWindsurf), ".windsurf/, .windsurfrules"},
		{string(harness.EditorContinue), ".continue/, .continuerules"},
		{string(harness.EditorCopilot), ".github/copilot-instructions.md"},
		{string(harness.EditorAider), ".aider.conf.yml, .aiderignore, CONVENTIONS.md"},
		{string(harness.EditorCodex), ".codex/, .codex/config.toml"},
	}
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
	Project         string         `json:"project"`
	SDD             bool           `json:"sdd,omitempty"`
	Counts          harness.Counts `json:"counts"`
	InProgress      *featureWire   `json:"in_progress,omitempty"`
	NextID          int            `json:"next_pending_id,omitempty"`
	NextSpecReadyID int            `json:"next_spec_ready_id,omitempty"`
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
	if ready := fl.NextSpecReady(); ready != nil {
		payload.NextSpecReadyID = ready.ID
	}
	payload.SDD = fl.Rules.RequireApprovedSpecToImplement

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
	if payload.SDD {
		fmt.Println("Mode:       SDD")
	}
	fmt.Println()
	fmt.Printf("  pending:     %d\n", c.Pending)
	// spec_ready only renders when the harness is SDD-mode OR the count
	// is non-zero (defensive: a hand-edited file may carry the status).
	if payload.SDD || c.SpecReady > 0 {
		fmt.Printf("  spec_ready:  %d\n", c.SpecReady)
	}
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
	if payload.NextSpecReadyID > 0 {
		fmt.Printf("Awaiting approval: #%d  (run 'korva harness start %d' to begin implementation)\n",
			payload.NextSpecReadyID, payload.NextSpecReadyID)
	}
	if payload.NextID > 0 {
		fmt.Printf("Next pending: #%d  (run 'korva harness next' to see acceptance criteria)\n", payload.NextID)
	} else if payload.NextSpecReadyID == 0 {
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
	SDD         bool
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
		SDD:         harnessAddOpts.SDD,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err := harness.SaveFeatureList(".", fl); err != nil {
		return err
	}
	mode := ""
	if harnessAddOpts.SDD {
		mode = " (SDD-gated — draft specs/<name>/* and run `korva harness ready` before implementing)"
	}
	printSuccess(fmt.Sprintf("Feature #%d (%s) added to backlog%s", nextID, harnessAddOpts.Name, mode))
	return nil
}

// --- spec ------------------------------------------------------------------

var harnessSpecOverwrite bool

var harnessSpecCmd = &cobra.Command{
	Use:   "spec <id>",
	Short: "Materialize specs/<feature>/{requirements,design,tasks}.md for an SDD feature",
	Args:  cobra.ExactArgs(1),
	RunE:  runHarnessSpec,
}

func runHarnessSpec(_ *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("id must be an integer, got %q", args[0])
	}
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	f := fl.FindByID(id)
	if f == nil {
		return fmt.Errorf("feature %d not found", id)
	}
	if !f.SDD {
		return fmt.Errorf("feature #%d (%s) is not SDD-flagged — add it with --sdd or set sdd:true in feature_list.json", id, f.Name)
	}
	res, err := harness.MaterializeSpec(".", f, harnessSpecOverwrite)
	if err != nil {
		return err
	}
	if len(res.Written) > 0 {
		printSuccess(fmt.Sprintf("Spec materialized at %s", res.Dir))
		for _, name := range res.Written {
			fmt.Printf("    + %s\n", name)
		}
	}
	for _, name := range res.Skipped {
		printInfo(fmt.Sprintf("Kept existing %s/%s (use --overwrite to replace)", res.Dir, name))
	}
	if len(res.Written) == 0 && len(res.Skipped) == len(harness.SpecFiles) {
		printInfo("All three spec files already exist; nothing to do.")
	}
	printInfo(fmt.Sprintf("When ready: korva harness ready %d", id))
	return nil
}

// --- ready -----------------------------------------------------------------
// `korva harness ready <id>` is the SDD spec_author → human handoff. It
// transitions pending → spec_ready *only* when the three spec files
// already exist on disk — preventing accidental approvals of an empty
// scaffold.

var harnessReadyCmd = &cobra.Command{
	Use:   "ready <id>",
	Short: "Mark a SDD feature's spec as ready for human review (pending → spec_ready)",
	Args:  cobra.ExactArgs(1),
	RunE:  runHarnessReady,
}

func runHarnessReady(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("id must be an integer, got %q", args[0])
	}
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	f := fl.FindByID(id)
	if f == nil {
		return fmt.Errorf("feature %d not found", id)
	}
	if !f.SDD {
		return fmt.Errorf("feature #%d (%s) is not SDD-flagged — the ready step only applies to SDD features", id, f.Name)
	}
	if !harness.SpecComplete(".", f.Name) {
		return fmt.Errorf("spec files missing — run `korva harness spec %d` to scaffold them, then draft them before marking ready", id)
	}
	owner, _ := cmd.Flags().GetString("agent")
	if owner == "" {
		owner = defaultAgentName()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := fl.SetStatus(id, harness.StatusSpecReady, owner, now); err != nil {
		return err
	}
	if err := harness.SaveFeatureList(".", fl); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Feature #%d (%s) → spec_ready", id, f.Name))
	printInfo("Awaiting human approval. Reviewer runs `korva harness start " + args[0] + "` to begin implementation.")
	return nil
}

// --- check -----------------------------------------------------------------
// `korva harness check` runs every harness invariant and exits non-zero
// when any error-severity issue is found. init.sh shells out to it; CI
// scripts can pipe the JSON form into jq.

var harnessCheckJSON bool

var harnessCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate the harness against every invariant (schema + SDD + spec coverage)",
	RunE:  runHarnessCheck,
}

func runHarnessCheck(_ *cobra.Command, _ []string) error {
	report, err := harness.Check(".")
	if err != nil {
		return err
	}
	if harnessCheckJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Print(harness.FormatReport(report))
	}
	if !report.OK {
		// Distinct sentinel error so callers (init.sh) can rely on the
		// non-zero exit code without parsing stderr.
		return fmt.Errorf("harness check failed — %d issue(s)", len(report.Issues))
	}
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
	case harness.StatusSpecReady:
		return "✎"
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

// --- review ---------------------------------------------------------------
// `korva harness review <id>` lints the spec files for an SDD feature
// against the EARS + traceability + R↔T coverage contract. Exits
// non-zero when any error-severity issue is found so it can be wired
// into CI or pre-merge hooks.

type harnessReviewFlags struct {
	JSON     bool
	Record   bool
	Verdict  string
	Reviewer string
	Note     string
}

var harnessReviewOpts harnessReviewFlags

var harnessReviewCmd = &cobra.Command{
	Use:   "review <id>",
	Short: "Lint an SDD feature's spec (EARS validity + R↔T traceability + acceptance coverage)",
	Long: `Runs the EARS linter + R↔T traceability + acceptance-coverage check.

Without --record the command is pure-read: it prints the report and
exits non-zero on errors so CI can gate on it.

With --record (Phase 18.A) the verdict derived from the report is
persisted under the feature's "review" field in feature_list.json.
The default verdict comes from the linter outcome (clean → approve,
warnings → needs_fixes, errors → reject); the reviewer can override
with --verdict <approve|needs_fixes|reject>. Recording NEVER changes
the feature's status — the operator keeps discretion to start (or
not) regardless of the verdict.`,
	Args: cobra.ExactArgs(1),
	RunE: runHarnessReview,
}

func runHarnessReview(_ *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("id must be an integer, got %q", args[0])
	}
	report, err := harness.ReviewSpec(".", id)
	if err != nil {
		return err
	}
	if harnessReviewOpts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Print(harness.FormatSpecReviewReport(report))
	}
	if harnessReviewOpts.Record {
		if err := recordReview(id, report); err != nil {
			return fmt.Errorf("record verdict: %w", err)
		}
	}
	if !report.OK && !harnessReviewOpts.Record {
		// --record is the "I'm taking responsibility for this verdict"
		// signal; the reviewer can record a reject and still exit 0 so
		// CI hooks behave predictably. Without --record, surface the
		// failure as a non-zero exit so scripted callers notice.
		return fmt.Errorf("spec review failed — %d issue(s)", len(report.Issues))
	}
	return nil
}

// recordReview persists the verdict to feature_list.json. The verdict
// is either derived from the report (default) or overridden via
// --verdict. --reviewer / --note come from flags or env fallbacks.
func recordReview(id int, report *harness.SpecReviewReport) error {
	verdict := report.Verdict()
	if override := strings.TrimSpace(harnessReviewOpts.Verdict); override != "" {
		v := harness.ReviewVerdict(strings.ToLower(override))
		if !harness.IsKnownReviewVerdict(v) {
			return fmt.Errorf("unknown verdict %q — pick approve | needs_fixes | reject", override)
		}
		verdict = v
	}
	reviewer := strings.TrimSpace(harnessReviewOpts.Reviewer)
	if reviewer == "" {
		reviewer = defaultAgentName()
	}
	errs, _ := report.CountBySeverity()
	dec := harness.ReviewDecision{
		Verdict:    verdict,
		Reviewer:   reviewer,
		At:         time.Now().UTC().Format(time.RFC3339),
		IssueCount: len(report.Issues),
		ErrorCount: errs,
		Note:       strings.TrimSpace(harnessReviewOpts.Note),
	}
	fl, err := harness.LoadFeatureList(".")
	if err != nil {
		return err
	}
	if err := fl.RecordReview(id, dec); err != nil {
		return err
	}
	if err := harness.SaveFeatureList(".", fl); err != nil {
		return err
	}
	if !harnessReviewOpts.JSON {
		fmt.Printf("\nRecorded verdict: %s (by %s)\n", dec.Verdict, dec.Reviewer)
	}
	return nil
}

// --- ci install ------------------------------------------------------------
// `korva harness ci install --provider=<X>` materializes a ready-to-use
// CI workflow that gates merges on `korva harness check`. Two providers
// ship out of the box: github-actions and gitlab-ci.

var harnessCICmd = &cobra.Command{
	Use:   "ci",
	Short: "Manage CI/CD integration for the harness",
	Long: `CI integration drops a ready-to-use workflow into the repo that runs
'korva harness check' on every PR / MR and posts the backlog summary as
a comment. Use 'korva harness ci install --provider=<X>' to materialize
the templates for your CI vendor.`,
}

type harnessCIInstallFlags struct {
	Provider  string
	Root      string
	Overwrite bool
}

var harnessCIInstallOpts harnessCIInstallFlags

var harnessCIInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Materialize a CI workflow that gates merges on `korva harness check`",
	RunE:  runHarnessCIInstall,
}

func runHarnessCIInstall(_ *cobra.Command, _ []string) error {
	provider := harness.CIProvider(strings.ToLower(strings.TrimSpace(harnessCIInstallOpts.Provider)))
	if provider == "" {
		// Auto-detect: prefer GitHub when a `.github/` dir exists, else
		// GitLab when `.gitlab-ci.yml` is present, else fail with an
		// actionable error.
		root := harnessCIInstallOpts.Root
		if root == "" {
			root = "."
		}
		switch {
		case fileExists(filepath.Join(root, ".github")):
			provider = harness.CIGitHubActions
		case fileExists(filepath.Join(root, ".gitlab-ci.yml")):
			provider = harness.CIGitLab
		default:
			return fmt.Errorf("could not auto-detect CI provider — pass --provider with one of: %s", joinCIProviders())
		}
	}
	if !harness.IsKnownCIProvider(provider) {
		return fmt.Errorf("unknown provider %q — pick one of: %s", provider, joinCIProviders())
	}
	root := harnessCIInstallOpts.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	res, err := harness.InstallCI(abs, provider, harnessCIInstallOpts.Overwrite)
	if err != nil {
		return err
	}
	if len(res.Written) > 0 {
		printSuccess(fmt.Sprintf("CI installed for %q", provider))
		for _, f := range res.Written {
			fmt.Printf("    + %s\n", f)
		}
	}
	for _, f := range res.Skipped {
		printInfo(fmt.Sprintf("Kept existing %s (use --overwrite to replace)", f))
	}
	if len(res.Written) == 0 && len(res.Skipped) > 0 {
		printInfo("Nothing to do — workflow already present.")
	}
	switch provider {
	case harness.CIGitHubActions:
		printInfo("Commit the workflow and push: GitHub will run it on the next PR.")
	case harness.CIGitLab:
		printInfo("Add `KORVA_GITLAB_TOKEN` (Maintainer scope, api) as a project CI/CD variable to enable MR comments.")
	}
	return nil
}

// joinCIProviders stringifies harness.AllCIProviders for help / errors.
func joinCIProviders() string {
	parts := make([]string, 0, len(harness.AllCIProviders))
	for _, p := range harness.AllCIProviders {
		parts = append(parts, string(p))
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
	harnessInitCmd.Flags().StringVar(&harnessInitOpts.Editors, "editors", "auto",
		"editor rule files to install: comma-separated list ("+joinEditors()+"), 'auto' to detect, or 'none'")
	harnessInitCmd.Flags().BoolVar(&harnessInitOpts.SDD, "sdd", false,
		"enable Spec-Driven Development mode: features must be drafted as specs/<name>/* and approved before implementation")
	harnessInitCmd.Flags().BoolVarP(&harnessInitOpts.Overwrite, "overwrite", "f", false, "replace existing harness files")

	// shared transition flag — the ready command joins the start/done/block/reopen
	// family because it also records OwnerAgent + UpdatedAt.
	for _, c := range []*cobra.Command{harnessStartCmd, harnessDoneCmd, harnessBlockCmd, harnessReopenCmd, harnessReadyCmd} {
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
	harnessAddCmd.Flags().BoolVar(&harnessAddOpts.SDD, "sdd", false, "mark as SDD-gated (requires spec drafting + ready approval before implementation)")

	// spec flags
	harnessSpecCmd.Flags().BoolVarP(&harnessSpecOverwrite, "overwrite", "f", false, "replace existing spec files (operator content is lost)")

	// check flags
	harnessCheckCmd.Flags().BoolVar(&harnessCheckJSON, "json", false, "emit machine-readable JSON")

	// review flags
	harnessReviewCmd.Flags().BoolVar(&harnessReviewOpts.JSON, "json", false, "emit machine-readable JSON")
	harnessReviewCmd.Flags().BoolVar(&harnessReviewOpts.Record, "record", false,
		"persist the verdict to feature_list.json (Phase 18.A)")
	harnessReviewCmd.Flags().StringVar(&harnessReviewOpts.Verdict, "verdict", "",
		"override the derived verdict: approve | needs_fixes | reject (requires --record)")
	harnessReviewCmd.Flags().StringVar(&harnessReviewOpts.Reviewer, "reviewer", "",
		"reviewer identifier recorded with the verdict (defaults to $KORVA_AGENT or 'cli')")
	harnessReviewCmd.Flags().StringVar(&harnessReviewOpts.Note, "note", "",
		"optional one-line note to surface in the dashboard (requires --record)")

	// ci install flags
	harnessCIInstallCmd.Flags().StringVar(&harnessCIInstallOpts.Provider, "provider", "",
		"CI vendor: "+joinCIProviders()+" (auto-detect when empty)")
	harnessCIInstallCmd.Flags().StringVar(&harnessCIInstallOpts.Root, "root", ".", "target repository root")
	harnessCIInstallCmd.Flags().BoolVarP(&harnessCIInstallOpts.Overwrite, "overwrite", "f", false,
		"replace an existing workflow file (operator edits lost)")
	harnessCICmd.AddCommand(harnessCIInstallCmd)

	// detect flags
	harnessDetectCmd.Flags().StringVar(&harnessDetectOpts.Root, "root", ".", "target directory to inspect")
	harnessDetectCmd.Flags().BoolVar(&harnessDetectOpts.SDD, "sdd", false, "include SDD-only files in the preview")
	harnessDetectCmd.Flags().BoolVar(&harnessDetectOpts.JSON, "json", false, "emit machine-readable JSON")

	harnessCmd.AddCommand(harnessInitCmd)
	harnessCmd.AddCommand(harnessDetectCmd)
	harnessCmd.AddCommand(harnessStatusCmd)
	harnessCmd.AddCommand(harnessListCmd)
	harnessCmd.AddCommand(harnessNextCmd)
	harnessCmd.AddCommand(harnessStartCmd)
	harnessCmd.AddCommand(harnessDoneCmd)
	harnessCmd.AddCommand(harnessBlockCmd)
	harnessCmd.AddCommand(harnessReopenCmd)
	harnessCmd.AddCommand(harnessAddCmd)
	harnessCmd.AddCommand(harnessSpecCmd)
	harnessCmd.AddCommand(harnessReadyCmd)
	harnessCmd.AddCommand(harnessCheckCmd)
	harnessCmd.AddCommand(harnessReviewCmd)
	harnessCmd.AddCommand(harnessCICmd)
}
