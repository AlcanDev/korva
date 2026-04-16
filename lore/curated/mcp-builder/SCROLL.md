---
id: mcp-builder
version: 1.0.0
team: all
stack: TypeScript, Python, MCP SDK, FastMCP, Model Context Protocol
---

# Scroll: Building MCP Servers

## Triggers — load when:
- Files: `mcp.json`, `*.mcp.ts`, `server.ts` (with MCP imports), `pyproject.toml` (with mcp dependency)
- Keywords: MCP, Model Context Protocol, mcp server, tools/list, tools/call, stdio, streamable HTTP, FastMCP, mcp-go
- Tasks: building an MCP server, adding tools to an AI agent, integrating an external API via MCP, testing an MCP server

## Context
MCP (Model Context Protocol) is the standard interface for giving AI agents structured access to external services. An MCP server exposes tools — typed, documented functions that agents call via JSON-RPC 2.0. The quality of an MCP server is measured by how well its tools enable agents to accomplish real-world tasks: discoverable names, actionable errors, focused responses, proper pagination. A poorly designed MCP server produces agents that loop, hallucinate tool names, or silently fail.

---

## Rules

### 1. Preferred stack

Default to **TypeScript** for new MCP servers. TypeScript has the highest-quality SDK, best compatibility across execution environments, and the strongest type-safety for tool schemas.

```
Language     SDK                   Transport
TypeScript   @modelcontextprotocol/sdk   streamable HTTP (remote) | stdio (local)
Python       mcp / FastMCP               stdio | streamable HTTP
Go           mcp-go                      stdio
```

Use **streamable HTTP + stateless JSON** for remote servers (easier to scale). Use **stdio** for local servers (simpler, no networking).

### 2. Development follows 4 phases

Never jump to implementation. Always complete Research → Design → Build → Test in order.

```
Phase 1: Research
  - Study the target API (auth, rate limits, key endpoints, data models)
  - Read the MCP spec: https://modelcontextprotocol.io/sitemap.xml
  - Load SDK docs before writing a single line of code

Phase 2: Design
  - List all endpoints to expose as tools
  - Name every tool using the naming rules (Rule 3)
  - Define input/output schemas for each tool
  - Identify shared utilities needed (auth client, error handler, pagination)

Phase 3: Implement
  - Build shared infrastructure first (API client, error handling)
  - Implement tools one by one
  - Add annotations to each tool (readOnly, destructive, idempotent)

Phase 4: Test + Evaluate
  - Build with `npm run build` (TypeScript) or verify syntax (Python)
  - Test interactively: `npx @modelcontextprotocol/inspector`
  - Write 10 evaluation questions (see Rule 7)
```

### 3. Tool naming — prefix + action-oriented

Tool names are the agent's primary discovery mechanism. They must be consistent and unambiguous.

```typescript
// ✅ GOOD — consistent prefix, clear action
github_create_issue
github_list_repos
github_get_pull_request
github_merge_pull_request

// ❌ BAD — no prefix, ambiguous
create_issue          // which service?
listRepos             // mixing naming conventions
getPR                 // abbreviation without context
```

**Pattern:** `{service}_{verb}_{noun}` — all lowercase, underscores only.

### 4. Input schemas: Zod (TypeScript) or Pydantic (Python)

Every tool input must be fully typed with constraints and descriptions. Schemas are how agents understand what to pass.

```typescript
// TypeScript with Zod
import { z } from 'zod';

server.registerTool('github_create_issue', {
  description: 'Create a new issue in a GitHub repository.',
  inputSchema: z.object({
    owner:  z.string().describe('Repository owner (user or org)'),
    repo:   z.string().describe('Repository name'),
    title:  z.string().min(1).max(256).describe('Issue title'),
    body:   z.string().optional().describe('Issue body in markdown'),
    labels: z.array(z.string()).optional().describe('Label names to attach'),
  }),
  annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: false },
}, async (input) => {
  // implementation
});
```

```python
# Python with FastMCP + Pydantic
from mcp.server.fastmcp import FastMCP
from pydantic import BaseModel, Field

mcp = FastMCP("github")

class CreateIssueInput(BaseModel):
    owner:  str = Field(description="Repository owner (user or org)")
    repo:   str = Field(description="Repository name")
    title:  str = Field(min_length=1, max_length=256, description="Issue title")
    body:   str | None = Field(None, description="Issue body in markdown")
    labels: list[str] | None = Field(None, description="Label names to attach")

@mcp.tool()
async def github_create_issue(input: CreateIssueInput) -> str:
    # implementation
    ...
```

### 5. Actionable error messages

Errors must guide the agent toward the fix. Vague errors cause agents to loop or hallucinate.

```typescript
// ❌ BAD — agent doesn't know what to do
throw new Error('Request failed');

// ✅ GOOD — agent can retry with the right input
throw new Error(
  `GitHub API returned 422: title is required. ` +
  `Provide a non-empty "title" parameter.`
);
```

Include: what failed, why it failed, what to try next.

### 6. Tool annotations

Annotate every tool so clients can make better decisions about caching and confirmation UI.

| Annotation | Meaning | Example |
|---|---|---|
| `readOnlyHint: true` | Tool only reads, never mutates | `github_list_repos` |
| `readOnlyHint: false` | Tool may write or mutate | `github_create_issue` |
| `destructiveHint: true` | Tool may delete or overwrite | `github_delete_branch` |
| `idempotentHint: true` | Calling twice = same result | `github_get_issue` |
| `openWorldHint: true` | Tool interacts with external world | any HTTP call |

### 7. Write 10 evaluation questions

Before shipping, verify your server works end-to-end by creating 10 realistic evaluation questions. Each question must:

- Require multiple tool calls to answer
- Be answerable with read-only operations only
- Have a single verifiable answer (string comparison)
- Be based on real data (not synthetic)
- Be independent of other questions

```xml
<!-- evaluations/github.xml -->
<evaluation>
  <qa_pair>
    <question>How many open issues does the "react" repository owned by "facebook" have?</question>
    <answer><!-- solve with your tools before shipping --></answer>
  </qa_pair>
  <!-- 9 more... -->
</evaluation>
```

Test with: `npx @modelcontextprotocol/inspector --config evaluations/github.xml`

---

## Anti-Patterns

### BAD: Generic tool names without prefix

```typescript
// ❌ — when an agent has 50 tools loaded, "create_issue" is ambiguous
create_issue({ title: "bug" })

// ✅ — unambiguous even in a multi-server context
github_create_issue({ owner: "acme", repo: "api", title: "bug" })
```

### BAD: Vague tool descriptions

```typescript
// ❌ — agent cannot infer when to use this
{ description: "Does stuff with issues" }

// ✅ — agent knows exactly when and how to use this
{ description: "Create a new GitHub issue. Returns the issue number and URL." }
```

### BAD: Undocumented input fields

```typescript
// ❌ — agent guesses what "ref" means
inputSchema: z.object({ ref: z.string() })

// ✅ — agent knows exactly what to pass
inputSchema: z.object({
  ref: z.string().describe('Branch name, tag, or full commit SHA (e.g. "main", "v1.2.0", "a3f4c2b")')
})
```

### BAD: Implementing before reading the SDK docs

```
// ❌ — writing MCP server code from memory
// Result: wrong transport setup, wrong tool registration pattern, wrong error format

// ✅ — Phase 1 always comes first
// Load https://raw.githubusercontent.com/modelcontextprotocol/typescript-sdk/main/README.md
// Then implement
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [MCP Builder](https://skills.sh/anthropics/skills/mcp-builder) | `npx skills add anthropics/skills --skill mcp-builder -a claude-code` |
