package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Phase 8.5 — verify the event-bus + SSE-handler contract:
//   - Publish fans the event to every subscriber
//   - Slow subscribers don't block publishers (drop-on-backpressure)
//   - Subscribe auto-detaches when the ctx is canceled
//   - The SSE handler writes the "data: …" framing we expect

func TestEventBus_PublishFansOut(t *testing.T) {
	b := NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, ca := b.Subscribe(ctx)
	bch, cb := b.Subscribe(ctx)
	defer ca()
	defer cb()

	b.Publish(Event{Kind: EventObservationSaved, Title: "hi"})

	for _, ch := range []<-chan Event{a, bch} {
		select {
		case ev := <-ch:
			if ev.Kind != EventObservationSaved || ev.Title != "hi" {
				t.Errorf("unexpected event: %+v", ev)
			}
			if ev.At.IsZero() {
				t.Error("Publish should stamp At when caller didn't")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("subscriber didn't receive the event in time")
		}
	}
}

func TestEventBus_DropsOnBackpressure(t *testing.T) {
	b := NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Subscribe but never drain — the channel will fill at 32 events.
	_, c := b.Subscribe(ctx)
	defer c()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1_000; i++ {
			b.Publish(Event{Kind: EventCommandRun, Title: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
		// Good — publisher wasn't blocked by the slow subscriber.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("publisher blocked by full subscriber buffer")
	}
}

func TestEventBus_SubscribeAutoDetachesOnCtxCancel(t *testing.T) {
	b := NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	_, c := b.Subscribe(ctx)
	defer c()
	if got := b.SubscriberCount(); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}
	cancel()
	// Wait for the auto-detach goroutine.
	deadline := time.Now().Add(200 * time.Millisecond)
	for b.SubscriberCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := b.SubscriberCount(); got != 0 {
		t.Errorf("after ctx cancel, subscriber count = %d, want 0", got)
	}
}

func TestAdminEventsSSE_WritesEventFramesAndPing(t *testing.T) {
	b := NewEventBus()
	srv := httptest.NewServer(adminEventsSSE(b))
	defer srv.Close()

	// Use a cancelable context so we can shut down the long-lived SSE
	// connection deterministically — without this, httptest.Server.Close()
	// blocks for up to its 5s graceful-shutdown window waiting for the
	// SSE handler's select loop to notice. With ctx cancel + explicit body
	// close at the bottom of the test, total runtime stays under 1 second.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read the hello frame, then publish, then read the data frame.
	buf := make([]byte, 256)
	deadline := time.Now().Add(500 * time.Millisecond)
	gotHello := false
	for time.Now().Before(deadline) && !gotHello {
		n, _ := resp.Body.Read(buf)
		if n > 0 && strings.Contains(string(buf[:n]), ": connected") {
			gotHello = true
		}
	}
	if !gotHello {
		t.Fatal("never received hello frame")
	}

	// Concurrent publish from another goroutine — give the server a tick
	// to register the subscriber.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		b.Publish(Event{Kind: EventObservationSaved, Title: "live"})
	}()

	// Read up to 1 KiB or until we see our event payload.
	var collected strings.Builder
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, _ := resp.Body.Read(buf)
		if n > 0 {
			collected.WriteString(string(buf[:n]))
			if strings.Contains(collected.String(), "observation_saved") {
				break
			}
		}
	}
	wg.Wait()
	out := collected.String()
	if !strings.Contains(out, "event: observation_saved") {
		t.Errorf("missing event-line in stream:\n%s", out)
	}
	if !strings.Contains(out, "\"title\":\"live\"") {
		t.Errorf("missing payload in stream:\n%s", out)
	}

	// Explicit cancel so the SSE handler's `<-r.Context().Done()` case
	// fires immediately, letting srv.Close() return without waiting.
	cancel()
	// Wait briefly for the handler goroutine to detach.
	deadline = time.Now().Add(200 * time.Millisecond)
	for b.SubscriberCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAdminEventsSSE_RespectsClientCancellation(t *testing.T) {
	b := NewEventBus()
	srv := httptest.NewServer(adminEventsSSE(b))
	defer srv.Close()

	// Open + cancel the request mid-flight; subscriber must detach.
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_, _ = resp.Body.Read(make([]byte, 32))
		_ = resp.Body.Close()
	}
	// Give the handler a tick to unregister.
	deadline := time.Now().Add(500 * time.Millisecond)
	for b.SubscriberCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := b.SubscriberCount(); got != 0 {
		t.Errorf("client cancel left %d subscribers attached", got)
	}
}
