package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminListDeferred_FiltersByStatus(t *testing.T) {
	s := newAPITestStore(t)
	_ = s.DeferApply("sync-1", store.DeferredEntityObservation, []byte(`{}`), "")
	_ = s.DeferApply("sync-2", store.DeferredEntityObservation, []byte(`{}`), "")
	_ = s.MarkDeferredApplied("sync-2")

	h := adminListDeferred(s)

	t.Run("default lists everything", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/cloud/deferred", nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		var resp struct {
			Deferred []store.DeferredApply `json:"deferred"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if len(resp.Deferred) != 2 {
			t.Errorf("len = %d, want 2", len(resp.Deferred))
		}
	})
	t.Run("filter to applied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/cloud/deferred?status=applied", nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		var resp struct {
			Deferred []store.DeferredApply `json:"deferred"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if len(resp.Deferred) != 1 || resp.Deferred[0].SyncID != "sync-2" {
			t.Errorf("filter leaked: %+v", resp.Deferred)
		}
	})
}

func TestAdminRetryDeferred_BumpsCount(t *testing.T) {
	s := newAPITestStore(t)
	_ = s.DeferApply("sync-1", store.DeferredEntityObservation, []byte(`{}`), "first")

	h := adminRetryDeferred(s)
	body, _ := json.Marshal(retryDeferredRequest{LastError: "still failing"})
	req := httptest.NewRequest(http.MethodPost, "/admin/cloud/deferred/sync-1/retry", bytes.NewReader(body))
	req.SetPathValue("sync_id", "sync-1")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, _ := s.GetDeferred("sync-1")
	if got.RetryCount != 1 {
		t.Errorf("retry_count = %d, want 1", got.RetryCount)
	}
	if got.LastError != "still failing" {
		t.Errorf("last_error = %q", got.LastError)
	}
}

func TestAdminRetryDeferred_MissingReturns404(t *testing.T) {
	s := newAPITestStore(t)
	h := adminRetryDeferred(s)
	req := httptest.NewRequest(http.MethodPost, "/admin/cloud/deferred/nope/retry", nil)
	req.SetPathValue("sync_id", "nope")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminMarkDeferredApplied_FlipsStatus(t *testing.T) {
	s := newAPITestStore(t)
	_ = s.DeferApply("sync-1", store.DeferredEntityObservation, []byte(`{}`), "")

	h := adminMarkDeferredApplied(s)
	req := httptest.NewRequest(http.MethodPost, "/admin/cloud/deferred/sync-1/applied", nil)
	req.SetPathValue("sync_id", "sync-1")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, _ := s.GetDeferred("sync-1")
	if got.ApplyStatus != store.DeferredStatusApplied {
		t.Errorf("status = %q, want applied", got.ApplyStatus)
	}
}

func TestAdminDeleteDeferred_RemovesRow(t *testing.T) {
	s := newAPITestStore(t)
	_ = s.DeferApply("sync-1", store.DeferredEntityObservation, []byte(`{}`), "")

	h := adminDeleteDeferred(s)
	req := httptest.NewRequest(http.MethodDelete, "/admin/cloud/deferred/sync-1", nil)
	req.SetPathValue("sync_id", "sync-1")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, _ := s.GetDeferred("sync-1")
	if got != nil {
		t.Errorf("expected row gone, got %+v", got)
	}
}

func TestAdminDeleteDeferred_MissingReturns404(t *testing.T) {
	s := newAPITestStore(t)
	h := adminDeleteDeferred(s)
	req := httptest.NewRequest(http.MethodDelete, "/admin/cloud/deferred/nope", nil)
	req.SetPathValue("sync_id", "nope")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
