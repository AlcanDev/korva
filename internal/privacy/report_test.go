package privacy

import (
	"strings"
	"testing"
)

// Phase 9.1 — verify FilterReport + the global counter contract.

func TestFilterReport_TalliesPerType(t *testing.T) {
	ResetRedactionStats()
	text := "password=hunter2 token=abc123 api_key=xyz999"
	out, report := FilterReport(text, nil)

	if strings.Contains(out, "hunter2") {
		t.Errorf("password not redacted in output: %s", out)
	}
	if strings.Contains(out, "abc123") {
		t.Errorf("token not redacted: %s", out)
	}
	if strings.Contains(out, "xyz999") {
		t.Errorf("api_key not redacted: %s", out)
	}

	want := map[RedactionType]bool{
		RedactionPassword: true,
		RedactionToken:    true,
		RedactionAPIKey:   true,
	}
	got := map[RedactionType]bool{}
	for _, e := range report.Entries {
		got[e.Type] = e.CharsRemoved > 0
	}
	for tp := range want {
		if !got[tp] {
			t.Errorf("expected redaction type %q in report, got %+v", tp, report.Entries)
		}
	}
	if report.TotalCharsRemoved <= 0 {
		t.Errorf("TotalCharsRemoved = %d, want > 0", report.TotalCharsRemoved)
	}
}

func TestFilterReport_BearerAndPrivateTags(t *testing.T) {
	ResetRedactionStats()
	text := "Authorization: Bearer abcd.efgh.ijkl\n<private>my secret notes</private> trailing"
	_, report := FilterReport(text, nil)
	have := map[RedactionType]int{}
	for _, e := range report.Entries {
		have[e.Type] += e.CharsRemoved
	}
	if have[RedactionBearer] == 0 {
		t.Error("bearer redaction not recorded")
	}
	if have[RedactionPrivateTag] == 0 {
		t.Error("private tag redaction not recorded")
	}
}

func TestFilterReport_CustomKeywords(t *testing.T) {
	ResetRedactionStats()
	_, report := FilterReport("myCustomToken=12345", []string{"myCustomToken"})
	have := map[RedactionType]int{}
	for _, e := range report.Entries {
		have[e.Type] += e.CharsRemoved
	}
	if have[RedactionCustom] == 0 {
		t.Errorf("custom keyword not tallied: %+v", report.Entries)
	}
}

func TestRedactionStats_TracksCumulative(t *testing.T) {
	ResetRedactionStats()
	_, _ = FilterReport("password=foo token=bar", nil)
	_, _ = FilterReport("secret=baz", nil)

	stats := RedactionStats()
	if stats.TotalEvents != 2 {
		t.Errorf("TotalEvents = %d, want 2", stats.TotalEvents)
	}
	if stats.TotalCharsRemoved <= 0 {
		t.Errorf("TotalCharsRemoved = %d, want > 0", stats.TotalCharsRemoved)
	}
	if stats.ByType[RedactionPassword] == 0 {
		t.Error("ByType[password] should be > 0")
	}
	if stats.ByType[RedactionToken] == 0 {
		t.Error("ByType[token] should be > 0")
	}
	if stats.ByType[RedactionSecret] == 0 {
		t.Error("ByType[secret] should be > 0")
	}
}

func TestFilterReport_CleanTextNoCounters(t *testing.T) {
	ResetRedactionStats()
	out, report := FilterReport("just a normal sentence", nil)
	if out != "just a normal sentence" {
		t.Errorf("clean text mutated: %q", out)
	}
	if report.TotalCharsRemoved != 0 {
		t.Errorf("clean text reported chars removed: %d", report.TotalCharsRemoved)
	}
	stats := RedactionStats()
	if stats.TotalEvents != 0 {
		t.Errorf("counter incremented on clean text: %d", stats.TotalEvents)
	}
}

func TestFilter_LegacyAPIStillWorks(t *testing.T) {
	// Sanity: existing callers using Filter() see the same redacted output
	// they always did. We delegate to FilterReport so this test is what
	// guarantees we didn't change observable behaviour.
	got := Filter("password=foo", nil)
	if got != "password=[REDACTED]" {
		t.Errorf("legacy Filter() output drifted: %q", got)
	}
}
