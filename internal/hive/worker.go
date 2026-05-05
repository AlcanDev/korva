package hive

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alcandev/korva/internal/privacy/cloud"
)

// Worker drains the outbox, applies the cloud privacy filter,
// and ships accepted observations to Hive.
//
// One Worker per process. Construct with NewWorker, run with Run(ctx).
// Use FlushOnce for one-shot runs (CLI `korva hive push`).
// Worker.Status() is safe to call from any goroutine.
// ObservationSaver is the minimal interface the worker needs to persist pulled
// observations. Implemented by *store.Store — defined here to avoid import cycle.
type ObservationSaver interface {
	// SavePulled persists a remotely-sourced observation. Duplicate IDs are
	// silently ignored (ON CONFLICT DO NOTHING at the DB level).
	SavePulled(id, project, obsType, title, content, author string, tags []string) error
	// ExistsObservation returns true if an observation with this ID is already stored.
	ExistsObservation(id string) (bool, error)
}

type Worker struct {
	outbox   *Outbox
	client   *Client
	filter   *cloud.Filter
	saver    ObservationSaver // nil = pull disabled
	clientID string
	interval time.Duration

	mu          sync.RWMutex
	status      WorkerStatus
	lastPullCursor string // RFC3339 cursor; empty = pull from beginning
}

// NewWorker assembles a worker. interval is how often the worker ticks
// when running with Run(); 0 falls back to 15 minutes.
// saver may be nil to disable the pull side of bidirectional sync.
func NewWorker(outbox *Outbox, client *Client, filter *cloud.Filter, saver ObservationSaver, clientID string, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Worker{
		outbox:   outbox,
		client:   client,
		filter:   filter,
		saver:    saver,
		clientID: clientID,
		interval: interval,
		status:   WorkerStatus{Phase: PhaseIdle},
	}
}

// Status returns a point-in-time snapshot of the worker state.
// Safe to call concurrently from any goroutine (e.g. the HTTP status handler).
func (w *Worker) Status() WorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	s := w.status
	// Enrich with live pending count on every read.
	if counts, err := w.outbox.Status(); err == nil {
		s.PendingCount = counts.Pending
	}
	return s
}

func (w *Worker) setPhase(phase SyncPhase) {
	w.mu.Lock()
	w.status.Phase = phase
	w.mu.Unlock()
}

func (w *Worker) recordSuccess(now time.Time) {
	w.mu.Lock()
	w.status.Phase = PhaseHealthy
	w.status.LastSyncAt = &now
	w.status.ConsecutiveErrors = 0
	w.status.LastError = ""
	w.status.BackoffUntil = nil
	w.mu.Unlock()
}

func (w *Worker) recordPullSuccess(pulled int) {
	now := time.Now().UTC()
	w.mu.Lock()
	w.status.LastPullAt = &now
	w.status.PullCount += pulled
	w.mu.Unlock()
}

func (w *Worker) recordFailure(err error) {
	w.mu.Lock()
	w.status.ConsecutiveErrors++
	w.status.LastError = err.Error()
	until := time.Now().UTC().Add(jitterBackoff(w.status.ConsecutiveErrors))
	w.status.BackoffUntil = &until
	w.status.Phase = PhaseBackoff
	w.mu.Unlock()
}

