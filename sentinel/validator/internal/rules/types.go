// Package rules defines Korva Sentinel validation rules.
package rules

// Severity indicates how serious a violation is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Violation represents a single rule violation found in a file.
type Violation struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Rule is the interface every Korva rule must implement.
type Rule interface {
	// ID returns the unique rule identifier, e.g. "HEX-001".
	ID() string
	// Applies reports whether this rule should run on the given file path.
	Applies(filePath string) bool
	// Check analyzes the file content and returns any violations.
	Check(filePath string, lines []string) []Violation
}
