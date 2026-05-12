package store

import (
	"testing"
)

func TestDeferApply_HappyPath(t *testing.T) {
	s := newTestStore(t)
	err := s.DeferApply("sync-1", DeferredEntityRelation, []byte(`{"x":1}`), "FK missing")
	if err != nil {
		t.Fatalf("DeferApply: %v", err)
	}

	got, err := s.GetDeferred("sync-1")
	if err != nil {
		t.Fatalf("GetDeferred: %v", err)
	}
	if got == nil {
		t.Fatal("expected row")
	}
	if got.ApplyStatus != DeferredStatusDeferred {
		t.Errorf("status = %q, want deferred", got.ApplyStatus)
	}
	if got.RetryCount != 0 {
		t.Errorf("retry_count = %d, want 0", got.RetryCount)
	}
	if got.LastError != "FK missing" {
		t.Errorf("last_error = %q", got.LastError)
	}
}

func TestDeferApply_RejectsUnknownEntity(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeferApply("sync-x", "alien", []byte(`{}`), ""); err == nil {
		t.Error("expected error for unknown entity")
	}
}

func TestDeferApply_UpsertsLastError(t *testing.T) {
	s := newTestStore(t)
	_ = s.DeferApply("sync-1", DeferredEntityObservation, []byte(`{}`), "first")
	// IncrementRetry to non-zero so we can verify the upsert does not reset.
	_ = s.IncrementDeferredRetry("sync-1", "second")
	_ = s.DeferApply("sync-1", DeferredEntityObservation, []byte(`{}`), "third")

	got, _ := s.GetDeferred("sync-1")
	if got.LastError != "third" {
		t.Errorf("last_error = %q, want third", got.LastError)
	}
	if got.RetryCount != 1 {
		t.Errorf("retry_count = %d, want 1 (upsert preserves counter)", got.RetryCount)
	}
}

func TestListDeferred_FIFO(t *testing.T) {
	s := newTestStore(t)
	for _, id := range []string{"sync-c", "sync-a", "sync-b"} {
		if err := s.DeferApply(id, DeferredEntityObservation, []byte(`{}`), ""); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	got, err := s.ListDeferred(DeferredStatusDeferred, 10)
	if err != nil {
		t.Fatalf("ListDeferred: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	// Insertion order (FIFO) is c, a, b — the listing returns them oldest-first.
	if got[0].SyncID != "sync-c" {
		t.Errorf("FIFO order broken: first = %q, want sync-c", got[0].SyncID)
	}
}

func TestListDeferred_FilterByStatus(t *testing.T) {
	s := newTestStore(t)
	_ = s.DeferApply("alive", DeferredEntityObservation, []byte(`{}`), "")
	_ = s.DeferApply("done", DeferredEntityObservation, []byte(`{}`), "")
	_ = s.MarkDeferredApplied("done")

	applied, _ := s.ListDeferred(DeferredStatusApplied, 10)
	if len(applied) != 1 || applied[0].SyncID != "done" {
		t.Errorf("applied filter returned %+v", applied)
	}
	deferred, _ := s.ListDeferred(DeferredStatusDeferred, 10)
	if len(deferred) != 1 || deferred[0].SyncID != "alive" {
		t.Errorf("deferred filter returned %+v", deferred)
	}
}

func TestMarkDeferredApplied_FlipsStatus(t *testing.T) {
	s := newTestStore(t)
	_ = s.DeferApply("sync-1", DeferredEntityObservation, []byte(`{}`), "")
	if err := s.MarkDeferredApplied("sync-1"); err != nil {
		t.Fatalf("MarkDeferredApplied: %v", err)
	}
	got, _ := s.GetDeferred("sync-1")
	if got.ApplyStatus != DeferredStatusApplied {
		t.Errorf("status = %q, want applied", got.ApplyStatus)
	}
	if got.LastAttemptedAt == nil {
		t.Error("last_attempted_at must be populated after MarkDeferredApplied")
	}
}

func TestIncrementDeferredRetry_BumpsCountAndDiesOnCap(t *testing.T) {
	s := newTestStore(t)
	_ = s.DeferApply("sync-1", DeferredEntityObservation, []byte(`{}`), "")

	// Cycle until just at the cap — status should still be 'deferred'.
	for i := 0; i < deadDeferredAttempts; i++ {
		if err := s.IncrementDeferredRetry("sync-1", "still failing"); err != nil {
			t.Fatalf("retry %d: %v", i, err)
		}
	}
	got, _ := s.GetDeferred("sync-1")
	if got.ApplyStatus != DeferredStatusDeferred {
		t.Errorf("at exactly cap, status = %q, want deferred", got.ApplyStatus)
	}
	if got.RetryCount != deadDeferredAttempts {
		t.Errorf("retry_count = %d, want %d", got.RetryCount, deadDeferredAttempts)
	}

	// One more push past the cap flips to 'dead'.
	if err := s.IncrementDeferredRetry("sync-1", "give up"); err != nil {
		t.Fatalf("over-cap retry: %v", err)
	}
	got, _ = s.GetDeferred("sync-1")
	if got.ApplyStatus != DeferredStatusDead {
		t.Errorf("over cap, status = %q, want dead", got.ApplyStatus)
	}
}

func TestIncrementDeferredRetry_MissingRowErrors(t *testing.T) {
	s := newTestStore(t)
	if err := s.IncrementDeferredRetry("missing", "x"); err == nil {
		t.Error("expected error for missing sync_id")
	}
}

func TestDeleteDeferred_RemovesRow(t *testing.T) {
	s := newTestStore(t)
	_ = s.DeferApply("sync-1", DeferredEntityObservation, []byte(`{}`), "")
	ok, err := s.DeleteDeferred("sync-1")
	if err != nil {
		t.Fatalf("DeleteDeferred: %v", err)
	}
	if !ok {
		t.Error("expected deleted=true")
	}
	got, _ := s.GetDeferred("sync-1")
	if got != nil {
		t.Errorf("expected row gone, got %+v", got)
	}
}
