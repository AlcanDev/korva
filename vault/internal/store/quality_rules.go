package store

// quality_rules.go defines the built-in quality criteria for each SDD phase and language.
//
// Design principles:
//   - Rules are version-controlled in Go code, not in the DB (DB only stores results).
//   - Custom team rules are added via skills/scrolls in Beacon, not here.
//   - Every phase has a description + criteria; gated phases (apply, verify) have
//     required=true criteria that must pass for gate_passed=true.
//   - The "e2e" category is mandatory in the verify phase.
//
// The AI agent reads these via vault_qa_checklist, performs the review, then
// calls vault_qa_checkpoint to record findings. Korva never runs code itself.

// GetQualityChecklist returns the quality criteria for a given SDD phase and language.
// Language "" returns general (language-agnostic) criteria.
// Language-specific criteria are merged with general ones.
func GetQualityChecklist(phase, language string) QualityChecklist {
	general := generalCriteria(phase)
	langSpecific := languageCriteria(phase, language)

	all := append(general.Criteria, langSpecific...)
	general.Criteria = all
	general.Language = language
	return general
}

// generalCriteria returns language-agnostic quality criteria per SDD phase.
func generalCriteria(phase string) QualityChecklist {
	switch phase {
	case "explore":
		return QualityChecklist{
			Phase:       "explore",
			Description: "Understand the current state: map existing coverage, identify gaps, list what is/isn't tested.",
			Criteria: []QualityCriterion{
				{ID: "EXP-001", Category: "testing", Severity: "info", Required: false,
					Rule:     "Identify existing test files and their coverage scope",
					Guidance: "List all *_test.go / *.spec.ts files relevant to the area being explored."},
				{ID: "EXP-002", Category: "testing", Severity: "warning", Required: false,
					Rule:     "Document untested code paths",
					Guidance: "Note functions, branches, or integrations that have zero test coverage."},
				{ID: "EXP-003", Category: "patterns", Severity: "info", Required: false,
					Rule:     "Note existing patterns and conventions in the codebase",
					Guidance: "Identify naming, error handling, and structural patterns used in nearby code."},
			},
		}

	case "propose":
		return QualityChecklist{
			Phase:       "propose",
			Description: "Each proposed solution must specify how it will be tested and must not introduce untestable dependencies.",
			Criteria: []QualityCriterion{
				{ID: "PRO-001", Category: "testing", Severity: "warning", Required: true,
					Rule:     "Each proposal must state its testing strategy",
					Guidance: "For each proposed solution, specify: unit tests, integration tests, and if a new user flow is added, an E2E test."},
				{ID: "PRO-002", Category: "patterns", Severity: "warning", Required: false,
					Rule:     "Proposals must follow existing architectural patterns",
					Guidance: "Check OpenSpec conventions. New patterns need explicit justification in vault_save."},
				{ID: "PRO-003", Category: "testing", Severity: "info", Required: false,
					Rule:     "Identify which existing tests will be affected",
					Guidance: "List tests that may need to change. Unexpected test changes are a smell."},
			},
		}

	case "spec":
		return QualityChecklist{
			Phase:       "spec",
			Description: "Every requirement must have concrete, automatable acceptance criteria.",
			Criteria: []QualityCriterion{
				{ID: "SPEC-001", Category: "testing", Severity: "error", Required: true,
					Rule:     "Each requirement has at least one testable acceptance criterion",
					Guidance: "Format: 'Given X, when Y, then Z' — must be expressible as an automated assertion."},
				{ID: "SPEC-002", Category: "e2e", Severity: "warning", Required: false,
					Rule:     "User-facing flows have E2E test scenarios defined",
					Guidance: "For any change visible to end users, write at least one E2E scenario (e.g. Playwright test steps)."},
				{ID: "SPEC-003", Category: "docs", Severity: "info", Required: false,
					Rule:     "Edge cases and error scenarios are specified",
					Guidance: "Include empty states, error responses, and boundary conditions in spec."},
			},
		}

	case "design":
		return QualityChecklist{
			Phase:       "design",
			Description: "Architecture must follow conventions, maximize testability, and define isolation strategy.",
			Criteria: []QualityCriterion{
				{ID: "DES-001", Category: "patterns", Severity: "error", Required: true,
					Rule:     "Architecture follows OpenSpec conventions",
					Guidance: "Check the project's OpenSpec. Deviations must be justified and saved to vault."},
				{ID: "DES-002", Category: "testing", Severity: "error", Required: true,
					Rule:     "Dependencies are injectable / mockable",
					Guidance: "No hardcoded external dependencies. Use interfaces/adapters so tests can inject fakes."},
				{ID: "DES-003", Category: "testing", Severity: "warning", Required: false,
					Rule:     "Test isolation strategy defined (mocks, stubs, in-memory)",
					Guidance: "Specify what gets mocked and what gets real implementations in tests."},
				{ID: "DES-004", Category: "security", Severity: "warning", Required: false,
					Rule:     "Security boundaries identified",
					Guidance: "Note input validation points, auth checks, and data sanitization requirements."},
			},
		}

	case "tasks":
		return QualityChecklist{
			Phase:       "tasks",
			Description: "Each task must be atomic, independently testable, and explicitly reference its test type.",
			Criteria: []QualityCriterion{
				{ID: "TSK-001", Category: "testing", Severity: "error", Required: true,
					Rule:     "Every implementation task has a paired test task",
					Guidance: "For each 'implement X' task, add 'write unit test for X'. No implementation task without a test task."},
				{ID: "TSK-002", Category: "e2e", Severity: "warning", Required: false,
					Rule:     "New user-facing features include an E2E test task",
					Guidance: "If the task changes something a user can see or do, add an E2E test task (Playwright/Cypress)."},
				{ID: "TSK-003", Category: "testing", Severity: "warning", Required: false,
					Rule:     "Integration test tasks exist for API/DB interactions",
					Guidance: "HTTP handler tests, DB query tests, and external service tests must be separate tasks."},
				{ID: "TSK-004", Category: "docs", Severity: "info", Required: false,
					Rule:     "A documentation task is included if public API changes",
					Guidance: "Public function signatures, REST endpoints, or MCP tools that change need a docs update task."},
			},
		}

	case "apply":
		return QualityChecklist{
			Phase:     "apply",
			GatePhase: "verify",
			Description: "Code must pass all quality criteria before the apply→verify transition is allowed. " +
				"This is the primary code quality gate.",
			Criteria: []QualityCriterion{
				{ID: "APP-001", Category: "testing", Severity: "error", Required: true,
					Rule:     "Unit tests written for all new/modified functions",
					Guidance: "Every exported function and any complex internal function must have at least one test."},
				{ID: "APP-002", Category: "testing", Severity: "error", Required: true,
					Rule:     "Error paths are tested",
					Guidance: "Tests must cover both happy path and error/edge cases. No 'only testing the success path'."},
				{ID: "APP-003", Category: "style", Severity: "error", Required: true,
					Rule:     "No debug output in production code",
					Guidance: "Remove all fmt.Println, console.log, debugger, print() calls from non-test code."},
				{ID: "APP-004", Category: "patterns", Severity: "error", Required: true,
					Rule:     "Error handling: no silent error suppression",
					Guidance: "Every error must be handled: returned, logged, or explicitly ignored with //nolint:errcheck justification."},
				{ID: "APP-005", Category: "style", Severity: "warning", Required: false,
					Rule:     "Functions are focused: ≤ 30 lines, single responsibility",
					Guidance: "Functions over 30 lines are a smell. Extract helpers. Cyclomatic complexity ≤ 10."},
				{ID: "APP-006", Category: "testing", Severity: "warning", Required: false,
					Rule:     "Tests are table-driven or parameterized",
					Guidance: "Prefer table-driven tests (Go: t.Run, TS: test.each) over repetitive test functions."},
				{ID: "APP-007", Category: "security", Severity: "error", Required: true,
					Rule:     "No secrets/credentials in code or test fixtures",
					Guidance: "Use environment variables or test helpers. Never commit keys, passwords, or tokens."},
			},
		}

	case "verify":
		return QualityChecklist{
			Phase:       "verify",
			GatePhase:   "archive",
			E2ERequired: true,
			Description: "All tests must pass. E2E tests for user-facing flows are required. " +
				"Quality score ≥ 70 is needed for gate_passed=true.",
			Criteria: []QualityCriterion{
				{ID: "VER-001", Category: "testing", Severity: "error", Required: true,
					Rule:     "All unit tests pass with zero failures",
					Guidance: "Run the full test suite. Zero failures required. Skipped tests need justification."},
				{ID: "VER-002", Category: "testing", Severity: "error", Required: true,
					Rule:     "No regressions: existing tests still pass",
					Guidance: "Run tests on the full module, not just the changed files."},
				{ID: "VER-003", Category: "e2e", Severity: "error", Required: true,
					Rule:     "E2E tests exist and pass for user-facing critical paths",
					Guidance: "At minimum: happy path + one error scenario. Tools: Playwright (web), Go httptest (API), k6 (load)."},
				{ID: "VER-004", Category: "testing", Severity: "warning", Required: false,
					Rule:     "Test coverage ≥ 70% for new code",
					Guidance: "Use 'go test -cover' or 'vitest --coverage'. Report the coverage percentage."},
				{ID: "VER-005", Category: "security", Severity: "warning", Required: false,
					Rule:     "Static analysis passes: no linter errors",
					Guidance: "Run: go vet ./... + golangci-lint (Go) or eslint (TS). Zero errors required."},
				{ID: "VER-006", Category: "e2e", Severity: "warning", Required: false,
					Rule:     "Integration tests validate API contract",
					Guidance: "HTTP endpoints: test status codes, response shape, and error cases with real DB (in-memory ok)."},
				{ID: "VER-007", Category: "testing", Severity: "info", Required: false,
					Rule:     "Performance: no obvious N+1 queries or unnecessary full scans",
					Guidance: "Review DB queries in changed code. Use EXPLAIN QUERY PLAN for SQLite."},
			},
		}

	case "archive":
		return QualityChecklist{
			Phase:       "archive",
			Description: "Document what was built, how it was tested, and what patterns emerged.",
			Criteria: []QualityCriterion{
				{ID: "ARC-001", Category: "docs", Severity: "warning", Required: true,
					Rule:     "Save key decisions and patterns to vault",
					Guidance: "Use vault_save with type=decision for architectural choices, type=pattern for reusable patterns."},
				{ID: "ARC-002", Category: "docs", Severity: "info", Required: false,
					Rule:     "Test coverage report noted in vault",
					Guidance: "Save a learning observation with the coverage percentage and any coverage gaps."},
				{ID: "ARC-003", Category: "docs", Severity: "info", Required: false,
					Rule:     "Public APIs documented",
					Guidance: "REST endpoints, MCP tools, and exported functions have up-to-date doc comments."},
			},
		}

	case "onboard":
		return QualityChecklist{
			Phase:       "onboard",
			Description: "Capture team knowledge: update skills, scrolls, and share testing patterns.",
			Criteria: []QualityCriterion{
				{ID: "ONB-001", Category: "docs", Severity: "warning", Required: true,
					Rule:     "Team scrolls/skills updated if new patterns emerged",
					Guidance: "Update team skills in Beacon if a new architectural pattern or convention was introduced."},
				{ID: "ONB-002", Category: "testing", Severity: "info", Required: false,
					Rule:     "QA findings shared with team",
					Guidance: "If quality issues were found and fixed, save as learning observation so the team benefits."},
				{ID: "ONB-003", Category: "e2e", Severity: "info", Required: false,
					Rule:     "E2E test templates saved if new flow was added",
					Guidance: "Save the E2E test structure as a scroll for future reference."},
			},
		}

	default:
		return QualityChecklist{
			Phase:       phase,
			Description: "No specific quality criteria for this phase.",
			Criteria:    []QualityCriterion{},
		}
	}
}

