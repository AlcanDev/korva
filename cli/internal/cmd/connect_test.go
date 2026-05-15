package cmd

import "testing"

func TestTeamConfigBaseURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://licensing.korva.dev/v1/activate", "https://licensing.korva.dev"},
		{"https://licensing.korva.dev/v1/heartbeat", "https://licensing.korva.dev"},
		{"https://example.com", "https://example.com"},
		{"", ""},
	}
	for _, tc := range cases {
		got := teamConfigBaseURL(tc.input)
		if got != tc.want {
			t.Errorf("teamConfigBaseURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
