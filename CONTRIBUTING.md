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

## Questions?

Open a GitHub Discussion or file an Issue. We're happy to help.
