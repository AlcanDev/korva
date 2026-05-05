# Contributing to Korva

Thank you for your interest in contributing! Korva is MIT licensed and welcomes contributions of all kinds.

## Ways to contribute

- 📜 **Knowledge Scrolls** — the highest-impact contribution. Write a SCROLL.md for a framework, pattern, or workflow your team uses.
- 🐛 **Bug reports** — use GitHub Issues with the `bug` label
- 💡 **Feature requests** — use GitHub Issues with the `enhancement` label
- 🔧 **Code** — see the development guide below

---

## Development setup

### Prerequisites

- Go 1.22+
- Git
- (Optional) Node.js 20+ for Beacon dashboard

### Clone and build

```bash
git clone https://github.com/AlcanDev/korva.git
cd korva
go work sync
go build ./...
go test github.com/alcandev/korva/...
```

### Repository structure

```
korva/
├── internal/       # Shared Go packages (config, db, privacy, admin, profile)
├── vault/          # Vault server (SQLite + MCP + REST)
├── cli/            # korva CLI
├── sentinel/       # Pre-commit hooks + validator
├── lore/curated/   # Curated knowledge Scrolls ← great place to contribute
├── forge/          # SDD workflow phases
└── beacon/         # React 19 web dashboard
```

### Running tests

```bash
# All packages
go test github.com/alcandev/korva/...

# Specific module
cd vault && go test ./...

# With coverage
go test ./internal/... -cover
```

We target **>80% coverage** on all testable packages. Please include tests for any new code.

---

## Contributing a Knowledge Scroll

Scrolls are the most valuable contribution — they encode team knowledge that the AI assistant uses automatically.

1. Copy `lore/SCROLL_TEMPLATE.md` to `lore/curated/<your-scroll-name>/SCROLL.md`
2. Fill in the frontmatter and content
3. The scroll ID should be kebab-case: `nestjs-hexagonal`, `react-testing`, etc.
4. Open a PR — scrolls don't need code review, just a quick check for accuracy

---

## Pull Request guidelines

- Keep PRs focused — one feature or fix per PR
- Include tests for new functionality
- Update documentation if you change behavior
- The PR title should be clear and in English (description can be in any language)
- CI must pass before merging

### Commit message format

```
type: short description

Optional longer explanation.
```

Types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`

---

## Code style

- Standard Go formatting (`gofmt`)
- Table-driven tests
- No hardcoded paths — use `config.PlatformPaths()`
- Privacy filter on every write to SQLite
- Errors wrapped with context: `fmt.Errorf("loading config: %w", err)`

---

## Versioning strategy

Korva follows [Semantic Versioning](https://semver.org): `vMAJOR.MINOR.PATCH`.

### Patch (`v1.1.x` → `v1.1.1`)

Backwards-compatible fixes. Release frequently — no planning needed.

- Bug fixes that don't change any public interface
- Performance improvements
- Documentation-only changes
- Security patches

### Minor (`v1.x.0` → `v1.2.0`)

New capabilities that don't break existing users. Batch several together when possible.

- New MCP tools (vault_* additions)
- New CLI commands or flags
- New API endpoints
- New Beacon pages or major UI additions
- New Knowledge Scrolls bundled in the release
- New configuration keys (always optional, with sensible defaults)

### Major (`vX.0.0` — breaking changes)

Breaking changes that require users to update their setup. Avoid unless necessary.

- MCP tool renames or removed parameters (AI agents have these hardcoded)
- CLI flag renames or removed commands that are in common use
- REST API breaking changes (renamed/removed endpoints, response format changes)
- `korva.config.json` schema changes that aren't backwards-compatible
- SQLite schema migrations that can't run automatically
- Minimum Go version bumps that affect build compatibility

**When NOT to bump major:** adding fields to JSON responses, deprecating (but not removing)
features, changing default values of optional settings.

### Release checklist

1. Update `CHANGELOG.md` (generated automatically by release-please if you use conventional commits)
2. Tag: `git tag v1.2.0 && git push --tags`
3. CI runs the release pipeline automatically (GoReleaser + Homebrew tap update)

### Conventional commits → automatic version bumps

The release pipeline uses [release-please](https://github.com/googleapis/release-please).
Commit message prefixes control the version bump:

| Prefix | Bump | Example |
|---|---|---|
| `fix:` | patch | `fix(vault): handle nil session_id` |
| `feat:` | minor | `feat(cli): add korva config command` |
| `feat!:` or `BREAKING CHANGE:` | major | `feat!: rename vault_observe to vault_save` |
| `docs:`, `chore:`, `test:`, `refactor:` | none | (skipped in changelog) |

---

## Questions?

Open a GitHub Discussion or file an Issue. We're happy to help.

---

*Last updated: 2026-05-04*
