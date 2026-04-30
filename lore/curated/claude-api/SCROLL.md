---
id: claude-api
version: 1.0.0
team: all
stack: Anthropic SDK, TypeScript, Python, Go, Claude API
last_updated: 2026-04-30
---

# Scroll: Building with the Claude API

## Triggers — load when:
- Files: files importing `anthropic`, `@anthropic-ai/sdk`, `from anthropic`, `mcp-go`
- Keywords: Claude API, Anthropic SDK, prompt caching, thinking, streaming, tool use, claude-opus, claude-sonnet, claude-haiku, Messages API, batch processing
- Tasks: building a Claude-powered feature, adding AI to an app, configuring Claude models, migrating Claude model versions, debugging API calls

## Context
The Anthropic SDK is the correct interface for all Claude API work — never raw HTTP in a TypeScript or Python project, never an OpenAI shim. The SDK handles retries, streaming, type safety, and prompt caching. Getting the model defaults right (version, thinking mode, streaming) prevents the most common performance and cost issues before they occur.

---

## Rules

### 1. Default model: `claude-opus-4-7`

Always use `claude-opus-4-7` unless the user explicitly names a different model. Never guess or use a deprecated model string.

```typescript
// ✅ GOOD — correct default
const response = await client.messages.create({
  model: 'claude-opus-4-7',
  max_tokens: 8096,
  messages: [{ role: 'user', content: prompt }],
});

// ❌ BAD — outdated or generic
model: 'claude-3-opus-20240229'   // retired model
model: 'claude-latest'            // not a real model string
```

**Current model strings:**
| Model | ID | Context | Use for |
|---|---|---|---|
| Claude Opus 4.7 | `claude-opus-4-7` | 1M | Default — complex reasoning |
| Claude Sonnet 4.6 | `claude-sonnet-4-6` | 1M | Balanced — cost + quality |
| Claude Haiku 4.5 | `claude-haiku-4-5` | 200K | Speed — simple tasks |

### 2. Default thinking: adaptive

Use `thinking: { type: "adaptive" }` for anything that requires reasoning. Adaptive thinking lets Claude decide how much compute to apply — it won't think when unnecessary.

```typescript
// ✅ GOOD — adaptive thinking for reasoning tasks
const response = await client.messages.create({
  model: 'claude-opus-4-7',
  max_tokens: 16000,
  thinking: { type: 'adaptive' },
  messages: [{ role: 'user', content: prompt }],
});

// For tasks that definitely don't need thinking (simple extraction, classification):
thinking: { type: 'disabled' }
```

### 3. Default to streaming for long I/O

Use streaming when the request may involve long input, long output, or high `max_tokens`. Streaming prevents timeout errors and improves perceived latency.

```typescript
// TypeScript — streaming with get_final_message()
const stream = await client.messages.stream({
  model: 'claude-opus-4-7',
  max_tokens: 8096,
  messages: [{ role: 'user', content: prompt }],
});

// Don't need individual events? Use the helper:
const message = await stream.getFinalMessage();
console.log(message.content);
```

```python
# Python — streaming with get_final_message()
with client.messages.stream(
    model='claude-opus-4-7',
    max_tokens=8096,
    messages=[{'role': 'user', 'content': prompt}],
) as stream:
    message = stream.get_final_message()
```

When you need to handle individual events (progress UI, partial streaming to the client):
```typescript
for await (const event of stream) {
  if (event.type === 'content_block_delta') {
    process.stdout.write(event.delta.text);
  }
}
```

### 4. Never mix SDK with raw HTTP

In a TypeScript or Python project, always use the SDK. Never fall back to `fetch`, `requests`, or `axios` just because it seems lighter.

```typescript
// ❌ BAD — raw HTTP in a TypeScript project
const response = await fetch('https://api.anthropic.com/v1/messages', {
  method: 'POST',
  headers: { 'x-api-key': apiKey, 'anthropic-version': '2023-06-01' },
  body: JSON.stringify({ model: 'claude-opus-4-7', ... }),
});

// ✅ GOOD — SDK handles auth, retries, types
import Anthropic from '@anthropic-ai/sdk';
const client = new Anthropic();  // reads ANTHROPIC_API_KEY from env
const response = await client.messages.create({ ... });
```

### 5. Choose the right surface

