package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestExtractPhrases_FindsBigramsAndTrigrams(t *testing.T) {
	phrases := extractPhrases("We use ulid for primary keys always")
	// Stopwords + ≥3-char filter strip "we", "use", "for" → tokens:
	// ["ulid", "primary", "keys", "always"]
	// bigrams: "ulid primary", "primary keys", "keys always"
	// trigrams: "ulid primary keys", "primary keys always"
	// All 5 ≥ minPhraseLen (8 chars).
	expected := []string{
		"ulid primary",
		"primary keys",
		"keys always",
		"ulid primary keys",
		"primary keys always",
	}
	for _, want := range expected {
		if _, ok := phrases[want]; !ok {
			t.Errorf("expected phrase %q in result, got %+v", want, phrases)
		}
	}
}

func TestExtractPhrases_SkipsShortAndStopwords(t *testing.T) {
	// "of the" → both stopwords, drop.
	phrases := extractPhrases("of the X is")
	if len(phrases) != 0 {
		t.Errorf("expected zero phrases (all stopwords or short), got %+v", phrases)
	}
}

func TestComputePatternSuggestions_FlagsRecurring(t *testing.T) {
	obs := []store.Observation{
		{ID: "1", Project: "korva", Type: store.TypeContext, Title: "ULID for primary keys", Content: "decided"},
		{ID: "2", Project: "korva", Type: store.TypeContext, Title: "ULID for primary keys again", Content: "x"},
		{ID: "3", Project: "korva", Type: store.TypeContext, Title: "use ULID for primary keys here too", Content: "y"},
	}
	out := computePatternSuggestions(obs, 2)
	// At least one suggestion should mention "primary keys" with count >= 2.
	found := false
	for _, s := range out {
		if s.Phrase == "primary keys" && s.Count >= 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a suggestion for 'primary keys', got %+v", out)
	}
}

func TestComputePatternSuggestions_AlreadyPatternFlag(t *testing.T) {
	obs := []store.Observation{
		{ID: "1", Project: "korva", Type: store.TypePattern, Title: "Outbox pattern", Content: "x"},
		{ID: "2", Project: "korva", Type: store.TypeContext, Title: "Outbox pattern used here", Content: "y"},
		{ID: "3", Project: "korva", Type: store.TypeContext, Title: "Outbox pattern used there", Content: "z"},
	}
	out := computePatternSuggestions(obs, 2)
	hasOutbox := false
	for _, s := range out {
		if s.Phrase == "outbox pattern" {
			hasOutbox = true
			if !s.AlreadyPattern {
				t.Error("AlreadyPattern should be true when at least one obs is a pattern")
			}
			if s.Severity != "info" {
				t.Errorf("Severity should be info when AlreadyPattern, got %q", s.Severity)
			}
		}
	}
	if !hasOutbox {
		t.Errorf("expected an outbox suggestion, got %+v", out)
	}
}

func TestAdminPatternInsights_EmptyStore(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/insights/patterns", nil)
	rec := httptest.NewRecorder()
	adminPatternInsights(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp InsightsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions on empty store, got %d", len(resp.Suggestions))
	}
}
