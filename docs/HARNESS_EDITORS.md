# Harness — supported AI editors

The Korva harness ships rule templates for seven AI coding editors.
The same `feature_list.json` state machine + MCP tools work
identically across all of them; the per-editor files just teach each
editor's chat surface how to drive the workflow.

| Editor                   | Markers (auto-detected)                                | Files installed                                                          |
| ------------------------ | ------------------------------------------------------ | ------------------------------------------------------------------------ |
| **Claude Code**          | `.claude/`, `CLAUDE.md`                                | `.claude/agents/{leader,implementer,reviewer}.md`                        |
| **Cursor**               | `.cursor/`, `.cursorrules`                             | `.cursor/rules/korva-harness.mdc`                                        |
| **Windsurf**             | `.windsurf/`, `.windsurfrules`                         | `.windsurf/rules/korva-harness.md`                                       |
| **Continue**             | `.continue/`, `.continuerules`                         | `.continuerules`                                                         |
| **GitHub Copilot**       | `.github/copilot-instructions.md`                      | `.github/copilot-instructions.md`                                        |
| **Aider**                | `.aider.conf.yml`, `.aider.conf`, `.aiderignore`, `CONVENTIONS.md` | `.aider.conf.yml`                                              |
| **OpenAI Codex CLI**     | `.codex/`, `.codex/config.toml`                        | `.codex/config.toml` (registers Vault MCP server)                        |

In addition, every harness — regardless of editor — installs the
agent-agnostic universal layer:

- `AGENTS.md` ([open standard](https://agents.md), read by all seven)
- `CHECKPOINTS.md`
- `progress/current.md` + `progress/history.md`
- `feature_list.json`
- `docs/architecture.md`, `docs/conventions.md`, `docs/verification.md`

This is the actual contract; the per-editor file just bootstraps the
editor's prompt with a pointer at `AGENTS.md` and the
`korva harness *` CLI verbs.

---

## Quickstart per editor

### Claude Code

```bash
korva harness init --editors claude
# Claude Code reads CLAUDE.md + AGENTS.md automatically.
# Subagents live in .claude/agents/ — invoke with @leader, etc.
```

### Cursor

```bash
korva harness init --editors cursor
# Cursor picks up .cursor/rules/korva-harness.mdc on every chat.
# Cursor also reads AGENTS.md automatically (https://agents.md).
```

### Windsurf

```bash
korva harness init --editors windsurf
# Cascade reads .windsurf/rules/korva-harness.md on every session.
```

### Continue

```bash
korva harness init --editors continue
# Continue reads .continuerules as the single instruction file.
```

### GitHub Copilot

```bash
korva harness init --editors copilot
# Copilot Chat reads .github/copilot-instructions.md per-repo.
# Note: Copilot does NOT speak MCP; agents must use the `korva
# harness *` CLI to drive the state machine (no `vault_harness_*`
# tool calls).
```

### Aider

```bash
korva harness init --editors aider
# .aider.conf.yml preloads AGENTS.md + CHECKPOINTS.md + progress/
# every time you run `aider` from the repo root. To opt out of one
# of the read-only files, edit the conf.
#
# Aider can drive the harness state machine through the `korva
# harness *` CLI verbs (it's a shell-capable agent) but does not
# yet speak MCP natively.
```

### OpenAI Codex CLI

```bash
korva harness init --editors codex
# .codex/config.toml registers `korva-vault --mode mcp` as a stdio
# MCP server, so an in-editor agent can call vault_harness_* tools
# directly.
#
# Codex reads AGENTS.md from the repo root automatically (the same
# open standard Cursor, Claude, Jules etc. share).
```

---

## Auto-detection rules

`korva harness init` (no `--editors` flag, or `--editors auto`) probes
the target repo for the markers in the table above and installs
templates for every editor that matches. Order matches the
declaration order in `internal/harness/templates.go`.

When **no markers** are present the default is `claude` — Korva's
primary editor and the only one guaranteed to handle the
materialized agent files out of the box.

To see what auto-detection would do without writing anything:

```bash
korva harness detect          # human-readable preview
korva harness detect --json   # CI-friendly output
korva harness detect --sdd    # also list SDD-only files
```

The detect command prints which markers triggered which editor, the
universal layer files, and the per-editor files. A repo with no
markers still gets the fallback preview so the operator sees the
exact set of files `korva harness init` would write.

---

## Multi-editor projects

Many teams have members on different editors. The harness handles
this cleanly:

```bash
korva harness init --editors claude,cursor,aider
```

All three rule files are written; they don't conflict because each
editor reads only its own. The universal `AGENTS.md` stays the
single source of truth and every per-editor rule file points back at
it.

---

## Adding a new editor

Two-step process — see `internal/harness/templates.go` for the
contract:

1. Append a constant + `AllEditors` entry:
   ```go
   const EditorMyEditor Editor = "myeditor"

   var AllEditors = []Editor{ ..., EditorMyEditor }
   ```

2. Add an `editorSpec` row with the markers Korva should look for:
   ```go
   var editorSpecs = []editorSpec{
       ...
       {id: EditorMyEditor, markers: []string{".myeditor", "myrules.md"}},
   }
   ```

3. Ship a template tree under
   `internal/harness/templates/editors/myeditor/`. Files use
   Go `text/template` syntax (`{{.Project}}`, `{{if .SDDMode}}…{{end}}`)
   and the `.tmpl` suffix; non-`.tmpl` files are written verbatim.

4. (Optional) ship an SDD-specific extra under
   `internal/harness/templates/editors-sdd/myeditor/` if the editor
   has a multi-file convention and benefits from a dedicated
   spec-author rule. Single-file editors skip this layer.

5. The drift-pinning tests
   (`TestAllEditorsHasTemplateTree`,
   `TestAllEditorsHasDetectionMarker`,
   `TestValidate_AgentAcceptsAllSupportedEditors`,
   `TestDetectHelpListsAllEditors`)
   will fail until you've completed all of the above. That's by
   design — they're the contract.

PRs welcome at https://github.com/AlcanDev/korva.