// Run blocks until ctx is canceled, ticking every interval.
// Honors the KORVA_HIVE_DISABLE=1 kill switch on every tick.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		if killSwitch() {
			w.setPhase(PhaseDisabled)
			log.Printf("hive: KORVA_HIVE_DISABLE active, skipping tick")
		} else if w.inBackoff() {
			// Still cooling down from a previous failure — skip this tick.
		} else if err := w.tick(ctx); err != nil {
			log.Printf("hive: tick: %v", err)
			w.recordFailure(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// FlushOnce runs a single drain cycle. Returns the number of rows processed
// (sent + rejected + failed). Used by `korva hive push`.
func (w *Worker) FlushOnce(ctx context.Context) (int, error) {
	if killSwitch() {
		return 0, errors.New("hive disabled via KORVA_HIVE_DISABLE")
	}
	n, err := w.processBatch(ctx)
	if err != nil {
		w.recordFailure(err)
	} else if n > 0 {
		w.recordSuccess(time.Now().UTC())
	}
	return n, err
}

func (w *Worker) inBackoff() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status.BackoffUntil != nil && time.Now().UTC().Before(*w.status.BackoffUntil)
}

func (w *Worker) tick(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.client.Health(probeCtx); err != nil {
		// Offline: leave rows pending for the next tick — not an error.
		return nil
	}
	w.setPhase(PhasePushing)
	n, err := w.processBatch(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		w.recordSuccess(time.Now().UTC())
	} else {
		w.setPhase(PhaseIdle)
	}

	// Pull side — fetch observations from Hive into local store.
	if w.saver != nil {
		if pullErr := w.pullTick(ctx); pullErr != nil {
			// Pull failures are non-fatal: log but don't mark the worker as failed.
			log.Printf("hive: pull: %v", pullErr)
		}
	}
	return nil
}

// pullTick fetches the next page of observations from Hive and saves any new
// ones to the local store. It advances lastPullCursor on each successful call.
func (w *Worker) pullTick(ctx context.Context) error {
	w.mu.RLock()
	cursor := w.lastPullCursor
	w.mu.RUnlock()

	resp, err := w.client.PullBatch(ctx, cursor, 100)
	if err != nil {
		return fmt.Errorf("pull batch: %w", err)
	}
	if resp.Count == 0 {
		return nil
	}

	saved := 0
	for _, o := range resp.Observations {
		exists, err := w.saver.ExistsObservation(o.ID)
		if err != nil || exists {
			continue
		}
		tags := make([]string, 0, len(o.Tags))
		for _, t := range o.Tags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		if err := w.saver.SavePulled(o.ID, o.Project, o.Type, o.Title, o.Content, o.Author, tags); err != nil {
			log.Printf("hive: save pulled observation %s: %v", o.ID, err)
			continue
		}
		saved++
	}

	if resp.NextSince != "" {
		w.mu.Lock()
		w.lastPullCursor = resp.NextSince
		w.mu.Unlock()
	}

	if saved > 0 {
		w.recordPullSuccess(saved)
		log.Printf("hive: pulled %d new observations (cursor: %s)", saved, resp.NextSince)
	}
	return nil
}

func (w *Worker) processBatch(ctx context.Context) (int, error) {
	rows, err := w.outbox.NextBatch(50)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	type pendingItem struct {
		rowID  string
		obsID  string
		filter cloud.Output
	}

	var ready []pendingItem
	for _, r := range rows {
		var raw cloud.Input
		if err := json.Unmarshal(r.Payload, &raw); err != nil {
			_ = w.outbox.MarkRejected(r.ID, "payload unmarshal: "+err.Error())
			continue
		}
		out, dec, reason := w.filter.Process(raw)
		if dec == cloud.Reject {
			_ = w.outbox.MarkRejected(r.ID, reason)
			continue
		}
		ready = append(ready, pendingItem{rowID: r.ID, obsID: r.ObservationID, filter: out})
	}

	if len(ready) == 0 {
		return len(rows), nil
	}

	batch := BatchRequest{
		ClientID:     w.clientID,
		BatchID:      newBatchID(),
		Schema:       1,
		Observations: make([]any, 0, len(ready)),
	}
	for _, it := range ready {
		batch.Observations = append(batch.Observations, it.filter)
	}

	if _, err := w.client.PostBatch(ctx, batch); err != nil {
		for _, it := range ready {
			row := findRow(rows, it.rowID)
			_ = w.outbox.MarkFailed(it.rowID, row.Attempts, err.Error())
		}
		return len(rows), fmt.Errorf("hive post: %w", err)
	}

	for _, it := range ready {
		_ = w.outbox.MarkSent(it.rowID)
	}
	return len(rows), nil
}

func findRow(rows []Row, id string) Row {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	return Row{}
}

// killSwitch returns true if the user has set KORVA_HIVE_DISABLE=1.
func killSwitch() bool {
	v := os.Getenv("KORVA_HIVE_DISABLE")
	return v == "1" || v == "true"
}

// jitterBackoff returns a randomized delay after n consecutive errors.
// Base schedule: 30s, 2m, 10m, 1h, 6h, then capped at 30m.
// A ±25% jitter is applied to prevent thundering herds on shared infrastructure.
func jitterBackoff(consecutiveErrors int) time.Duration {
	bases := []time.Duration{
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		time.Hour,
		6 * time.Hour,
	}
	idx := consecutiveErrors - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(bases) {
		idx = len(bases) - 1
	}
	base := bases[idx]
	// ±25% jitter: random value in [-base/4, +base/4]
	quarter := base / 4
	jitter := time.Duration(rand.Int63n(int64(quarter)*2)) - quarter
	result := base + jitter
	if result < 0 {
		result = base
	}
	return result
}

// batchEntropy is a process-wide monotonic ULID entropy source. The mutex
// serializes access since ulid.Monotonic is not safe for concurrent reads.
//
// Same fix as store.newID: math/rand seeded with time.Now().UnixNano() per
// call produces duplicate IDs on Windows (16 ms time resolution). Using a
// shared monotonic source ensures strict ordering within the same millisecond.
var (
	batchEntropyMu sync.Mutex
	batchEntropy   = ulid.Monotonic(cryptorand.Reader, 0)
)

func newBatchID() string {
	batchEntropyMu.Lock()
	defer batchEntropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), batchEntropy).String()
}
