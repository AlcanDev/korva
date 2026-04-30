---
id: release-engineering
version: 1.0.0
team: devops
stack: Conventional Commits, semver, release-please, GoReleaser
last_updated: 2026-04-30
---

# Scroll: Release Engineering — Commits, Semver, Automated Releases

## Triggers — load when:
- Files: `release-please-config.json`, `.github/workflows/release*.yml`, `CHANGELOG.md`, `.goreleaser.yaml`
- Keywords: release, semver, conventional commit, changelog, tag, version bump, breaking change
- Tasks: cutting a release, bumping a version, writing a commit message, configuring CI release pipeline

## Context
Manual releases drift — tags get skipped, changelogs get out of sync, and the version baked into the binary stops matching the git tag. Conventional Commits + automated release tooling close that gap: every commit message becomes the source of truth for both the changelog entry and the version bump. The only manual decision left is "is this PR ready to ship?" — everything else is mechanical.

---

## Rules

### 1. Conventional Commits — type(scope): subject

```text
feat(vault): add MCP write queue
fix(cli): resolve panic on missing config
fix!(api): rename /context to /session  ← breaking change
perf(store): batch INSERTs in single tx
docs: update install instructions
chore(deps): bump go to 1.26.x
```

Required types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `chore`, `ci`, `build`, `style`.
Add `!` after the type/scope to mark a breaking change. The body should still describe what broke and how to migrate.

### 2. Semver mapping — what bumps what

| Commit type | Version bump |
|-------------|--------------|
| `feat:` | minor (0.1.0 → 0.2.0) |
| `fix:`, `perf:`, `refactor:` | patch (0.1.0 → 0.1.1) |
| `feat!:` or `BREAKING CHANGE:` in body | major (0.1.0 → 1.0.0) |
| `docs:`, `chore:`, `test:`, `ci:` | no bump |

Pre-1.0: `feat!:` bumps minor (0.1.0 → 0.2.0), not major. Configure with `bump-minor-pre-major: true`.

### 3. release-please configuration

```json
{
  "release-type": "go",
  "packages": {
    ".": {
      "release-type": "go",
      "changelog-sections": [
        { "type": "feat", "section": "🚀 Features" },
        { "type": "fix",  "section": "🐛 Bug Fixes" },
        { "type": "perf", "section": "⚡ Performance" },
        { "type": "sec",  "section": "🔒 Security" },
        { "type": "docs", "section": "📚 Documentation", "hidden": true }
      ],
      "bump-minor-pre-major": true,
      "include-component-in-tag": false
    }
  },
  "include-v-in-tag": true,
  "separate-pull-requests": false
}
```

The `hidden: true` flag keeps `docs/chore/test` out of the public changelog while still tracking them.

### 4. The release flow

```
feature PR → CI green → squash-merge to main
    ↓
release-please workflow runs
    ↓
release-please opens "chore: release X.Y.Z" PR (accumulates commits)
    ↓
review the changelog → squash-merge the release PR
    ↓
release-please tags vX.Y.Z + creates GitHub Release
    ↓
release.yml workflow triggers on tag push
    ↓
GoReleaser builds artifacts → uploads to release → updates Homebrew tap
```

Never tag manually unless recovering from a failure. Every tag must have a matching release-please commit.

### 5. CI gate before release

The release workflow MUST wait for CI on the same SHA before publishing artifacts:

```yaml
jobs:
  wait-ci:
    runs-on: ubuntu-latest
    steps:
      - uses: lewagon/wait-on-check-action@v1.3.4
        with:
          ref: ${{ github.sha }}
          check-name: "Go build + test (ubuntu-latest, 1.26.x)"
          allowed-conclusions: success,skipped

  release:
    needs: wait-ci
    runs-on: ubuntu-latest
    steps:
      - uses: goreleaser/goreleaser-action@v6
```

Without this, a flaky test that lands on main gets shipped.

### 6. Embed version into the binary

```go
// internal/version/version.go
package version

var (
    Version = "dev"     // overridden by ldflags at build time
    Commit  = "none"
    Date    = "unknown"
)

func String() string {
    return fmt.Sprintf("%s (commit %s, %s)", Version, Commit, Date)
}
```

```yaml
# .goreleaser.yaml
ldflags:
  - -s -w
  - -X github.com/your-org/your-app/internal/version.Version={{.Version}}
  - -X github.com/your-org/your-app/internal/version.Commit={{.Commit}}
  - -X github.com/your-org/your-app/internal/version.Date={{.Date}}
```

Now `your-cli --version` reports the exact tag and SHA, matching the GitHub release.

### 7. Breaking changes — communicate explicitly

Every breaking change needs THREE things:
1. The `!` marker in the commit type
2. A `BREAKING CHANGE:` paragraph in the commit body explaining what to do
3. A `## Migration` section in the release PR description

```text
feat!(api): rename /context to /session

BREAKING CHANGE: clients calling GET /context must update to GET /session.
The response shape is unchanged. Old endpoint returns 410 Gone for two minor
versions before removal.
```

Hiding a breaking change in a "fix:" commit poisons every downstream consumer.

---

## Anti-Patterns

### BAD: free-form commit messages
```text
fixes
update stuff
WIP
asdf
```
release-please cannot generate a changelog from these. The release PR ends up empty or wrong.

### BAD: `chore: release 1.2.3` written by hand
You can do it once and get away with it. Do it twice and you'll forget to bump the version in `package.json` or the embedded constant. Let the tooling do it.

### BAD: tagging from a feature branch
```bash
git checkout feat/new-thing
git tag v1.0.0
git push --tags
```
The tag now points at a commit that was never on main. CI may or may not have run on it. Always tag from main, always after the release PR is merged.

### BAD: skipping the CI gate
```yaml
release:
  runs-on: ubuntu-latest
  # no needs: wait-ci → ships unconditionally
```
A red CI on the tagged SHA still ships a green release. Add the `wait-ci` dependency.

### BAD: putting `BREAKING CHANGE` in a `chore:` commit
Conventional Commits doesn't recognise it; release-please ignores it; the next minor goes out silently breaking everyone.
