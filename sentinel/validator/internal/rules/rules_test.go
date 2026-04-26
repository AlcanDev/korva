package rules

import (
	"strings"
	"testing"
)

// helper: split a multi-line string into lines for Check().
func lines(src string) []string {
	return strings.Split(src, "\n")
}

// helper: assert violation count and optional rule/severity.
func assertViolations(t *testing.T, rule string, got []Violation, want int) {
	t.Helper()
	if len(got) != want {
		t.Errorf("[%s] expected %d violation(s), got %d: %+v", rule, want, len(got), got)
	}
}

func assertContains(t *testing.T, viol []Violation, ruleID string) {
	t.Helper()
	for _, v := range viol {
		if v.Rule == ruleID {
			return
		}
	}
	t.Errorf("expected violation with rule %q, not found in %+v", ruleID, viol)
}

// ---------------------------------------------------------------------------
// HEX-001: Domain must not import from infrastructure or application
// ---------------------------------------------------------------------------

func TestHEX001_Applies(t *testing.T) {
	r := HEX001{}
	cases := []struct {
		path  string
		match bool
	}{
		{"src/domain/ports/insurance.port.ts", true},
		{"src/domain/entities/policy.ts", true},
		{"src/application/services/insurance.service.ts", false}, // wrong layer
		{"src/infrastructure/adapters/cl/insurance.adapter.cl.ts", false},
		{"src/domain/ports/insurance.port.js", false}, // not .ts
	}
	for _, c := range cases {
		got := r.Applies(c.path)
		if got != c.match {
			t.Errorf("HEX001.Applies(%q) = %v, want %v", c.path, got, c.match)
		}
	}
}

