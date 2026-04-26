package rules

import (
	"fmt"
	"regexp"
	"strings"
)

// NAM001 — DTO classes must end with uppercase DTO.
type NAM001 struct{}

var reClassDecl = regexp.MustCompile(`\bclass\s+(\w+)`)
var reDTOSuffix = regexp.MustCompile(`DTO$`)
var reDtoLowerSuffix = regexp.MustCompile(`Dto$`)

func (r NAM001) ID() string { return "NAM-001" }
func (r NAM001) Applies(path string) bool {
	return strings.Contains(normalizePath(path), "/dto/") && isTS(path)
}
func (r NAM001) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		m := reClassDecl.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if reDtoLowerSuffix.MatchString(name) && !reDTOSuffix.MatchString(name) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("DTO class %q must use uppercase suffix `DTO`, not `Dto`", name),
			})
		}
	}
	return vs
}

// NAM002 — DI port tokens must be SCREAMING_SNAKE_CASE.
type NAM002 struct{}

var rePortExport = regexp.MustCompile(`export\s+const\s+(\w+)\s*=\s*['"][^'"]*Port['"]`)
var reScreamingSnake = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)

func (r NAM002) ID() string { return "NAM-002" }
func (r NAM002) Applies(path string) bool {
	return strings.Contains(normalizePath(path), "/domain/") && isTS(path)
}
func (r NAM002) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		m := rePortExport.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if !reScreamingSnake.MatchString(name) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("Port token %q must be SCREAMING_SNAKE_CASE, e.g. INSURANCE_PORT", name),
			})
		}
	}
	return vs
}

// NAM003 — Adapter files must follow naming pattern.
type NAM003 struct{}

func (r NAM003) ID() string { return "NAM-003" }
func (r NAM003) Applies(path string) bool {
	return strings.Contains(normalizePath(path), "/adapters/") && isTS(path) && !isSpec(path)
}
func (r NAM003) Check(filePath string, lines []string) []Violation {
	p := normalizePath(filePath)
	base := p[strings.LastIndex(p, "/")+1:]
	// Must match: *.adapter.*.ts or *.adapter.ts
	if !strings.Contains(base, ".adapter.") {
		return []Violation{{
			File:     filePath,
			Line:     1,
			Rule:     r.ID(),
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Adapter file %q should follow pattern `name.adapter[.variant].ts`", base),
		}}
	}
	return nil
}

// SEC001 — No hardcoded secrets or API keys.
type SEC001 struct{}

var reHardcodedSecret = regexp.MustCompile(
	`(?i)(password|api_key|apikey|secret|token|auth)\s*[=:]\s*['"][^'"]{6,}['"]`,
)
var reProcessEnv = regexp.MustCompile(`process\.env\.|configService\.get`)

func (r SEC001) ID() string { return "SEC-001" }
func (r SEC001) Applies(path string) bool {
	return isInSrc(path) && isTS(path) && !isSpec(path)
}
func (r SEC001) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		if reHardcodedSecret.MatchString(line) && !reProcessEnv.MatchString(line) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  "Potential hardcoded secret — use process.env.* or configService.get()",
			})
		}
	}
	return vs
}

// TEST001 — Spec files must be co-located with source.
type TEST001 struct{}

func (r TEST001) ID() string { return "TEST-001" }
func (r TEST001) Applies(path string) bool {
	return isSpec(path) && isTS(path)
}
func (r TEST001) Check(filePath string, lines []string) []Violation {
	p := normalizePath(filePath)
	if strings.Contains(p, "/__tests__/") || strings.Contains(p, "/test/") {
		return []Violation{{
			File:     filePath,
			Line:     1,
			Rule:     r.ID(),
			Severity: SeverityWarning,
			Message:  "Spec files should be co-located with source, not in a separate __tests__/ folder",
		}}
	}
	return nil
}

// AllRules returns all built-in rules.
func AllRules() []Rule {
	return []Rule{
		HEX001{},
		HEX002{},
		HEX003{},
		HEX004{},
		HEX005{},
		NAM001{},
		NAM002{},
		NAM003{},
		SEC001{},
		TEST001{},
	}
}

// RuleProfile controls which rules are active.
//
//   - minimal  — security-critical only (SEC001). Recommended for bootstrapping teams.
//   - standard — security + critical architecture rules (SEC001, HEX001-003). Default.
//   - strict   — all rules. Enforces naming, testing, and full architecture compliance.
type RuleProfile string

const (
	ProfileMinimal  RuleProfile = "minimal"
	ProfileStandard RuleProfile = "standard"
	ProfileStrict   RuleProfile = "strict"
)

// RulesForProfile returns the subset of AllRules active under the given profile.
// Unknown profile names fall back to ProfileStandard.
func RulesForProfile(p RuleProfile) []Rule {
	switch p {
	case ProfileMinimal:
		// Only hard security violations — no architecture noise for new teams.
		return []Rule{SEC001{}}
	case ProfileStrict:
		// Every rule: security, architecture, naming, and testing.
		return AllRules()
	default: // ProfileStandard
		// Security + critical hexagonal rules. Balanced for established teams.
		return []Rule{
			HEX001{},
			HEX002{},
			HEX003{},
			SEC001{},
		}
	}
}
