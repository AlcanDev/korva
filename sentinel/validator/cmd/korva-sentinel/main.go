// korva-sentinel validates source files against Korva architecture rules.
//
// Usage:
//
//	# From a git pre-commit hook
//	git diff --cached --name-only | korva-sentinel
//
//	# Explicit file list
//	korva-sentinel src/domain/entities/insurance.entity.ts src/application/services/insurance.service.ts
//
//	# JSON output (for CI integration)
//	korva-sentinel --format json src/...
//
//	# Use a lighter rule profile for bootstrapping teams
//	korva-sentinel --profile minimal src/...
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alcandev/korva/sentinel/validator/internal/analyzer"
	"github.com/alcandev/korva/sentinel/validator/internal/rules"
)

func main() {
	format        := flag.String("format", "text", "Output format: text | json")
	failOnWarning := flag.Bool("fail-on-warning", false, "Exit 1 on warnings as well as errors")
	profile       := flag.String("profile", "standard", "Rule profile: minimal | standard | strict")
	flag.Parse()

	// File paths from args, or stdin if none provided
	paths := flag.Args()
	if len(paths) == 0 {
		paths = analyzer.ReadPathsFromStdin()
	}

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "korva-sentinel: no files to analyze")
		os.Exit(0)
	}

	selectedRules := rules.RulesForProfile(rules.RuleProfile(*profile))
	a := analyzer.New(selectedRules)
	report := a.AnalyzeFiles(paths)

	switch *format {
	case "json":
		if err := analyzer.PrintJSON(os.Stdout, report); err != nil {
			fmt.Fprintf(os.Stderr, "korva-sentinel: JSON encoding error: %v\n", err)
			os.Exit(2)
		}
	default:
		analyzer.PrintText(os.Stdout, report)
	}

	if report.Errors > 0 {
		os.Exit(1)
	}
	if *failOnWarning && report.Warnings > 0 {
		os.Exit(1)
	}
}
