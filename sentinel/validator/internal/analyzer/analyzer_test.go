package analyzer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/sentinel/validator/internal/rules"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeTemp creates a temporary .ts file with the given content.
// Returns the file path. Caller is responsible for cleanup via t.TempDir().
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// AnalyzeFiles
// ---------------------------------------------------------------------------

func TestAnalyzeFiles_EmptyInput(t *testing.T) {
	a := New(nil)
	report := a.AnalyzeFiles([]string{})
	if report.Files != 0 {
		t.Errorf("expected 0 files, got %d", report.Files)
	}
	if len(report.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(report.Violations))
	}
}

func TestAnalyzeFiles_SkipsEmptyPaths(t *testing.T) {
	a := New(nil)
	report := a.AnalyzeFiles([]string{"", "  "})
	if report.Files != 0 {
		t.Errorf("expected 0 files for empty paths, got %d", report.Files)
	}
}

func TestAnalyzeFiles_SkipsMissingFiles(t *testing.T) {
	a := New(nil)
	report := a.AnalyzeFiles([]string{"/nonexistent/path/to/file.ts"})
	// Missing files should be skipped silently — no panic, no error
	if report.Files != 0 {
		t.Errorf("expected 0 files for missing file, got %d", report.Files)
	}
}

func TestAnalyzeFiles_CleanFile(t *testing.T) {
	dir := t.TempDir()
	// File lives directly in temp dir — path won't match any Applies() filter
	// (no /src/ segment in the path), so 0 violations expected regardless of content
	path := writeTemp(t, dir, "clean.service.ts",
		`export class CleanService {
  doWork(): string { return 'result'; }
}
`)
	a := New(nil)
	report := a.AnalyzeFiles([]string{path})
	if report.Files != 1 {
		t.Errorf("expected 1 file, got %d", report.Files)
	}
	if len(report.Violations) != 0 {
		t.Errorf("expected 0 violations for non-src path, got %d", len(report.Violations))
	}
}

func TestAnalyzeFiles_DetectsConsoleLog(t *testing.T) {
	dir := t.TempDir()
	// Create a file at a path that matches HEX003's Applies() filter
	srcDir := filepath.Join(dir, "src", "application", "services")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := writeTemp(t, srcDir, "bad.service.ts",
		`export class BadService {
  doWork() {
    console.log('this is bad');
    console.error('also bad');
    this.logger.log('this is ok');
  }
}
`)
	a := New(nil)
	report := a.AnalyzeFiles([]string{path})
	if report.Files != 1 {
		t.Fatalf("expected 1 file, got %d", report.Files)
	}
	if report.Errors != 2 {
		t.Errorf("expected 2 errors (2 console.*), got %d errors %d warnings",
			report.Errors, report.Warnings)
	}
}

func TestAnalyzeFiles_DetectsHardcodedSecret(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "infrastructure", "adapters")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := writeTemp(t, srcDir, "bad.adapter.ts",
		`export class BadAdapter {
  private readonly apiKey = 'super-secret-api-key-12345';
  private readonly password = process.env.DB_PASSWORD; // this is ok
}
`)
	a := New(nil)
	report := a.AnalyzeFiles([]string{path})
	if report.Errors < 1 {
		t.Errorf("expected at least 1 error for hardcoded secret, got %d errors", report.Errors)
	}
}

func TestAnalyzeFiles_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "application", "services")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	clean := writeTemp(t, srcDir, "clean.service.ts",
		`export class CleanService { ok() { this.logger.log('ok'); } }`)
	dirty := writeTemp(t, srcDir, "dirty.service.ts",
		`export class DirtyService { bad() { console.log('bad'); } }`)

	a := New(nil)
	report := a.AnalyzeFiles([]string{clean, dirty})

	if report.Files != 2 {
		t.Errorf("expected 2 files analyzed, got %d", report.Files)
	}
	if report.Errors != 1 {
		t.Errorf("expected 1 error (console.log), got %d", report.Errors)
	}
}

func TestAnalyzeFiles_CustomRules(t *testing.T) {
	// Use only HEX003 to verify custom rule injection works
	a := New([]rules.Rule{rules.HEX003{}})
	if len(a.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(a.rules))
	}
}

func TestAnalyzeFiles_ErrorCount(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "application", "services")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := writeTemp(t, srcDir, "multi.service.ts",
		`export class Svc {
  run() {
    console.log('a');
    console.warn('b');
    console.error('c');
  }
}`)
	a := New([]rules.Rule{rules.HEX003{}})
	report := a.AnalyzeFiles([]string{path})
	if report.Errors != 3 {
		t.Errorf("expected 3 errors, got %d", report.Errors)
	}
	if report.Warnings != 0 {
		t.Errorf("expected 0 warnings, got %d", report.Warnings)
	}
}

// ---------------------------------------------------------------------------
// PrintText
// ---------------------------------------------------------------------------

func TestPrintText_NoViolations(t *testing.T) {
	var buf bytes.Buffer
	r := Report{Files: 5}
	PrintText(&buf, r)

	out := buf.String()
	if !strings.Contains(out, "5 file(s) analyzed") {
		t.Errorf("expected file count in output, got: %q", out)
	}
	if !strings.Contains(out, "no violations") {
		t.Errorf("expected 'no violations' in output, got: %q", out)
	}
}

func TestPrintText_WithViolations(t *testing.T) {
	var buf bytes.Buffer
	r := Report{
		Files:  2,
		Errors: 1, Warnings: 1,
		Violations: []rules.Violation{
			{File: "src/bad.ts", Line: 5, Rule: "HEX-003", Severity: rules.SeverityError, Message: "console.log not allowed"},
			{File: "src/warn.ts", Line: 10, Rule: "HEX-005", Severity: rules.SeverityWarning, Message: "avoid any"},
		},
	}
	PrintText(&buf, r)

	out := buf.String()
	if !strings.Contains(out, "HEX-003") {
		t.Errorf("expected HEX-003 in output")
	}
	if !strings.Contains(out, "HEX-005") {
		t.Errorf("expected HEX-005 in output")
	}
	if !strings.Contains(out, "src/bad.ts") {
		t.Errorf("expected file path in output")
	}
	if !strings.Contains(out, "1 error(s)") {
		t.Errorf("expected error count in output")
	}
}

// ---------------------------------------------------------------------------
// PrintJSON
// ---------------------------------------------------------------------------

func TestPrintJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	r := Report{
		Files:  3,
		Errors: 1,
		Violations: []rules.Violation{
			{File: "src/bad.ts", Line: 5, Rule: "HEX-003", Severity: rules.SeverityError},
		},
	}
	if err := PrintJSON(&buf, r); err != nil {
		t.Fatalf("PrintJSON returned error: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if decoded.Files != 3 {
		t.Errorf("expected files=3, got %d", decoded.Files)
	}
	if len(decoded.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(decoded.Violations))
	}
}

func TestPrintJSON_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	r := Report{}
	if err := PrintJSON(&buf, r); err != nil {
		t.Fatalf("PrintJSON with empty report: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Errorf("output is not valid JSON: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// New defaults
// ---------------------------------------------------------------------------

func TestNew_NilUsesAllRules(t *testing.T) {
	a := New(nil)
	all := rules.AllRules()
	if len(a.rules) != len(all) {
		t.Errorf("expected %d default rules, got %d", len(all), len(a.rules))
	}
}

func TestNew_CustomRulesSet(t *testing.T) {
	custom := []rules.Rule{rules.HEX001{}, rules.HEX002{}}
	a := New(custom)
	if len(a.rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(a.rules))
	}
}
