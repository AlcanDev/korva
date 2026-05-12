package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCustomRulesFile_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rules.yaml")
	yaml := `version: 1
profile: custom
rules:
  - id: CUSTOM-001
    description: "no console.log in src"
    severity: error
    pattern: 'console\.(log|debug)'
    paths_include:
      - 'src/**/*.ts'
    paths_exclude:
      - 'src/**/*.spec.ts'
    message: "no console.log allowed"
`
	mustWrite(t, path, yaml)

	got, err := LoadCustomRulesFile(path)
	if err != nil {
		t.Fatalf("LoadCustomRulesFile() error = %v", err)
	}
	if got.Profile != "custom" {
		t.Errorf("Profile = %q, want custom", got.Profile)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("rules len = %d, want 1", len(got.Rules))
	}
	r := got.Rules[0]
	if r.IDValue != "CUSTOM-001" || r.Pattern == "" {
		t.Errorf("rule mismatch: %+v", r)
	}
}

func TestLoadCustomRulesFile_MissingFile(t *testing.T) {
	got, err := LoadCustomRulesFile("/nonexistent/path/rules.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if got == nil {
		t.Fatal("expected empty file struct, got nil")
	}
	if len(got.Rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(got.Rules))
	}
}

func TestLoadCustomRulesFile_InvalidRegex(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rules.yaml")
	yaml := `version: 1
rules:
  - id: BAD-001
    pattern: '['
`
	mustWrite(t, path, yaml)
	if _, err := LoadCustomRulesFile(path); err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestLoadCustomRulesFile_DuplicateID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rules.yaml")
	yaml := `version: 1
rules:
  - id: CUSTOM-001
    pattern: 'foo'
  - id: CUSTOM-001
    pattern: 'bar'
`
	mustWrite(t, path, yaml)
	if _, err := LoadCustomRulesFile(path); err == nil {
		t.Error("expected error for duplicate id")
	}
}

func TestLoadCustomRulesFile_InvalidID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rules.yaml")
	yaml := `version: 1
rules:
  - id: too-low
    pattern: 'foo'
`
	mustWrite(t, path, yaml)
	if _, err := LoadCustomRulesFile(path); err == nil {
		t.Error("expected error for lowercase id")
	}
}

func TestSaveCustomRulesFile_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rules.yaml")

	original := &CustomRulesFile{
		Version: 1,
		Profile: "custom",
		Rules: []*CustomRule{
			{IDValue: "CUSTOM-1", Pattern: `console\.log`, Message: "x"},
			{IDValue: "CUSTOM-2", Pattern: `\bany\b`, SeverityValue: SeverityWarning},
		},
	}
	if err := SaveCustomRulesFile(path, original); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	round, err := LoadCustomRulesFile(path)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if len(round.Rules) != 2 {
		t.Errorf("len(rules) = %d, want 2", len(round.Rules))
	}
	if round.Rules[1].SeverityValue != SeverityWarning {
		t.Errorf("severity = %q, want warning", round.Rules[1].SeverityValue)
	}
}

func TestCustomRule_Check_RegexMatch(t *testing.T) {
	r := &CustomRule{IDValue: "TEST-1", Pattern: `console\.(log|debug)`, Message: "no console"}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	violations := r.Check("src/app.ts", []string{
		"const x = 1",
		"console.log('hello')",
		"console.debug(x)",
	})
	if len(violations) != 2 {
		t.Errorf("violations = %d, want 2", len(violations))
	}
	if violations[0].Line != 2 {
		t.Errorf("line = %d, want 2", violations[0].Line)
	}
}

func TestCustomRule_Applies_PathFilters(t *testing.T) {
	r := &CustomRule{
		IDValue:      "TEST-2",
		Pattern:      "x",
		PathsInclude: []string{"src/**/*.ts"},
		PathsExclude: []string{"src/**/*.spec.ts"},
	}
	tests := []struct {
		path string
		want bool
	}{
		{"src/app.ts", true},
		{"src/utils/helper.ts", true},
		{"src/app.spec.ts", false}, // excluded
		{"test/app.ts", false},     // not included
		{"README.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := r.Applies(tc.path); got != tc.want {
				t.Errorf("Applies(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestCustomRule_Validate_Errors(t *testing.T) {
	tests := []struct {
		name string
		r    *CustomRule
	}{
		{"bad id", &CustomRule{IDValue: "lower", Pattern: "x"}},
		{"empty pattern", &CustomRule{IDValue: "TEST-3", Pattern: ""}},
		{"invalid regex", &CustomRule{IDValue: "TEST-4", Pattern: "["}},
		{"bad severity", &CustomRule{IDValue: "TEST-5", Pattern: "x", SeverityValue: "fatal"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.Validate(); err == nil {
				t.Error("expected error")
			}
		})
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
