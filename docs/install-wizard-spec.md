# Korva Install Wizard — UI Integration Spec

> Context document for integrating an interactive **install / configure /
> update wizard** into the Korva landing site. The wizard takes the user
> through a series of choices (button-driven), composes a single shell
> command that fully reflects those choices, and exposes a library of
> "quick commands" for day-2 operations.
>
> **Companion file:** [`install-wizard-manifest.json`](./install-wizard-manifest.json)
> contains the same data in machine-readable form so the UI can render
> it without hard-coding flags, env vars or labels.
>
> Sources of truth used to build this spec (verified, not invented):
> - `install.sh` / `install.ps1` (env vars, archive layout)
> - `cli/internal/cmd/*.go` (every Cobra command and flag)
> - `vault/cmd/korva-vault/main.go` (vault server flags)
> - `internal/config/schema.go` (config fields and defaults)
> - `integrations/` (editor manifests)
> - `README.md` install section

---

## 1. Goal

Let a user assemble a **fully personalised install + setup recipe** by
clicking through a guided UI, then copy a single command (or a small
script block) and paste it in their terminal. Plus: surface common
post-install commands ("status", "doctor", "update"…) as one-click
copyable snippets.

The wizard **never executes** anything itself — it only **composes
strings**. The user runs them in their own shell. This keeps the landing
fully static and avoids any auth / sandbox / privilege concerns.

---

## 2. UX skeleton (steps)

The wizard is a linear stepper. Steps 1–6 are the install path; 7+ are
post-install enrichment. Steps marked *(advanced)* should collapse
behind a "Show advanced" toggle.

| # | Step | Required | Output contributes to |
|---|------|----------|------------------------|
| 1 | Platform — macOS / Linux / Windows | yes | shell dialect, install one-liner |
| 2 | Install method — Curl one-liner / Homebrew / Docker / Manual | yes | top of script |
| 3 | Version — latest / pinned (vX.Y.Z) | yes | `KORVA_VERSION` |
| 4 | Components — `korva` (always) / `korva-sentinel` (always) / `korva-vault` (toggle) | yes | `KORVA_NO_VAULT` |
| 5 | Install directory — auto / custom path | yes | `KORVA_INSTALL_DIR` |
| 6 | Editor / agent integrations — multi-select | no | `korva setup …` lines |
| 7 | Project init — run `korva init`? + admin / profile / agent | no | `korva init …` line |
| 8 | Mode preset — Cloud / Offline / Cloud-only / Claude-only / Custom | no | toggles env vars + post-install lines |
| 9 | Vault server *(advanced)* — port / host / mode / db | no | `korva config set vault.*` lines |
| 10 | Privacy & policy *(advanced)* — Hive on/off, retention, scrolls, block-on-violation | no | post-install `korva hive …`, `korva config set …` |
| 11 | Review — preview the generated script | yes | — |
| 12 | Quick commands gallery — copyable, grouped by purpose | n/a | independent of install path |

---

## 3. Mode presets (step 8) — the killer shortcut

These are one-click bundles that pre-fill the toggles. They map to
real flags / commands; nothing magical.

### `cloud` (default)
Everything on. Hive sync enabled. Vault HTTP+MCP. Update checks on.
- env: *(none extra)*
- post-install: *(none)*

### `offline` / air-gapped
For machines with no internet after install.
- env: `KORVA_NO_UPDATE_CHECK=1`
- post-install:
  - `korva hive disable`
  - `korva config set vault.auto_sync false`

### `cloud-only` (no local memory, MCP only)
For users who only want the AI-assistant integration but not a local
vault DB. Vault still runs in MCP-only mode (no HTTP).
- env: `KORVA_NO_VAULT=yes` *(skips vault binary entirely)* OR run
  `korva-vault --mode=mcp` if they keep the binary.
- post-install: `korva config set vault.auto_start false`

### `claude-only`
Minimal install for Claude Code users. Only one editor wiring.
- editors: `claude-code` only
- post-install: `korva setup claude-code`

### `custom`
Don't touch anything. The user picks every toggle by hand.

---

## 4. Command-generation logic

Pseudocode (runs in the browser, no backend):