// languageCriteria returns language-specific quality criteria for a given phase.
// Returns nil for unsupported languages.
func languageCriteria(phase, language string) []QualityCriterion {
	switch language {
	case "go":
		return goCriteria(phase)
	case "typescript", "ts":
		return typescriptCriteria(phase)
	case "react", "tsx":
		return reactCriteria(phase)
	default:
		return nil
	}
}

// goCriteria returns Go-specific quality criteria for the apply and verify phases.
func goCriteria(phase string) []QualityCriterion {
	switch phase {
	case "apply":
		return []QualityCriterion{
			{ID: "GO-APP-001", Category: "testing", Severity: "error", Required: true,
				Rule:     "Tests use table-driven pattern with t.Run()",
				Guidance: "Prefer: var tests = []struct{...}{{...}} / for _, tt := range tests { t.Run(tt.name, func...) }"},
			{ID: "GO-APP-002", Category: "patterns", Severity: "error", Required: true,
				Rule:     "All error returns are handled — no blank identifier for errors",
				Guidance: "Never: _, err := foo(); _ = err. Always handle or explicitly return the error."},
			{ID: "GO-APP-003", Category: "patterns", Severity: "warning", Required: false,
				Rule:     "Context propagation: functions that do I/O accept context.Context as first arg",
				Guidance: "DB queries, HTTP calls, and long operations must accept and pass ctx."},
			{ID: "GO-APP-004", Category: "style", Severity: "warning", Required: false,
				Rule:     "No init() functions; use constructors",
				Guidance: "Prefer explicit New*() constructors over init() for testability and clarity."},
			{ID: "GO-APP-005", Category: "testing", Severity: "warning", Required: false,
				Rule:     "In-memory DB for store tests (no real file on disk)",
				Guidance: "Use store.NewMemory() in tests. Never use a file-backed DB in unit tests."},
		}
	case "verify":
		return []QualityCriterion{
			{ID: "GO-VER-001", Category: "testing", Severity: "error", Required: true,
				Rule:     "go vet ./... passes with zero errors",
				Guidance: "Run: go vet ./... — must produce no output."},
			{ID: "GO-VER-002", Category: "testing", Severity: "error", Required: true,
				Rule:     "go test ./... passes with -race flag",
				Guidance: "Run: go test -race ./... — catches data races in concurrent code."},
			{ID: "GO-VER-003", Category: "e2e", Severity: "warning", Required: false,
				Rule:     "HTTP handler E2E tests use net/http/httptest.NewRecorder()",
				Guidance: "API tests must use httptest, not real HTTP servers. Full request → response cycle."},
		}
	}
	return nil
}