```
Single task (classification, extraction, Q&A)
  → client.messages.create()

Multi-step workflow (agent loop with tools, you control the loop)
  → client.messages.create() + tools array + manual loop

Large batch (>10 independent requests, cost optimization)
  → client.beta.messages.batches.create()

Long-running agent (Anthropic runs the loop and hosts tool execution)
  → Managed Agents (client.beta.agents.create())
```

### 6. Prompt caching

Add `cache_control: { type: "ephemeral" }` to large, stable content (system prompt, large documents, few-shot examples). Cache reduces cost by up to 90% on repeated calls.

```typescript
const response = await client.messages.create({
  model: 'claude-opus-4-7',
  max_tokens: 8096,
  system: [
    {
      type: 'text',
      text: longSystemPrompt,  // 1000+ tokens that don't change per request
      cache_control: { type: 'ephemeral' },
    }
  ],
  messages: [{ role: 'user', content: userQuery }],
});
```

Cache hits show in `usage.cache_read_input_tokens`. Monitor with `/cost` when running long sessions.

### 7. Tool use

Define tools with JSON schema. Let Claude decide when to call them. Implement the loop.

```typescript
const tools: Anthropic.Tool[] = [
  {
    name: 'search_codebase',
    description: 'Search for files and code in the repository. Returns file paths and matching lines.',
    input_schema: {
      type: 'object',
      properties: {
        query:    { type: 'string', description: 'Search term or regex pattern' },
        file_glob: { type: 'string', description: 'Optional file glob filter, e.g. "**/*.ts"' },
      },
      required: ['query'],
    },
  },
];

async function runWithTools(prompt: string) {
  const messages: Anthropic.MessageParam[] = [{ role: 'user', content: prompt }];

  while (true) {
    const response = await client.messages.create({
      model: 'claude-opus-4-7',
      max_tokens: 8096,
      tools,
      messages,
    });

    if (response.stop_reason === 'end_turn') break;

    if (response.stop_reason === 'tool_use') {
      const toolResults = await executeTools(response.content);
      messages.push({ role: 'assistant', content: response.content });
      messages.push({ role: 'user',      content: toolResults });
    }
  }
}
```

### 8. Environment variables — never hardcode API keys

```typescript
// ❌ BAD — hardcoded key in source
const client = new Anthropic({ apiKey: 'sk-ant-api03-...' });

// ✅ GOOD — reads from environment automatically
const client = new Anthropic();   // uses process.env.ANTHROPIC_API_KEY
```

Set in shell: `export ANTHROPIC_API_KEY=sk-ant-...`
Or in `.env` (never committed): `ANTHROPIC_API_KEY=sk-ant-...`

---

## Anti-Patterns

### BAD: Using a deprecated model string

```typescript
// ❌ — retired model, will fail with API error
model: 'claude-3-opus-20240229'
model: 'claude-3-5-sonnet-20241022'

// ✅ — current
model: 'claude-opus-4-7'
model: 'claude-sonnet-4-6'
```

### BAD: Polling instead of streaming

```typescript
// ❌ — may timeout on long responses, no progress feedback
const response = await client.messages.create({ max_tokens: 16000, ... });

// ✅ — streaming for anything with high max_tokens
const stream = await client.messages.stream({ max_tokens: 16000, ... });
const message = await stream.getFinalMessage();
```

### BAD: Ignoring cache_read_input_tokens

```typescript
// ❌ — sending 5000-token system prompt on every call, paying full price
const response = await client.messages.create({
  system: hugeSystemPrompt,   // no cache_control
  ...
});

// ✅ — cache the stable content
system: [{ type: 'text', text: hugeSystemPrompt, cache_control: { type: 'ephemeral' } }]
// cache_read_input_tokens shows up in usage — validate it's working
```

### BAD: Mixing providers in the same file

```typescript
// ❌ — OpenAI and Claude in the same module creates confusion and bugs
import OpenAI from 'openai';
import Anthropic from '@anthropic-ai/sdk';
const gptClient = new OpenAI();
const claudeClient = new Anthropic();
// Which one does this function use? Which model? Which schema?

// ✅ — one provider per module/service
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [Claude API](https://skills.sh/anthropics/skills/claude-api) | `npx skills add anthropics/skills --skill claude-api -a claude-code` |
| [MCP Builder](https://skills.sh/anthropics/skills/mcp-builder) | `npx skills add anthropics/skills --skill mcp-builder -a claude-code` |