```ts
function buildScript(opts: WizardState): string {
  const lines: string[] = [];
  const isWin = opts.platform === "windows";
  const envPrefix = isWin ? "$env:" : "";
  const setEnv = (k: string, v: string) =>
    isWin ? `$env:${k} = "${v}"` : `export ${k}=${shellQuote(v)}`;

  // ---- 1. Pre-install env vars (KORVA_* read by install.sh / install.ps1)
  if (opts.version !== "latest")     lines.push(setEnv("KORVA_VERSION", opts.version));
  if (opts.installDir)               lines.push(setEnv("KORVA_INSTALL_DIR", opts.installDir));
  if (!opts.components.vault)        lines.push(setEnv("KORVA_NO_VAULT", "yes"));
  if (opts.preset === "offline")     lines.push(setEnv("KORVA_NO_UPDATE_CHECK", "1"));

  // ---- 2. Installer one-liner
  if (opts.method === "curl") {
    lines.push(isWin
      ? `irm https://korva.dev/install.ps1 | iex`
      : `curl -fsSL https://korva.dev/install | bash`);
  } else if (opts.method === "brew") {
    lines.push(`brew install alcandev/tap/korva`);
  } else if (opts.method === "docker") {
    lines.push(`docker compose -f docker-compose.yml up -d korva-vault`);
  }

  // ---- 3. korva init (optional)
  if (opts.runInit) {
    const flags = [];
    if (opts.init.admin)        flags.push("--admin");
    if (opts.init.owner)        flags.push(`--owner ${shellQuote(opts.init.owner)}`);
    if (opts.init.profileRepo)  flags.push(`--profile ${shellQuote(opts.init.profileRepo)}`);
    if (opts.init.agent)        flags.push(`--agent ${opts.init.agent}`);
    lines.push(`korva init ${flags.join(" ")}`.trim());
  }

  // ---- 4. korva setup <editor> for each selected
  for (const ed of opts.editors) {
    const flags = [];
    if (opts.editorScope === "global") flags.push("--global");
    if (opts.editorScope === "local")  flags.push("--local");
    if (opts.editorForce)              flags.push("--force");
    lines.push(`korva setup ${ed} ${flags.join(" ")}`.trim());
  }

  // ---- 5. Preset / advanced post-install
  if (opts.preset === "offline" || opts.hive === "off") lines.push("korva hive disable");
  if (opts.sentinel.installHook)                        lines.push("korva sentinel install");
  if (opts.skills.installHook)                          lines.push("korva skills hook install");
  for (const s of opts.lore.add)    lines.push(`korva lore add ${s}`);
  for (const s of opts.lore.remove) lines.push(`korva lore remove ${s}`);
  for (const [k, v] of Object.entries(opts.configSets))
    lines.push(`korva config set ${k} ${shellQuote(String(v))}`);

  // ---- 6. Sanity check at the end
  lines.push("korva doctor");

  return lines.join(isWin ? "\n" : " && \\\n  ");
}
```

### Shell dialect rules

| Concern | bash / zsh | PowerShell |
|---------|------------|-------------|
| Set env var inline | `KEY=value cmd` | `$env:KEY = "value"; cmd` |
| Multi-line continuation | `\` at EOL | newline |
| Quoting paths with spaces | `'…'` | `"…"` |
| Run installer | `curl -fsSL … \| bash` | `irm … \| iex` |

The wizard **must** detect the platform first and switch the entire
output between dialects — don't try to be clever and ship a hybrid.

---

## 5. Worked examples

### Example A — "Cloud, default, Claude Code on macOS"
User picks: macOS · curl · latest · all components · auto dir · `claude-code`.

```bash
curl -fsSL https://korva.dev/install | bash && \
  korva init && \
  korva setup claude-code && \
  korva doctor
```

### Example B — "Offline pinned install, no vault, on Linux"
User picks: Linux · curl · `v0.5.0` · vault off · `/opt/korva/bin` ·
preset=offline · no editors · no init.

```bash
export KORVA_VERSION=v0.5.0 && \
  export KORVA_INSTALL_DIR=/opt/korva/bin && \
  export KORVA_NO_VAULT=yes && \
  export KORVA_NO_UPDATE_CHECK=1 && \
  curl -fsSL https://korva.dev/install | bash && \
  korva hive disable && \
  korva config set vault.auto_sync false && \
  korva doctor