// typescriptCriteria returns TypeScript-specific quality criteria.
func typescriptCriteria(phase string) []QualityCriterion {
	switch phase {
	case "apply":
		return []QualityCriterion{
			{ID: "TS-APP-001", Category: "style", Severity: "error", Required: true,
				Rule:     "No `any` type without // korva-ignore comment (Sentinel HEX-005)",
				Guidance: "Use explicit types. If `any` is unavoidable, add // korva-ignore: <reason>."},
			{ID: "TS-APP-002", Category: "patterns", Severity: "error", Required: true,
				Rule:     "Hexagonal boundary respected: domain never imports infra (Sentinel HEX-001/HEX-002)",
				Guidance: "Domain layer: zero imports from /infrastructure/ or /application/. Run Sentinel."},
			{ID: "TS-APP-003", Category: "style", Severity: "error", Required: true,
				Rule:     "No console.log in src/ (Sentinel HEX-003)",
				Guidance: "Use injected Logger. console.* is allowed in tests only."},
			{ID: "TS-APP-004", Category: "patterns", Severity: "warning", Required: false,
				Rule:     "Adapters wired via DI, not instantiated with `new` (Sentinel HEX-004)",
				Guidance: "NestJS: use module DI. Never: new SomethingAdapter() outside module files."},
			{ID: "TS-APP-005", Category: "testing", Severity: "error", Required: true,
				Rule:     "Tests use Jest/Vitest with describe/it structure",
				Guidance: "Group tests with describe(). Each test name starts with 'should'."},
		}
	case "verify":
		return []QualityCriterion{
			{ID: "TS-VER-001", Category: "testing", Severity: "error", Required: true,
				Rule:     "ESLint passes with zero errors",
				Guidance: "Run: npx eslint src/ --ext .ts,.tsx — zero errors required."},
			{ID: "TS-VER-002", Category: "e2e", Severity: "warning", Required: false,
				Rule:     "Playwright E2E tests cover critical user flows",
				Guidance: "At minimum: login flow + main feature flow. Use page.goto() + expect(locator)."},
		}
	}
	return nil
}

