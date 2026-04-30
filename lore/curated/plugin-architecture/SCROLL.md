---
id: plugin-architecture
version: 1.0.0
team: backend
stack: Go, plugin systems, extensibility, semver
last_updated: 2026-04-30
---

# Scroll: Plugin Architecture — Designing Extensible Systems

## Triggers — load when:
- Files: `**/plugin*.go`, `**/extensions/**`, `**/registry*.go`, `manifest.json`
- Keywords: plugin, extension, hook, registry, manifest, dynamic load, sandboxed, capabilities
- Tasks: designing a plugin API, adding a third-party extension point, versioning a public interface, sandboxing untrusted code

## Context
A plugin system is a public contract you can never break without breaking every plugin author. Get the contract right on day one — narrow surface, explicit versioning, declared capabilities — and the ecosystem grows. Get it wrong and either you ossify (refusing to evolve to protect plugins) or fragment (every minor version invalidates third-party work). The patterns below let the host evolve without breaking the contract.

---

## Rules

### 1. Declarative manifest, code loaded later

Plugins describe themselves in a static manifest before any code runs:

```json
{
  "id": "my-plugin",
  "version": "1.2.0",
  "host": ">=1.0.0 <2.0.0",
  "entry": "./dist/index.js",
  "capabilities": ["read:vault", "write:vault"],
  "triggers": {
    "files": ["*.ts"],
    "keywords": ["typescript"]
  }
}
```

The host reads every manifest first, validates compatibility (`host` semver range), then loads code only for plugins that pass. This means a malformed plugin can never crash the host — its manifest is rejected before its code executes.

### 2. Capabilities — least privilege by default

Every plugin declares the resources it needs. The host grants nothing else.

```go
type Capability string

const (
    CapReadVault    Capability = "read:vault"
    CapWriteVault   Capability = "write:vault"
    CapNetworkOut   Capability = "network:out"
    CapFilesystem   Capability = "filesystem:read"
)

func (h *Host) Grant(plugin *Plugin, cap Capability) error {
    if !slices.Contains(plugin.Manifest.Capabilities, cap) {
        return fmt.Errorf("plugin %q did not declare capability %q",
            plugin.Manifest.ID, cap)
    }
    return nil
}
```

A plugin that declares only `read:vault` can never call a write API even if it tries. The check happens in the host, not in the plugin's own code (where it could be bypassed).

### 3. Versioned API contract — host range in plugin manifest

```json
{
  "host": ">=1.2.0 <2.0.0"
}
```

The plugin says: "I work with host versions 1.2.x through 1.x.x but not 2.x". The host refuses to load incompatible plugins with a clear error:

```
Plugin "my-plugin" requires host >=1.2.0 <2.0.0; running 2.1.0. Skipping.
```

This is the difference between a clean degradation and a 3am crash.

### 4. Hooks, not subclasses

Expose plugin extension points as named hooks the host calls at well-defined moments:

```go
type Hook string

const (
    HookBeforeSave Hook = "before:save"
    HookAfterSave  Hook = "after:save"
    HookOnError    Hook = "on:error"
)

func (h *Host) Emit(hook Hook, ctx context.Context, payload any) {
    for _, p := range h.plugins[hook] {
        go p.Handle(ctx, payload) // sandboxed, timeout-bounded
    }
}
```

Avoid letting plugins inherit from host classes or override host methods — that ties their lifetime and signatures to your internal refactors. Hooks are stable; method names are not.

### 5. Plugin update flow — declarative install, separate update

Two distinct operations:

| Op | Behaviour |
|----|-----------|
| **Install** | Add the plugin to the manifest, pin a version, fetch the artifact |
| **Update** | Bump the pinned version to the latest compatible release, refetch |

```bash
# Install — declarative, idempotent, version-pinned
your-cli plugin install foo@1.2.0

# Update — moves the pin within the host's compat range
your-cli plugin update foo
your-cli plugin update --all
```

The install command MUST fail if a plugin is already at the requested version (idempotency); the update command MUST be a no-op if no newer compatible version exists.

### 6. Dependency conflicts between plugins

Plugin A wants library X@1, Plugin B wants library X@2. Two strategies:

**Strategy A — bundled deps (recommended for JS/Node)**
Each plugin ships its own dependencies. Slight disk/memory cost; zero conflict surface.

**Strategy B — host provides deps as a stable surface**
Plugins import a versioned facade exposed by the host:
```go
import "your-host/sdk/v1"  // the host re-exports its own types
```
The host bumps `v2` only on breaking changes; `v1` and `v2` coexist for one minor cycle.

Never resolve at install time by picking the higher version — that silently breaks Plugin A.

### 7. Logging and error attribution

Every log line and error from a plugin gets tagged with the plugin ID before reaching the host's logger:

```go
func (p *Plugin) Logger() *slog.Logger {
    return p.host.Logger().With("plugin", p.Manifest.ID, "version", p.Manifest.Version)
}
```

When plugin foo emits an error, the operator sees:
```
ERROR  plugin=foo version=1.2.0  panic in before:save hook: nil pointer
```
not just an unattributed stack trace from somewhere in the host process.

### 8. Timeout every plugin call

A plugin that hangs blocks the hook. Wrap every call:

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()

if err := plugin.Handle(ctx, payload); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        h.disable(plugin, "timeout exceeded")
    }
    return err
}
```

A misbehaving plugin gets disabled, the host stays responsive. The operator sees in logs which plugin was disabled and why.

---

## Anti-Patterns

### BAD: implicit version compatibility
```json
{ "id": "foo", "version": "1.2.0" }
```
No `host` range. The plugin loads against host 5.0 and crashes mid-call. The user has no warning that the plugin was untested with this host.

### BAD: plugins share global state with the host
```go
package globals
var Cache = make(map[string]any)
```
A plugin writes a key, another plugin overwrites it, the host reads stale data. Make every shared resource go through an explicit host API with the plugin ID as part of the key.

### BAD: opaque "do anything" capability
```json
{ "capabilities": ["admin"] }
```
The user installing the plugin has no way to evaluate the risk. List specific resources (`read:vault`, `write:network:api.example.com`).

### BAD: host imports plugin types
```go
import "third-party/foo-plugin/types"
func saveObservation(o foo.Observation) { ... }
```
The host now depends on every plugin's release schedule. Inverting: plugins import host SDK types; host is plugin-agnostic.

### BAD: silent fallback when a plugin fails to load
```go
plugin, err := load(manifest)
if err != nil { continue }  // log nothing, move on
```
Plugin install reports success, plugin never ran, user wastes hours debugging. Always log the failure with plugin ID and the specific cause.
