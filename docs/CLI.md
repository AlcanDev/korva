# CLI Reference

> Every Korva command, every flag, with examples.

*Last updated: 2026-04-30*

---

## Top-level commands

```
korva <command> [flags] [args...]

Commands:
  init       Bootstrap ~/.korva/ and start the vault
  setup      Wire one editor for the current project
  status     Show running services and license
  doctor     Health-check + diagnose problems
  sync       Force a Hive sync now
  lore       Manage knowledge scrolls
  sentinel   Run / install the architecture validator
  admin      Server-side admin commands
  hive       Hive sync admin
  license    Activate / deactivate / status
  teams      Teams admin (RBAC, members)
  auth       Authentication helpers
  vault      Direct vault CRUD
  update     Self-update binary
  obs        Observability (logs, metrics)
  skills     Smart Skill Loader (Teams+)
  completion Generate shell completion scripts

Global flags:
  --version    Print version + commit + build date
  -h, --help   Show help for any command
```

---

## `korva init`

Bootstrap a fresh installation.

```bash
korva init                 # interactive
korva init --non-interactive --team my-team
korva init --profile devops
```

Effects:
- Creates `~/.korva/` with default config
- Generates `install.id` (stable per-machine ID)
- Initialises the SQLite vault
- Optionally installs a team profile from `lore/private/`

Idempotent — safe to re-run.

---

## `korva setup`

Wire one or more AI editors to use the Korva Vault MCP server. Run without
flags to auto-detect every installed editor and configure them all at once.

```bash
korva setup                         # auto-detect + configure everything
korva setup --vscode --cursor       # configure a specific subset
korva setup --gemini-cli            # Google Gemini CLI
korva setup --opencode              # OpenCode (open-source AI agent)
korva setup --codex                 # OpenAI Codex CLI
korva setup --global                # write only global editor settings
korva setup --local                 # write only workspace files
korva setup --force                 # overwrite even if already configured
```

Supported targets (each behind its own flag):

| Flag | Config file |
|---|---|
| `--vscode`     | `~/Library/Application Support/Code/User/settings.json` + `.vscode/mcp.json` |
| `--cursor`     | `~/.cursor/mcp.json` |
| `--claude`     | `~/.claude/settings.json` |
| `--gemini-cli` | `~/.gemini/settings.json` |
| `--opencode`   | `~/.config/opencode/opencode.json` |
| `--codex`      | `~/.codex/config.toml` |

Each writer is idempotent: re-running with the same flags is safe.

---

## `korva status`

Show what's running and the license state.

```bash
korva status
```

Sample output:
```
Korva Vault          ✓ running on :7437  (PID 14523)
License              ✓ Teams (expires 2027-04-30)
Hive sync            ✓ last push 4m ago — 0 queued
Smart Skill Loader   ✓ enabled (24 scrolls indexed)
Beacon dashboard     → http://localhost:7437
```

---

## `korva doctor`

Diagnose common problems.

```bash
korva doctor
korva doctor --fix       # apply suggested auto-fixes
```

Checks:
- `~/.korva/` permissions (admin.key must be 0600)
- Port 7437 is reachable
- Vault binary is on PATH and matches CLI version
- Editor manifests in the current project are valid
- License signature is intact

---

## `korva sync`

Force an immediate Hive sync without waiting for the worker tick.

```bash
korva sync
korva sync --dry-run     # show what would be pushed without pushing
```

---

## `korva lore`

Manage knowledge scrolls.

```bash
korva lore list                       # all scrolls
korva lore list --team backend        # filter by team
korva lore info skill-authoring       # show metadata
korva lore add my-team-scroll         # install a private scroll
korva lore export                     # dump all scrolls as JSON
korva lore search "JWT rotation"      # full-text search
```

---

## `korva projects`

Inspect, consolidate, and clean up project namespaces.

```bash
korva projects list                                       # inventory + counts
korva projects suggest                                    # merge candidates
korva projects consolidate --canonical X --source A --source B
korva projects prune                                      # dry-run (default)
korva projects prune --apply                              # actually delete
```

Use this when a single team has accumulated variants (`alpha` vs `Alpha`,
`my-project` vs `my_project`) or orphan sessions from abandoned MCP runs.
`suggest` groups projects whose names normalize to the same canonical form
and proposes the variant with the most observations as the target;
`consolidate` folds the sources into the canonical name; `prune` drops
sessions whose project has zero observations.

All four subcommands require an admin key — run `korva init --admin` first.

---

## `korva export obsidian`

Render the vault as Obsidian-flavored markdown.

```bash
korva export obsidian --out ~/vaults/korva
korva export obsidian --out /tmp/scoped --project korva    # one project
korva export obsidian --out /tmp/scoped --type decision    # one type
```

