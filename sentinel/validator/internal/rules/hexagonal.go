package rules

import (
	"fmt"
	"regexp"
	"strings"
)

// reImport matches TypeScript/JavaScript static import statements.
var reImport = regexp.MustCompile(`^\s*import\s+.*\s+from\s+['"]([^'"]+)['"]`)

// reDynImport matches dynamic import() calls.
var reDynImport = regexp.MustCompile(`\bimport\(['"]([^'"]+)['"]\)`)

// reConsoleLog matches console.log / console.error / console.warn calls.
var reConsoleLog = regexp.MustCompile(`\bconsole\.(log|error|warn|debug|info)\s*\(`)

// reNewAdapter matches `new SomethingAdapter` outside of module files.
var reNewAdapter = regexp.MustCompile(`\bnew\s+\w+Adapter\w*\s*\(`)

// HEX001 — Domain must not import from infrastructure or application.
type HEX001 struct{}

func (r HEX001) ID() string { return "HEX-001" }
func (r HEX001) Applies(path string) bool {
	return isInLayer(path, "domain") && isTS(path)
}
func (r HEX001) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		imp := extractImport(line)
		if imp == "" {
			continue
		}
		if containsLayer(imp, "infrastructure") || containsLayer(imp, "application") {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("Domain layer must not import from %q — violates hexagonal boundary", imp),
			})
		}
	}
	return vs
}

// HEX002 — Application must not import from infrastructure.
type HEX002 struct{}

func (r HEX002) ID() string { return "HEX-002" }
func (r HEX002) Applies(path string) bool {
	return isInLayer(path, "application") && isTS(path)
}
func (r HEX002) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		imp := extractImport(line)
		if imp == "" {
			continue
		}
		if containsLayer(imp, "infrastructure") {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("Application layer must not import from infrastructure %q", imp),
			})
		}
	}
	return vs
}

// HEX003 — No console.log/warn/error in src/ files.
type HEX003 struct{}

func (r HEX003) ID() string { return "HEX-003" }
func (r HEX003) Applies(path string) bool {
	return isInSrc(path) && isTS(path) && !isSpec(path)
}
func (r HEX003) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		if reConsoleLog.MatchString(line) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  "console.* is not allowed in src/ — use the injected Logger",
			})
		}
	}
	return vs
}

// HEX004 — Adapters must not be instantiated with `new` outside module files.
type HEX004 struct{}

func (r HEX004) ID() string { return "HEX-004" }
func (r HEX004) Applies(path string) bool {
	return isInSrc(path) && isTS(path) && !isModuleFile(path)
}
func (r HEX004) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		if reNewAdapter.MatchString(line) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityError,
				Message:  "Adapter must be wired via NestJS DI — never instantiated with `new` outside module files",
			})
		}
	}
	return vs
}

// HEX005 — No `any` without a korva-ignore comment.
type HEX005 struct{}

var reAnyType = regexp.MustCompile(`:\s*any\b`)
var reKorvaIgnore = regexp.MustCompile(`//\s*korva-ignore`)

func (r HEX005) ID() string { return "HEX-005" }
func (r HEX005) Applies(path string) bool {
	return isInSrc(path) && isTS(path)
}
func (r HEX005) Check(filePath string, lines []string) []Violation {
	var vs []Violation
	for i, line := range lines {
		if reAnyType.MatchString(line) && !reKorvaIgnore.MatchString(line) {
			vs = append(vs, Violation{
				File:     filePath,
				Line:     i + 1,
				Rule:     r.ID(),
				Severity: SeverityWarning,
				Message:  "Avoid `any` — add explicit type or // korva-ignore: <reason>",
			})
		}
	}
	return vs
}

// --- helpers ---

func extractImport(line string) string {
	if m := reImport.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	if m := reDynImport.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

func containsLayer(imp, layer string) bool {
	return strings.Contains(imp, "/"+layer+"/") || strings.Contains(imp, "/"+layer)
}

func isInLayer(path, layer string) bool {
	return strings.Contains(path, "/"+layer+"/") || strings.Contains(path, "/src/"+layer)
}

func isInSrc(path string) bool {
	return strings.Contains(path, "/src/") || strings.HasPrefix(path, "src/")
}

func isTS(path string) bool {
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")
}

func isSpec(path string) bool {
	return strings.Contains(path, ".spec.") || strings.Contains(path, ".test.")
}

func isModuleFile(path string) bool {
	return strings.HasSuffix(path, ".module.ts")
}
