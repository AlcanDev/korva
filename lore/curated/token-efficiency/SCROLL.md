---
id: token-efficiency
version: 1.0.0
team: all
stack: Claude Code, Cursor, Copilot, any AI agent
last_updated: 2026-04-30
---

# Scroll: Token-Efficient AI Sessions

## Triggers — load when:
- Files: `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`
- Keywords: token, context window, cache, /cost, session, sycophantic, verbose, output length
- Tasks: starting any coding session, configuring AI behavior, reducing token usage, improving response quality

## Context
AI sessions that drain context windows produce worse code over time — the model has less room to reason. Long, padded responses waste tokens that should be used for thinking. The patterns here keep sessions sharp, outputs tight, and context budgets healthy. These principles apply whether you are writing the AI's instructions or configuring how your agent behaves.

---

## Rules

### 1. Think before acting

Read relevant files before writing code. Never write code and then check whether it was the right approach.

```
BAD workflow:
  write code → run → fail → rewrite → run → fail → rewrite

GOOD workflow:
  read file → understand context → write once → run → done
```

When modifying existing code: read the file first. When creating new code: scan related files for patterns and conventions first.

### 2. Prefer editing over rewriting

Edit specific lines. Never rewrite an entire file when a targeted edit achieves the same result.

```
BAD: Rewrite the entire 400-line service because one method needs fixing.
GOOD: Edit the 3 lines that need changing.
```

Use the `Edit` tool (or equivalent) instead of `Write` for existing files. This preserves context for the human reviewer and costs fewer tokens.

### 3. Do not re-read files you have already read

If a file was read in the current session and has not changed, do not read it again. Reference what was already loaded.

Exception: if you suspect the file changed (e.g., after a tool call that modifies it), re-read before acting.

### 4. Skip large files unless explicitly required

Files over 100KB should not be loaded into context unless the user explicitly requests it. Instead:
- Search for the specific section needed (Grep, line-targeted Read)
- Ask the user which part is relevant

Large files consume context that could be used for reasoning.

### 5. Concise output, thorough reasoning

Internal reasoning can be as long as needed. Output to the user should be as short as possible while remaining complete.

```
BAD: "Great question! I'll be happy to help you with that. Let me walk through 
     this step by step and explain everything in detail..."

GOOD: [tool call result, then the answer]
```

No openers. No closers. No affirmations. Answer the question.

### 6. Tool first, explain second

When a tool call gives the answer, show the result. Explain only if the user asks why or if the result is ambiguous.

```
BAD: "I'll now run the tests to check if everything is working correctly."
     [runs tests]
     "As you can see, the tests passed successfully!"

GOOD: [runs tests — output shows passing]
```

### 7. Monitor session health

When a session runs long or switches topics significantly, surface the cost:

```
Suggest /cost when:
- The session has made 20+ tool calls
- The task is complete and a new unrelated task begins
- Context seems congested (long file reads, large diffs)
```

Recommend starting a new session when switching to an unrelated task. Fresh context produces better code.

### 8. Test before declaring done

Never say "this should work" without verifying. Run the code, test the function, or check the output before marking a task complete.

```
BAD: "The fix looks correct. Let me know if you have any issues."
GOOD: [runs tests] → "Tests pass. Done."
```

### 9. Keep solutions simple and direct

The simplest solution that solves the problem is the right solution. Avoid abstractions that aren't needed yet, premature generalization, and over-engineering.

```
BAD: Build a plugin system to handle one case that exists today.
GOOD: Write the one case directly. Generalize when the second case arrives.
```

### 10. User instructions override everything

If the user gives an explicit instruction that contradicts a rule in this scroll, follow the user instruction. This scroll sets defaults — it does not override the human.

---

## Communication Rules

These apply to all prose output (not to code):

| Rule | Example |
|---|---|
| Short sentences. 8-10 words max. | "The test fails. The mock is misconfigured." |
| No filler or preamble | ~~"Certainly! I'd be happy to..."~~ |
| No closing pleasantries | ~~"Let me know if you need anything else!"~~ |
| Never use em-dashes | Use a period or semicolon instead |
| Avoid parenthetical clauses | Pull the thought into its own sentence |
| Code stays normal | Only English prose gets compressed |

---

## Anti-Patterns

### BAD: Padded opener

```
// ❌ — wastes ~40 tokens before saying anything
"Great question! I'll take a look at that for you. Let me think through 
this carefully and provide a comprehensive answer..."
```

### BAD: Re-reading without cause

```
// ❌ — already read this file 3 messages ago, nothing changed
[reads payment.service.ts again]
[reads payment.service.ts again]
```

### BAD: Full rewrite for a targeted fix

```
// ❌ — user asked to rename one method
[writes entire 300-line file from scratch with the renamed method]

// ✅ — targeted edit
[edits 3 lines: the method name + 2 call sites]
```

### BAD: Unverified "should work"

```
// ❌ — no test run
"The null check I added should fix the issue. Let me know if it still occurs."

// ✅ — tested
[runs unit test] → "Test passes. Fix confirmed."
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [Claude API Patterns](https://skills.sh/anthropics/skills/claude-api) | `npx skills add anthropics/skills --skill claude-api -a claude-code` |
