package privacy

import (
	"strings"
	"testing"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantIn   string // expected substring in output
		wantOut  string // substring that should NOT be in output
	}{
		{
			name:    "password equals sign",
			input:   "DB config: password=supersecret123",
			wantIn:  "[REDACTED]",
			wantOut: "supersecret123",
		},
		{
			name:    "password colon",
			input:   "connect with password: hunter2",
			wantIn:  "[REDACTED]",
			wantOut: "hunter2",
		},
		{
			name:    "TOKEN uppercase",
			input:   "export TOKEN=abc123xyz",
			wantIn:  "[REDACTED]",
			wantOut: "abc123xyz",
		},
		{
			name:    "api_key",
			input:   "api_key=sk-1234567890abcdef",
			wantIn:  "[REDACTED]",
			wantOut: "sk-1234567890abcdef",
		},
		{
			name:    "ROLE_ID",
			input:   "ROLE_ID=my-vault-role-id",
			wantIn:  "[REDACTED]",
			wantOut: "my-vault-role-id",
		},
		{
			name:    "SECRET_ID",
			input:   "SECRET_ID=vault-secret-value",
			wantIn:  "[REDACTED]",
			wantOut: "vault-secret-value",
		},
		{
			name:    "Bearer token",
			input:   "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.payload.signature",
			wantIn:  "[REDACTED]",
			wantOut: "eyJhbGciOiJSUzI1NiJ9",
		},
		{
			name:    "private tag single line",
			input:   "before <private>secret content</private> after",
			wantIn:  "[REDACTED]",
			wantOut: "secret content",
		},
		{
			name:    "private tag multiline",
			input:   "text <private>\nline1\nline2\n</private> end",
			wantIn:  "[REDACTED]",
			wantOut: "line1",
		},
		{
			name:    "safe text unchanged",
			input:   "We decided to use hexagonal architecture for the insurances module.",
			wantIn:  "hexagonal architecture",
			wantOut: "[REDACTED]",
		},
		{
			name:    "client_secret",
			input:   "client_secret=oauth-client-secret-value",
			wantIn:  "[REDACTED]",
			wantOut: "oauth-client-secret-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(tt.input, nil)

			if tt.wantIn != "[REDACTED]" {
				if !strings.Contains(result, tt.wantIn) {
					t.Errorf("Filter(%q) = %q, missing expected %q", tt.input, result, tt.wantIn)
				}
			} else {
				if !strings.Contains(result, "[REDACTED]") {
					t.Errorf("Filter(%q) = %q, expected [REDACTED] to be present", tt.input, result)
				}
			}

			if tt.wantOut != "[REDACTED]" && strings.Contains(result, tt.wantOut) {
				t.Errorf("Filter(%q) = %q, sensitive value %q should have been redacted", tt.input, result, tt.wantOut)
			}
		})
	}
}

func TestFilterExtraPatterns(t *testing.T) {
	result := Filter("myCustomSecret=toplevel", []string{"myCustomSecret"})
	if strings.Contains(result, "toplevel") {
		t.Errorf("Custom pattern 'myCustomSecret' should have been redacted, got: %s", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("Expected [REDACTED] in output, got: %s", result)
	}
}

func TestContainsSensitiveData(t *testing.T) {
	if !ContainsSensitiveData("password=abc") {
		t.Error("'password=abc' should be detected as sensitive")
	}
	if !ContainsSensitiveData("Bearer sometoken") {
		t.Error("'Bearer sometoken' should be detected as sensitive")
	}
	if !ContainsSensitiveData("<private>data</private>") {
		t.Error("private tag should be detected as sensitive")
	}
	if ContainsSensitiveData("hexagonal architecture in NestJS") {
		t.Error("safe text should not be detected as sensitive")
	}
}

func TestStripPrivateTags(t *testing.T) {
	input := "public info <private>secret</private> more public"
	result := StripPrivateTags(input)

	if strings.Contains(result, "secret") {
		t.Errorf("StripPrivateTags should remove private content, got: %s", result)
	}
	if !strings.Contains(result, "public info") {
		t.Errorf("StripPrivateTags should preserve non-private content, got: %s", result)
	}
}