Output layout:

```
<out>/
  README.md                     ← root index of every project
  <project>/_index.md           ← per-project index, grouped by type
  <project>/<type>/<slug>.md    ← one note per observation
```

Each note carries a YAML frontmatter block, the title as H1, the content,
and a Related section with `[[wikilinks]]` to every reachable relation.
The slug picks `topic_key` when present (stable across re-saves), else the
last 8 chars of the ULID. Re-running over the same output directory is
safe — note files are byte-stable; only the root index timestamp moves.

Open the output folder in Obsidian via *File → Open vault → choose folder*.

---

## `korva sentinel`

Pre-commit architecture validator.

```bash
korva sentinel install --hook pre-commit    # wire the git hook
korva sentinel install --hook post-commit   # auto-save on commit
korva sentinel run                          # run on staged files manually
korva sentinel run --format json            # machine-readable output
korva sentinel rules                        # list active rules
```

The pre-commit hook reads filenames from stdin (standard pre-commit protocol), validates them, and exits non-zero on violations.

---

## `korva license`

```bash
korva license status                          # current state
korva license activate <key>                  # activate Teams license
korva license deactivate                      # deactivate this machine
korva license refresh                         # force a heartbeat now
```

The license is RS256-signed JWS. Verification happens locally — no network round-trip required for normal use. Heartbeat is a soft check that runs every 24h.

---

## `korva teams` (Teams+)

```bash
korva teams members                  # list seats
korva teams invite <email>           # invite a teammate
korva teams revoke <email>           # remove a seat
korva teams role <email> <role>      # set RBAC role
```

---

## `korva auth`

Helpers for authenticated flows.

```bash
korva auth whoami          # show current identity
korva auth login           # SSO / OAuth flow (where supported)
korva auth logout          # clear local credentials
```

---

## `korva vault`

Direct CRUD against the vault — useful for scripting.

```bash
korva vault save --project my-app --type decision --title "..." --content "..."
korva vault search --project my-app --query "JWT"
korva vault context --project my-app
korva vault timeline --project my-app --from 2026-01-01 --to 2026-04-30
korva vault summary --project my-app
korva vault export --project my-app > backup.json
korva vault import --project my-app --file backup.json
korva vault purge --project my-app --before 2025-01-01
```

---

## `korva update`

SHA256-verified self-update.

```bash
korva update            # update to latest stable
korva update --check    # show available without updating
korva update --pin v1.0.0
```

The update process:
1. Fetches the latest release manifest from GitHub
2. Downloads the platform-specific archive
3. Verifies the SHA256 against the published checksums file
4. Atomically swaps the running binary with the new one

Disable the periodic check with `KORVA_NO_UPDATE_CHECK=1`.

---

## `korva obs`

Observability — logs, metrics, traces.

```bash
korva obs logs              # tail vault logs
korva obs logs --since 1h
korva obs metrics           # Prometheus-format metrics
korva obs traces            # recent OpenTelemetry traces
```

---

## `korva skills` (Teams+)

Inspect the Smart Skill Loader.

```bash
korva skills match "rotate JWT keys"     # see what would auto-load
korva skills index                       # rebuild the trigger index
korva skills disable observability       # temporarily disable a scroll
korva skills enable observability
```

---

## `korva completion`

```bash
korva completion bash > /usr/local/etc/bash_completion.d/korva
korva completion zsh > /usr/local/share/zsh/site-functions/_korva
korva completion fish > ~/.config/fish/completions/korva.fish
```

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `KORVA_VAULT_HOST` | Bind address for `korva-vault --mode http` (default `127.0.0.1`, set `0.0.0.0` for Docker) |
| `KORVA_VAULT_PORT` | Override default port `7437` |
| `KORVA_VAULT_DB` | Override SQLite path |
| `KORVA_VAULT_MODE` | `mcp` \| `http` \| `both` \| `tui` |
| `KORVA_LICENSING_ENDPOINT` | Override the licensing server URL |
| `KORVA_HIVE_ENDPOINT` | Override the Hive sync server URL |
| `KORVA_EMAIL_API_KEY` | Resend API key (for license email events) |
| `KORVA_EMAIL_FROM` | Sender address for license emails |
| `KORVA_NO_UPDATE_CHECK` | Set to `1` to disable the 24h update-check ping |

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Generic error (see stderr) |
| `2` | Invalid usage / bad flags |
| `3` | License required for this command |
| `4` | Vault unreachable |
| `5` | Network error (Hive / licensing) |

Use these in shell scripts to branch on outcome:

```bash
if korva sentinel run; then
  echo "architecture clean"
else
  case $? in
    1) echo "violations found" ;;
    4) echo "vault not running — start with korva init" ;;
  esac
fi
```
