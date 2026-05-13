package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Phase 8.5 — In-process event bus + SSE endpoint.
//
// The bus is intentionally simple: an in-memory fan-out by event kind to any
// number of subscribers. Anything in the API package that mutates user-
// visible state (save observation, judge conflict, run command, …) calls
// `eventBus.Publish(...)`; the SSE handler streams every event over a single
// long-lived response to dashboards listening to GET /admin/events.
//
// Why SSE and not WebSocket: events are server → client only, the protocol
// is dirt-simple (newline-delimited "data:" frames), and reverse proxies +
// the Hive sync path already speak it. WebSocket would add a dependency
// (gorilla/ws) for zero added value here.
//
// Why in-process and not Redis pub/sub: Korva is local-first. A single
// process serves the dashboard; a remote bus would defeat the privacy
// promise and add ops overhead we don't need.

// EventKind classifies the wire payload. Adding a new kind requires adding
// a Publish call where the state changes AND a UI handler — the empty enum
// gates accidental drift.
type EventKind string

const (
	EventObservationSaved EventKind = "observation_saved"
	EventSessionStarted   EventKind = "session_started"
	EventSessionEnded     EventKind = "session_ended"
	EventConflictDetected EventKind = "conflict_detected"
	EventCommandRun       EventKind = "command_run"
	EventExportWritten    EventKind = "export_written"
	EventHivePhaseChanged EventKind = "hive_phase_changed"
)

// Event is the wire shape sent over SSE. Keep it flat — easier for the
// browser EventSource API to parse and easier to stamp telemetry on.
type Event struct {
	Kind    EventKind `json:"kind"`
	Project string    `json:"project,omitempty"`
	Title   string    `json:"title,omitempty"`
	Actor   string    `json:"actor,omitempty"`
	Meta    any       `json:"meta,omitempty"`
	At      time.Time `json:"at"`
}

// EventBus dispatches events to N subscribers. Designed for low rates
// (humans producing observations); not optimised for millions/sec.
type EventBus struct {
	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
}

// NewEventBus returns a ready-to-use bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan Event]struct{})}
}

// Publish fans the event out to every subscriber. Slow subscribers are
// skipped (drop-on-backpressure) so a stuck client never blocks publishers.
// Stamps `At` if the caller forgot.
func (b *EventBus) Publish(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for ch := range b.subs {
		// Non-blocking send. If a subscriber's buffer is full, we drop this
		// event for that subscriber — better than head-of-line blocking
		// every other listener (and the publisher).
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribe returns a buffered channel that receives every future event
// until the context is canceled OR the bus is closed. The returned cancel
// fn detaches the subscriber explicitly. cancel is idempotent — calling
// it twice (or once after ctx.Done already triggered it) is safe.
func (b *EventBus) Subscribe(ctx context.Context) (<-chan Event, func()) {
	ch := make(chan Event, 32) // buffer 32 events — fine for human rates
	b.mu.Lock()
	if !b.closed {
		b.subs[ch] = struct{}{}
	}
	b.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			b.mu.Unlock()
			close(ch)
		})
	}
	// Auto-detach when the parent ctx ends.
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ch, cancel
}

// Close stops every subscription. Used in shutdown.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
}

// SubscriberCount is handy for tests + debug endpoints.
func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// adminEventsSSE handles GET /admin/events.
//
// Streams every event on the bus to the client. The connection stays open
// until either side closes it; we send a comment keep-alive ping every 25 s
// so intermediaries (reverse proxies, browsers) don't time out the response.
func adminEventsSSE(bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // nginx hint

		// Send a hello frame so the client knows the channel is alive.
		_, _ = fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ch, cancel := bus.Subscribe(r.Context())
		defer cancel()

		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				// SSE comment frames serve as keep-alive; no-op for clients.
				if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case ev, more := <-ch:
				if !more {
					return
				}
				data, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				// SSE frame: "event: kind\ndata: <json>\n\n"
				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
