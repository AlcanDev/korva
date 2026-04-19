package cloud

import (
	"strings"
	"testing"
	"time"
)

func newTestFilter() *Filter {
	return New([]string{"pattern", "decision", "learning"}, "test-install-salt-32-bytes-hex-fake")
}

func TestProcess_AcceptsCleanObservation(t *testing.T) {
	f := newTestFilter()
	in := Input{
		ID:        "01J0000000000000000000ZZZZ",
		Type:      "pattern",
		Title:     "Use repository pattern for data access",
		Content:   "Wrap SQL queries behind a repository to keep handlers thin.",
		Tags:      []string{"go", "architecture"},
		Project:   "korva",
		Team:      "core",
		Author:    "felipe",
		CreatedAt: time.Now().UTC(),
	}
	out, dec, reason := f.Process(in)
	if dec != Accept {
		t.Fatalf("expected Accept, got Reject: %s", reason)
	}
	if out.ProjectHash == "" || out.TeamHash == "" || out.AuthorHash == "" {
		t.Errorf("identifiers were not anonymized: %+v", out)
	}
	if out.Title != in.Title {
		t.Errorf("clean title was modified: %q", out.Title)
	}
}

func TestProcess_RejectsHardBlockedTypes(t *testing.T) {
	f := newTestFilter()
	for _, badType := range []string{"skill", "credential", "incident", "private"} {
		_, dec, reason := f.Process(Input{Type: badType, Content: "x", CreatedAt: time.Now()})
		if dec != Reject {
			t.Errorf("type %q should be hard-blocked", badType)
		}
		if !strings.Contains(reason, "hard-blocked") {
			t.Errorf("expected hard-blocked reason for %q, got %q", badType, reason)
		}
	}
}

func TestProcess_RejectsTypeNotInAllowlist(t *testing.T) {
	f := newTestFilter()
	_, dec, reason := f.Process(Input{Type: "bugfix", Content: "x", CreatedAt: time.Now()})
	if dec != Reject || !strings.Contains(reason, "not in cloud allowlist") {
		t.Fatalf("expected allowlist rejection, got dec=%v reason=%q", dec, reason)
	}
}

func TestProcess_RejectsEmptyContent(t *testing.T) {
	f := newTestFilter()
	_, dec, _ := f.Process(Input{Type: "pattern", Content: "   "})
	if dec != Reject {
		t.Fatal("empty content must be rejected")
	}
}

func TestProcess_RejectsResidualPrivateMarker(t *testing.T) {
	f := newTestFilter()
	_, dec, reason := f.Process(Input{
		Type:    "pattern",
		Content: "Use this approach <private>internal-only details</private>.",
	})
	if dec != Reject || !strings.Contains(reason, "private") {
		t.Fatalf("expected private-marker rejection, got dec=%v reason=%q", dec, reason)
	}
}

func TestProcess_RejectsContentWithMultiplePIIHits(t *testing.T) {
	f := newTestFilter()
	// email + ipv4 → 2 distinct PII patterns → reject
	in := Input{
		Type:    "pattern",
		Content: "Contact admin@example.com from 10.0.0.42 for the staging cluster.",
	}
	_, dec, reason := f.Process(in)
	if dec != Reject {
		t.Fatalf("multi-PII content should be rejected, got Accept (%s)", reason)
	}
}

func TestProcess_AcceptsContentWithSinglePIIRedacted(t *testing.T) {
	f := newTestFilter()
	in := Input{
		Type:    "pattern",
		Content: "Email is allowed in isolation: contact@example.com only.",
	}
	out, dec, reason := f.Process(in)
	if dec != Accept {
		t.Fatalf("single-PII content should be accepted with redaction, got Reject: %s", reason)
	}
	if strings.Contains(out.Content, "@example.com") {
		t.Errorf("email leaked through: %q", out.Content)
	}
	if !strings.Contains(out.Content, "[EMAIL]") {
		t.Errorf("email was not replaced with placeholder: %q", out.Content)
	}
}

func TestProcess_RejectsOversizedContent(t *testing.T) {
	f := newTestFilter()
	big := strings.Repeat("a", 10*1024)
	_, dec, reason := f.Process(Input{Type: "pattern", Content: big})
	if dec != Reject || !strings.Contains(reason, "size cap") {
		t.Fatalf("oversized content should be rejected, got dec=%v reason=%q", dec, reason)
	}
}

func TestPIIDetector_GoldenFixtures(t *testing.T) {
	d := defaultPIIDetector()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"email", "user@corp.io", "[EMAIL]"},
		{"ipv4", "192.168.1.1", "[IPV4]"},
		{"ipv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "[IPV6]"},
		{"unix_path", "/Users/felipe/projects/secret", "[USER_PATH]"},
		{"home_path", "/home/alice/code/repo", "[USER_PATH]"},
		{"win_path", `C:\Users\bob\projects`, "[USER_PATH]"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature_here", "[JWT]"},
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", "[UUID]"},
		{"long_hex", "abcdef0123456789abcdef0123456789", "[HEX]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, hits := d.scrub(c.in)
			if hits == 0 {
				t.Fatalf("pattern %s did not match input %q", c.name, c.in)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("expected placeholder %s in %q, got %q", c.want, c.in, out)
			}
		})
	}
}

func TestHashField_StableAndAnonymous(t *testing.T) {
	salt := []byte("install-salt-x")
	h1 := hashField("korva", salt)
	h2 := hashField("KORVA", salt)
	if h1 != h2 {
		t.Errorf("hash should be case-insensitive: %s != %s", h1, h2)
	}
	if len(h1) != 32 {
		t.Errorf("expected 32 hex chars, got %d", len(h1))
	}
	if hashField("", salt) != "" {
		t.Error("empty input should hash to empty string")
	}
	other := hashField("other", salt)
	if h1 == other {
		t.Error("different inputs collided")
	}
	otherSalt := hashField("korva", []byte("different-salt"))
	if h1 == otherSalt {
		t.Error("hash must depend on salt")
	}
}

func TestSanitizeTags_DropsBlanksAndTooLong(t *testing.T) {
	got := sanitizeTags([]string{"go", "", "  ", strings.Repeat("x", 100), "ok"})
	if len(got) != 2 || got[0] != "go" || got[1] != "ok" {
		t.Errorf("unexpected sanitized tags: %v", got)
	}
}