```

### Example C — "Windows, full team setup"
User picks: Windows · irm · latest · all components · default dir ·
admin init with team profile · Cursor + VS Code + Claude Code.

```powershell
irm https://korva.dev/install.ps1 | iex
korva init --admin --owner "ops@acme.com" --profile "git@github.com:acme/korva-profile.git"
korva setup cursor --global
korva setup vscode --global
korva setup claude-code --global
korva sentinel install
korva doctor
```

---

## 6. Quick-commands gallery (step 12)

A grid/list of curated copy-buttons grouped by intent. The full
catalogue with descriptions is in `install-wizard-manifest.json`
under `quick_commands`. High-level groups:

- **Health** — `korva status`, `korva doctor`, `korva vault status`,
  `korva config list`
- **Vault server** — `korva vault start`, `korva vault stop`,
  `korva vault logs`, `korva vault clean --dry-run`
- **Updates** — `korva update --check`, `korva update --changelog`,
  `korva update --yes`
- **Knowledge (Lore / Obs)** — `korva lore list`,
  `korva lore add <scroll>`, `korva obs search "…" --cloud`,
  `korva obs list --type decision`
- **Sentinel** — `korva sentinel install`, `korva sentinel check`
- **Skills** — `korva skills sync`, `korva skills hook install`,
  `korva skills list`
- **Auth & Teams** — `korva auth redeem <token>`, `korva auth status`,
  `korva teams sync`, `korva teams invite <email> --team <id>`
- **License** — `korva license activate <key>`,
  `korva license status`, `korva license deactivate`
- **Hive (community brain)** — `korva hive status`,
  `korva hive enable` / `disable`, `korva hive push`
- **Admin (requires `~/.korva/admin.key`)** —
  `korva admin init --owner <email>`, `korva admin rotate-key`
- **Config** — `korva config get <key>`,
  `korva config set <key> <value>` (`--global` / `--local`)

Each card needs: **title**, **command**, **one-line description**, an
optional **placeholder list** (e.g. `<token>`, `<email>`) so the UI can
render input fields, and a **tier badge** (community / teams / admin).

---

## 7. Constraints the UI must enforce

These come from real behaviour in the codebase — violating them
produces installs that fail at runtime.

- **Homebrew** is macOS / Linux only. Hide the option on Windows.
- **`KORVA_NO_VAULT=yes`** removes the vault binary. If the user later
  picks `cloud-only` mode that needs MCP, the wizard must re-enable
  the binary or warn.
- **`korva init --admin`** requires `--owner <email>`. The UI must
  validate and block "Next" if owner is missing.
- **`korva setup --global` and `--local`** are mutually exclusive
  (one-of). `--all` overrides editor-specific flags.
- **`korva auth redeem`** requires the vault to be running. If the
  user disables the vault, hide the "redeem invite" quick command.
- **`korva license activate`** requires `install.id`, which is
  generated by `korva init`. Don't expose it before init.
- **Admin commands** should be hidden behind an "I have admin.key"
  toggle — they always fail without it.
- **AI agent values** for `--agent` / `agent` config: only
  `copilot`, `claude`, `cursor` are accepted today.
- **Vault `--mode`** values: `mcp`, `http`, `both`, `tui`.
- **Active scrolls**: validated as plain names (no `/`, no `.`). The
  UI's scroll picker should reject invalid characters.

---

## 8. What the UI needs from us (already in the manifest)

- `platforms[]` with shell dialect metadata.
- `install_methods[]` per platform.
- `env_vars[]` understood by the installer scripts.
- `components[]` with sizes / descriptions / required flag.
- `editors[]` (slug, label, icon hint, setup-command).
- `presets[]` with the toggle deltas they apply.
- `commands[]` — every `korva` subcommand with its flags, types,
  defaults, tier and a human description.
- `config_keys[]` — every `korva config set` key with type, default,
  enum values where applicable.
- `quick_commands[]` — curated day-2 commands grouped by category.
- `validation[]` — cross-field rules (e.g. "admin requires owner").

The manifest is the contract. If new flags or commands ship, update
the manifest and the wizard re-renders without code changes.

---

## 9. Out of scope (for v1)

- Live execution of commands from the browser.
- Auth / login on the landing.
- Streaming install logs back to the UI.
- Multi-machine / fleet installs.
- Generating CI scripts (GitHub Actions YAML, etc.) — could be a v2.

---

*Spec authored against codebase state on branch
`claude/custom-install-wizard-vPCkd`. Refresh by re-running the
inventory pass over `cli/internal/cmd/`, `vault/cmd/korva-vault/`,
`install.sh`, `install.ps1` and `internal/config/schema.go`.*
