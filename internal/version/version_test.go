package version

import (
	"strings"
	"testing"
)

func TestString_ContainsVersion(t *testing.T) {
	s := String()
	if !strings.Contains(s, Version) {
		t.Errorf("String() = %q, should contain Version %q", s, Version)
	}
}

func TestString_ContainsCommit(t *testing.T) {
	s := String()
	if !strings.Contains(s, Commit) {
		t.Errorf("String() = %q, should contain Commit %q", s, Commit)
	}
}

func TestString_ContainsDate(t *testing.T) {
	s := String()
	if !strings.Contains(s, Date) {
		t.Errorf("String() = %q, should contain Date %q", s, Date)
	}
}

func TestString_DefaultValues(t *testing.T) {
	// These are the default values set in the source
	if Version != "dev" {
		t.Errorf("default Version = %q, want %q", Version, "dev")
	}
	if Commit != "none" {
		t.Errorf("default Commit = %q, want %q", Commit, "none")
	}
	if Date != "unknown" {
		t.Errorf("default Date = %q, want %q", Date, "unknown")
	}
}

func TestString_Format(t *testing.T) {
	s := String()
	// Should be non-empty
	if s == "" {
		t.Error("String() returned empty string")
	}
	// Should contain the build keyword
	if !strings.Contains(s, "built") {
		t.Errorf("String() = %q, should contain 'built'", s)
	}
}
