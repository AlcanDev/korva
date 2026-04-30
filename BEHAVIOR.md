# BEHAVIOR.md — Korva behavioral guidelines for AI coding agents

Behavioral discipline that applies to **every** task in this repository,
regardless of which AI tool you are using (Claude Code, Cursor, Windsurf,
Copilot, Codex, OpenCode, Gemini CLI, VS Code with MCP).

These principles complement (not replace) the project-context guidance in
[CLAUDE.md](CLAUDE.md) — that file describes *what Korva is*; this file
describes *how to work in it without making the typical LLM mistakes*.

> **Tradeoff:** these guidelines bias toward caution over speed. For trivial
> edits (typo, one-liner) use judgment.

Inspired by Andrej Karpathy's observations about LLM coding failure modes,
adapted to Korva's Go monorepo + MCP-driven workflow.

---

## 1. Think before coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before writing code:

- **State your assumptions explicitly.** If uncertain, ask — never silently pick.
- **If multiple interpretations exist, present them.** Don't choose for the user.
- **If a simpler approach exists, say so.** Push back when warranted.
- **If something is unclear, stop. Name what is confusing. Ask.**

Korva-specific applications:

- Touching the privacy filter? State whether your change keeps the default-deny
  invariant or relaxes it.
- Adding a new MCP tool? State which `Profile` (`agent` / `readonly` / `admin`)
  it belongs in and why.
- Changing an SQL migration? State whether it is idempotent on existing installs.
- Editing the licensing flow? State which tier (Community / Teams) the change
  affects.

When the request is ambiguous, run `vault_search` (or `vault_hint` for cheap
discovery) before proposing — there may be a prior decision you should align with.

## 2. Simplicity first

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" the user did not request.
- No error handling for impossible scenarios.
- If you write 200 lines and 50 would do, rewrite.

Ask yourself: **"Would a senior Go engineer say this is overcomplicated?"**
If yes, simplify before sending.

Korva-specific applications:

- Default to standard library + `modernc.org/sqlite` + `oklog/ulid`. New deps
  need a real reason — `cd <module> && go get` adds to one module's go.mod, not
  the workspace.
- Prefer table-driven tests with in-memory SQLite over elaborate mocking.
- Korva's MCP profiles already exist (`agent` / `readonly` / `admin`); don't
  invent a fourth without evidence.

## 3. Surgical changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- **Don't "improve" adjacent code, comments, or formatting.**
- **Don't refactor things that aren't broken.**
- **Match existing style**, even if you would write it differently.
- If you notice unrelated dead code, **mention it — don't delete it.**

When your changes create orphans:

- Remove imports / variables / functions **that your changes** made unused.
- Don't remove pre-existing dead code unless asked.

**The test:** every changed line should trace directly back to the user's request.

Korva-specific applications:

- The `gofmt -w` and misspell sweeps live in their own dedicated commits. Don't
  combine them with feature work.
- A migration always appends to the migrations slice; never modify an existing
  migration.
- The `internal/privacy/filter.go` rules and gitleaks allowlist are intentional
  — don't relax them as a side effect.

## 4. Goal-driven execution

**Define success criteria. Loop until verified.**

Transform vague tasks into verifiable goals:

| Vague request | Goal-driven framing |
|---|---|
| "Add validation" | Write tests for invalid inputs, then make them pass |
| "Fix the bug" | Write a test that reproduces it, then make it pass |
| "Refactor X" | Ensure tests pass before AND after the refactor |
| "Make it faster" | Add a benchmark, refactor, prove the benchmark improved |

For multi-step tasks, state a brief plan with verifiable checks:

```
1. <step> → verify: <how I'll know it worked>
2. <step> → verify: <how I'll know it worked>
3. <step> → verify: <how I'll know it worked>
```

Strong success criteria let the agent loop independently. Weak criteria
("make it work") force constant clarification.

Korva-specific verification commands:

```bash
# Build all binaries
go build github.com/alcandev/korva/...

# Run the full Go test suite (workspace-aware)
go test github.com/alcandev/korva/...

# Lint per workspace module (matches CI)
cd vault && golangci-lint run --config=$(pwd)/../.golangci.yml ./...

# Beacon (UI) — uses public npm registry (see beacon/.npmrc)
cd beacon && npm run build && npm test

# Sentinel self-check
go build -o /tmp/korva-sentinel ./sentinel/validator/cmd/korva-sentinel
find beacon/src \( -name '*.ts' -o -name '*.tsx' \) | /tmp/korva-sentinel --format json
```

---

## How to know these guidelines are working

- Diffs show **fewer unnecessary changes** (no orthogonal cleanups, no random
  formatting tweaks).
- Fewer rewrites caused by **overcomplicated initial implementations**.
- Clarifying questions arrive **before** implementation, not after a wrong
  assumption was already coded.
- Tests are written **first** for non-trivial behavior changes.

If you finish a task and your diff includes lines that don't trace to the
user's request, the next iteration must remove them.
