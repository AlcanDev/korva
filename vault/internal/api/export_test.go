package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminExportObsidian_WritesNotes(t *testing.T) {
	s := newAPITestStore(t)
	if _, err := s.Save(store.Observation{
		Project: "korva", Type: store.TypeDecision,
		Title: "Adopt ULID", Content: "x", TopicKey: "adopt-ulid",
	}); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	body, _ := json.Marshal(map[string]any{"out": out})
	req := httptest.NewRequest(http.MethodPost, "/admin/export/obsidian", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	adminExportObsidian(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp store.ObsidianExportResult
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", resp.FileCount)
	}
	if _, err := os.Stat(filepath.Join(out, "korva", "decision", "adopt-ulid.md")); err != nil {
		t.Errorf("expected note file: %v", err)
	}
}

func TestAdminExportObsidian_Validation(t *testing.T) {
	s := newAPITestStore(t)
	h := adminExportObsidian(s)

	tests := []struct {
		name string
		body string
	}{
		{"missing out", `{}`},
		{"empty out", `{"out":""}`},
		{"invalid json", `not-json`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/export/obsidian",
				bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}
