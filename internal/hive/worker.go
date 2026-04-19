package hive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alcandev/korva/internal/privacy/cloud"
)

// Worker drains the outbox, applies the cloud privacy filter,
// and ships accepted observations to Hive.
//
// One Worker per process. Construct with NewWorker, run with Run(ctx).
// Use FlushOnce for one-shot runs (CLI `korva hive push`).
type Worker struct {
	outbox   *Outbox
	client   *Client
	filter   *cloud.Filter
	clientID string
	interval time.Duration
}

// NewWorker assembles a worker. interval is how often the worker ticks
// when running with Run(); 0 falls back to 15 minutes.
func NewWorker(outbox *Outbox, client *Client, filter *cloud.Filter, clientID string, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Worker{
		outbox:   outbox,
		client:   client,
		filter:   filter,
		clientID: clientID,
		interval: interval,
	}
}

// Run blocks until ctx is cancelled, ticking every interval.
// Honours the KORVA_HIVE_DISABLE=1 kill switch on every tick.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		if killSwitch() {
			log.Printf("hive: KORVA_HIVE_DISABLE active, skipping tick")
		} else if err := w.tick(ctx); err != nil {
			log.Printf("hive: tick: %v", err)
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
	return w.processBatch(ctx)
}

func (w *Worker) tick(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.client.Health(probeCtx); err != nil {
		// Offline: leave rows pending for the next tick.
		return nil
	}
	_, err := w.processBatch(ctx)
	return err
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
		// Network or server failure: bump attempts on every row in this batch.
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
// Honoured by both Run and FlushOnce so an emergency stop never requires
// editing config or restarting Vault.
func killSwitch() bool {
	v := os.Getenv("KORVA_HIVE_DISABLE")
	return v == "1" || v == "true"
}

func newBatchID() string {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
