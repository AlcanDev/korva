package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/internal/privacy"
)

// Phase 9.1 — verify that the /admin/privacy/stats endpoint exposes the
// process-wide redaction counters.

func TestAdminPrivacyStats_ReflectsFilterReportActivity(t *testing.T) {
	privacy.ResetRedactionStats()

	// Drive the filter a few times.
	_, _ = privacy.FilterReport("password=hunter2 token=abc123", nil)
	_, _ = privacy.FilterReport("secret=topsecret", nil)
	_, _ = privacy.FilterReport("api_key=very-long-key-here-12345", nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/privacy/stats", nil)
	rec := httptest.NewRecorder()
	adminPrivacyStats()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp privacy.RedactionStatsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalEvents != 3 {
		t.Errorf("TotalEvents = %d, want 3", resp.TotalEvents)
	}
	if resp.TotalCharsRemoved <= 0 {
		t.Errorf("TotalCharsRemoved should be > 0")
	}
	// All four categories should appear.
	for _, want := range []privacy.RedactionType{
		privacy.RedactionPassword,
		privacy.RedactionToken,
		privacy.RedactionSecret,
		privacy.RedactionAPIKey,
	} {
		if resp.ByType[want] == 0 {
			t.Errorf("ByType[%s] = 0, expected > 0", want)
		}
	}
}

func TestAdminPrivacyStats_EmptyWhenNoRedactions(t *testing.T) {
	privacy.ResetRedactionStats()
	req := httptest.NewRequest(http.MethodGet, "/admin/privacy/stats", nil)
	rec := httptest.NewRecorder()
	adminPrivacyStats()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp privacy.RedactionStatsSnapshot
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.TotalEvents != 0 {
		t.Errorf("TotalEvents = %d, want 0 on fresh process", resp.TotalEvents)
	}
}
