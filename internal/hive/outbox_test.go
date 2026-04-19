package hive

import (
	"testing"

	"github.com/alcandev/korva/internal/db"
)

func newTestOutbox(t *testing.T) *Outbox {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("openmemory: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return NewOutbox(d)
}

func TestOutbox_EnqueueAndNextBatch(t *testing.T) {
	o := newTestOutbox(t)
	if err := o.Enqueue("obs-1", []byte(`{"id":"obs-1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := o.Enqueue("obs-2", []byte(`{"id":"obs-2"}`)); err != nil {
		t.Fatal(err)
	}
	rows, err := o.NextBatch(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestOutbox_StatusCounts(t *testing.T) {
	o := newTestOutbox(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := o.Enqueue(id, []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := o.NextBatch(10)
	_ = o.MarkSent(rows[0].ID)
	_ = o.MarkRejected(rows[1].ID, "test")

	c, err := o.Status()
	if err != nil {
		t.Fatal(err)
	}
	if c.Pending != 1 || c.Sent != 1 || c.Rejected != 1 {
		t.Errorf("unexpected counts: %+v", c)
	}
}

func TestOutbox_MarkFailedBackoffParksAtSix(t *testing.T) {
	o := newTestOutbox(t)
	_ = o.Enqueue("obs", []byte("{}"))
	rows, _ := o.NextBatch(10)
	id := rows[0].ID

	for i := 0; i < 6; i++ {
		if err := o.MarkFailed(id, i, "boom"); err != nil {
			t.Fatal(err)
		}
	}
	c, _ := o.Status()
	if c.Failed != 1 {
		t.Errorf("expected 1 failed row after 6 attempts, got %+v", c)
	}
}

func TestOutbox_RetryReenqueuesFailed(t *testing.T) {
	o := newTestOutbox(t)
	_ = o.Enqueue("obs", []byte("{}"))
	rows, _ := o.NextBatch(10)
	for i := 0; i < 6; i++ {
		_ = o.MarkFailed(rows[0].ID, i, "boom")
	}
	n, err := o.Retry()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 row retried, got %d", n)
	}
	c, _ := o.Status()
	if c.Pending != 1 || c.Failed != 0 {
		t.Errorf("retry did not move row to pending: %+v", c)
	}
}

func TestBackoffSchedule(t *testing.T) {
	cases := []struct {
		attempts int
		min      int // minimum expected seconds
	}{
		{0, 30}, {1, 30}, {2, 120}, {3, 600}, {4, 3600}, {5, 21600},
	}
	for _, c := range cases {
		got := backoff(c.attempts).Seconds()
		if int(got) < c.min {
			t.Errorf("backoff(%d) = %.0fs, want at least %d", c.attempts, got, c.min)
		}
	}
}