func TestHEX001_Check(t *testing.T) {
	r := HEX001{}
	path := "src/domain/ports/insurance.port.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{
			name:    "clean import from external lib",
			src:     `import { Injectable } from '@nestjs/common';`,
			wantVio: 0,
		},
		{
			name:    "import from infrastructure — error",
			src:     `import { InsuranceAdapter } from '../infrastructure/adapters/insurance.adapter';`,
			wantVio: 1,
		},
		{
			name:    "import from application — error",
			src:     `import { InsuranceService } from '../application/services/insurance.service';`,
			wantVio: 1,
		},
		{
			name:    "dynamic import from infrastructure — error",
			src:     `const mod = import('../infrastructure/http/client');`,
			wantVio: 1,
		},
		{
			name:    "import from domain itself — ok",
			src:     `import { GetInsuranceOffersCommand } from '../domain/commands/get-offers.command';`,
			wantVio: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "HEX-001", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// HEX-002: Application must not import from infrastructure
// ---------------------------------------------------------------------------

func TestHEX002_Check(t *testing.T) {
	r := HEX002{}
	path := "src/application/services/insurance.service.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{
			name:    "import from domain port — ok",
			src:     `import { INSURANCE_PORT } from '../../domain/ports/insurance.port';`,
			wantVio: 0,
		},
		{
			name:    "import from infrastructure — error",
			src:     `import { InsuranceCLAdapter } from '../../infrastructure/adapters/cl/insurance.adapter.cl';`,
			wantVio: 1,
		},
		{
			name:    "import from common lib — ok",
			src:     `import { Inject } from '@nestjs/common';`,
			wantVio: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "HEX-002", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// HEX-003: No console.* in src/ (non-spec) files
// ---------------------------------------------------------------------------

func TestHEX003_Applies(t *testing.T) {
	r := HEX003{}
	cases := []struct {
		path  string
		match bool
	}{
		{"src/application/services/insurance.service.ts", true},
		{"src/domain/entities/policy.ts", true},
		{"src/application/services/insurance.service.spec.ts", false}, // spec excluded
		{"test/helpers.ts", false},                                    // not in src/
	}
	for _, c := range cases {
		got := r.Applies(c.path)
		if got != c.match {
			t.Errorf("HEX003.Applies(%q) = %v, want %v", c.path, got, c.match)
		}
	}
}

func TestHEX003_Check(t *testing.T) {
	r := HEX003{}
	path := "src/application/services/insurance.service.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{"console.log — error", `  console.log('something');`, 1},
		{"console.error — error", `  console.error(err);`, 1},
		{"console.warn — error", `  console.warn('deprecated');`, 1},
		{"this.logger.log — ok", `  this.logger.log('something');`, 0},
		{"comment with console.log — still caught", `// console.log('debug')`, 1},
		{"string containing console.log — ok", `const s = 'no console.log here but in a var';`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "HEX-003", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// HEX-004: No `new XxxAdapter(` outside module files
// ---------------------------------------------------------------------------

func TestHEX004_Applies(t *testing.T) {
	r := HEX004{}
	cases := []struct {
		path  string
		match bool
	}{
		{"src/application/services/insurance.service.ts", true},
		{"src/infrastructure/adapters/cl/insurance.adapter.cl.ts", true},
		{"src/insurance.module.ts", false}, // module file — excluded
		{"src/app.module.ts", false},
	}
	for _, c := range cases {
		got := r.Applies(c.path)
		if got != c.match {
			t.Errorf("HEX004.Applies(%q) = %v, want %v", c.path, got, c.match)
		}
	}
}

func TestHEX004_Check(t *testing.T) {
	r := HEX004{}
	path := "src/application/services/insurance.service.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{"new InsuranceAdapter( — error", `  const adapter = new InsuranceAdapter();`, 1},
		{"new InsuranceCLAdapter( — error", `  const a = new InsuranceCLAdapter(client);`, 1},
		{"injected adapter via DI — ok", `  constructor(@Inject(INSURANCE_PORT) private readonly port: IInsurancePort) {}`, 0},
		{"new Date() — ok", `  const d = new Date();`, 0},
		{"new HttpService() — ok (not *Adapter)", `  const svc = new HttpService();`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "HEX-004", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// HEX-005: No `any` without korva-ignore comment
// ---------------------------------------------------------------------------

func TestHEX005_Check(t *testing.T) {
	r := HEX005{}
	path := "src/domain/entities/policy.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{"explicit any — warning", `  private data: any;`, 1},
		{"any with ignore — ok", `  private data: any; // korva-ignore: external SDK type`, 0},
		{"typed field — ok", `  private data: PolicyData;`, 0},
		{"any inside string — ok", `  const msg = 'this is any text';`, 0},
		{"array of any — warning", `  items: any[];`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "HEX-005", got, c.wantVio)
			if c.wantVio > 0 && len(got) > 0 {
				if got[0].Severity != SeverityWarning {
					t.Errorf("expected Warning severity, got %v", got[0].Severity)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NAM-001: DTO classes must end with uppercase DTO
// ---------------------------------------------------------------------------

func TestNAM001_Applies(t *testing.T) {
	r := NAM001{}
	cases := []struct {
		path  string
		match bool
	}{
		{"src/infrastructure/dto/request.dto.ts", true},
		{"src/infrastructure/dto/response.dto.ts", true},
		{"src/domain/entities/policy.ts", false}, // not in dto/
		{"src/application/services/foo.service.ts", false},
	}
	for _, c := range cases {
		got := r.Applies(c.path)
		if got != c.match {
			t.Errorf("NAM001.Applies(%q) = %v, want %v", c.path, got, c.match)
		}
	}
}

func TestNAM001_Check(t *testing.T) {
	r := NAM001{}
	path := "src/infrastructure/dto/request.dto.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{"uppercase DTO — ok", `export class GetInsuranceOffersRequestDTO {}`, 0},
		{"lowercase Dto — error", `export class GetInsuranceOffersRequestDto {}`, 1},
		{"no suffix — ok (not flagged by this rule)", `export class GetInsuranceOffersRequest {}`, 0},
		{"interface — not a class, ok", `export interface GetOffersDto {}`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "NAM-001", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// NAM-002: Port tokens must be SCREAMING_SNAKE_CASE
// ---------------------------------------------------------------------------

func TestNAM002_Check(t *testing.T) {
	r := NAM002{}
	path := "src/domain/ports/insurance.port.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{"SCREAMING_SNAKE — ok", `export const INSURANCE_PORT = 'InsurancePort';`, 0},
		{"camelCase — error", `export const insurancePort = 'InsurancePort';`, 1},
		{"PascalCase — error", `export const InsurancePort = 'InsurancePort';`, 1},
		{"SINGLE_WORD — ok", `export const PORT = 'Port';`, 0},
		{"not a port export — ignored", `export const something = 'value';`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "NAM-002", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// NAM-003: Adapter files must follow naming pattern
// ---------------------------------------------------------------------------

func TestNAM003_Check(t *testing.T) {
	r := NAM003{}

	cases := []struct {
		name    string
		path    string
		wantVio int
	}{
		{"correct name — ok", "src/infrastructure/adapters/cl/insurance.adapter.cl.ts", 0},
		{"base adapter — ok", "src/infrastructure/adapters/base/insurance.adapter.base.ts", 0},
		{"minimal adapter — ok", "src/infrastructure/adapters/insurance.adapter.ts", 0},
		{"wrong name — warning", "src/infrastructure/adapters/cl/insurance-cl.ts", 1},
		{"no .adapter. in name", "src/infrastructure/adapters/cl/insurancecl.ts", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(c.path, []string{"// content"})
			assertViolations(t, "NAM-003", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// SEC-001: No hardcoded secrets
// ---------------------------------------------------------------------------

func TestSEC001_Applies(t *testing.T) {
	r := SEC001{}
	cases := []struct {
		path  string
		match bool
	}{
		{"src/infrastructure/adapters/insurance.adapter.ts", true},
		{"src/application/services/insurance.service.ts", true},
		{"src/application/services/insurance.service.spec.ts", false}, // spec excluded
	}
	for _, c := range cases {
		got := r.Applies(c.path)
		if got != c.match {
			t.Errorf("SEC001.Applies(%q) = %v, want %v", c.path, got, c.match)
		}
	}
}

func TestSEC001_Check(t *testing.T) {
	r := SEC001{}
	path := "src/infrastructure/adapters/insurance.adapter.ts"

	cases := []struct {
		name    string
		src     string
		wantVio int
	}{
		{
			name:    "hardcoded password — error",
			src:     `  const password = 'super-secret-123';`,
			wantVio: 1,
		},
		{
			name:    "hardcoded api_key — error",
			src:     `  const api_key = 'abcdef1234567890';`,
			wantVio: 1,
		},
		{
			name:    "from process.env — ok",
			src:     `  const password = process.env.DB_PASSWORD;`,
			wantVio: 0,
		},
		{
			name:    "from configService — ok",
			src:     `  const token = this.configService.get('API_TOKEN');`,
			wantVio: 0,
		},
		{
			name:    "short value under threshold — ok",
			src:     `  const secret = 'abc';`,
			wantVio: 0,
		},
		{
			name:    "property name contains token but value is injected — ok",
			src:     `  private readonly token = process.env.BEARER_TOKEN;`,
			wantVio: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(path, lines(c.src))
			assertViolations(t, "SEC-001", got, c.wantVio)
		})
	}
}

// ---------------------------------------------------------------------------
// TEST-001: Spec files must be co-located with source
// ---------------------------------------------------------------------------

func TestTEST001_Check(t *testing.T) {
	r := TEST001{}

	cases := []struct {
		name    string
		path    string
		wantVio int
	}{
		{"co-located — ok", "src/application/services/insurance.service.spec.ts", 0},
		{"in __tests__ — warning", "src/application/__tests__/insurance.service.spec.ts", 1},
		{"in /test/ dir — warning", "src/test/insurance.service.spec.ts", 1},
		{"co-located test — ok", "src/domain/entities/policy.test.ts", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Check(c.path, []string{"// test content"})
			assertViolations(t, "TEST-001", got, c.wantVio)
			if c.wantVio > 0 && len(got) > 0 {
				if got[0].Severity != SeverityWarning {
					t.Errorf("expected Warning severity, got %v", got[0].Severity)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AllRules: verify all 10 rules are registered
// ---------------------------------------------------------------------------

func TestAllRules_Count(t *testing.T) {
	all := AllRules()
	if len(all) != 10 {
		t.Errorf("expected 10 rules, got %d", len(all))
	}
}

func TestAllRules_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, r := range AllRules() {
		id := r.ID()
		if seen[id] {
			t.Errorf("duplicate rule ID: %q", id)
		}
		seen[id] = true
	}
}

func TestAllRules_ExpectedIDs(t *testing.T) {
	expected := []string{
		"HEX-001", "HEX-002", "HEX-003", "HEX-004", "HEX-005",
		"NAM-001", "NAM-002", "NAM-003",
		"SEC-001",
		"TEST-001",
	}
	all := AllRules()
	ids := make(map[string]bool)
	for _, r := range all {
		ids[r.ID()] = true
	}
	for _, want := range expected {
		if !ids[want] {
			t.Errorf("expected rule %q not found in AllRules()", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Multi-line file simulation
// ---------------------------------------------------------------------------

func TestHEX001_MultipleViolationsInFile(t *testing.T) {
	r := HEX001{}
	path := "src/domain/ports/policy.port.ts"
	src := `import { Injectable } from '@nestjs/common';
import { PolicyAdapter } from '../infrastructure/adapters/policy.adapter';
import { PolicyService } from '../application/services/policy.service';
export interface IPolicyPort {
  getPolicy(id: string): Promise<Policy>;
}`
	got := r.Check(path, lines(src))
	if len(got) != 2 {
		t.Errorf("expected 2 violations, got %d: %+v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("first violation should be on line 2, got %d", got[0].Line)
	}
	if got[1].Line != 3 {
		t.Errorf("second violation should be on line 3, got %d", got[1].Line)
	}
}

func TestHEX003_MultipleConsoles(t *testing.T) {
	r := HEX003{}
	path := "src/application/services/svc.ts"
	src := `export class Svc {
  doWork() {
    console.log('start');
    console.warn('warn');
    this.logger.log('ok');
    console.error('end');
  }
}`
	got := r.Check(path, lines(src))
	if len(got) != 3 {
		t.Errorf("expected 3 violations, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// RuleProfile tests
// ---------------------------------------------------------------------------

func TestRulesForProfile_Minimal_OnlySecRule(t *testing.T) {
	rs := RulesForProfile(ProfileMinimal)
	if len(rs) != 1 {
		t.Fatalf("minimal should have 1 rule, got %d", len(rs))
	}
	if rs[0].ID() != "SEC-001" {
		t.Errorf("minimal rule should be SEC-001, got %q", rs[0].ID())
	}
}

func TestRulesForProfile_Standard_HasKeyRules(t *testing.T) {
	rs := RulesForProfile(ProfileStandard)
	ids := make(map[string]bool)
	for _, r := range rs {
		ids[r.ID()] = true
	}
	for _, want := range []string{"HEX-001", "HEX-002", "HEX-003", "SEC-001"} {
		if !ids[want] {
			t.Errorf("standard profile missing rule %q", want)
		}
	}
	// standard should NOT include naming or testing rules
	for _, unwanted := range []string{"NAM-001", "NAM-002", "NAM-003", "TEST-001"} {
		if ids[unwanted] {
			t.Errorf("standard profile should not include %q", unwanted)
		}
	}
}

func TestRulesForProfile_Strict_HasAll(t *testing.T) {
	rs := RulesForProfile(ProfileStrict)
	all := AllRules()
	if len(rs) != len(all) {
		t.Errorf("strict should have %d rules, got %d", len(all), len(rs))
	}
}

func TestRulesForProfile_UnknownFallsBackToStandard(t *testing.T) {
	rs := RulesForProfile("unknown-profile")
	standard := RulesForProfile(ProfileStandard)
	if len(rs) != len(standard) {
		t.Errorf("unknown profile should fall back to standard (%d rules), got %d", len(standard), len(rs))
	}
}