// reactCriteria returns React-specific quality criteria.
func reactCriteria(phase string) []QualityCriterion {
	switch phase {
	case "apply":
		return []QualityCriterion{
			{ID: "REACT-APP-001", Category: "testing", Severity: "error", Required: true,
				Rule:     "Components tested with React Testing Library (not enzyme)",
				Guidance: "Use @testing-library/react. Test behavior, not implementation. Avoid testing CSS classes."},
			{ID: "REACT-APP-002", Category: "patterns", Severity: "warning", Required: false,
				Rule:     "No direct DOM manipulation — use React state/refs",
				Guidance: "Never use document.querySelector in component code. Use useRef if DOM access is needed."},
			{ID: "REACT-APP-003", Category: "style", Severity: "warning", Required: false,
				Rule:     "Components under 150 lines; extract sub-components or hooks",
				Guidance: "Large components are hard to test. Extract reusable logic into custom hooks."},
		}
	case "verify":
		return []QualityCriterion{
			{ID: "REACT-VER-001", Category: "e2e", Severity: "warning", Required: false,
				Rule:     "Critical UI flows covered by Playwright tests",
				Guidance: "Login, main CRUD operations, and error states must have E2E coverage."},
		}
	}
	return nil
}

// PhaseGates defines which phases require a quality checkpoint to advance.
// Key = current phase, Value = phase you're trying to enter.
var PhaseGates = map[string]string{
	"apply":  "verify",  // cannot enter verify without a passing apply checkpoint
	"verify": "archive", // cannot enter archive without a passing verify checkpoint
}

// IsGatedTransition returns true when moving from fromPhase to toPhase requires
// a quality checkpoint with gate_passed=true.
func IsGatedTransition(fromPhase, toPhase string) bool {
	required, ok := PhaseGates[fromPhase]
	return ok && required == toPhase
}
