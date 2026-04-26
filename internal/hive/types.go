// Package hive talks to the Korva community cloud (Korva Hive).
//
// Architecture:
//
//	Store.Save  ──►  outbox.Enqueue  ──►  worker tick  ──►  cloud.Filter ──►  client.PostBatch
//	                  (sqlite local)                          (decide here)
//
// All cloud-related responsibilities live in this package. The Store knows
// nothing about Hive other than how to enqueue raw observations.
package hive

import "time"

// Status values stored in cloud_outbox.status.
const (
	StatusPending  = "pending"
	StatusSent     = "sent"
	StatusRejected = "rejected_privacy"
	StatusFailed   = "failed"
)

// Outbox row as returned by NextBatch.
type Row struct {
	ID            string
	ObservationID string
	Payload       []byte
	Attempts      int
	NextAttemptAt time.Time
	CreatedAt     time.Time
}

// BatchRequest is the wire payload posted to /v1/observations/batch.
type BatchRequest struct {
	ClientID     string `json:"client_id"`
	BatchID      string `json:"batch_id"`
	Schema       int    `json:"schema"`
	Observations []any  `json:"observations"` // each item is a cloud.Output
}

// BatchResponse mirrors the Hive server reply.
type BatchResponse struct {
	Accepted int      `json:"accepted"`
	Skipped  []string `json:"skipped,omitempty"`
}

// SearchResult is one item from a hybrid search call.
type SearchResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"` // "local" | "hive"
}

// Status counts returned by Outbox.Status.
type StatusCounts struct {
	Pending  int `json:"pending"`
	Sent     int `json:"sent"`
	Rejected int `json:"rejected_privacy"`
	Failed   int `json:"failed"`
}

// SyncPhase describes the current activity of the Hive worker.
type SyncPhase string

const (
	PhaseIdle     SyncPhase = "idle"     // worker is waiting for the next tick
	PhasePushing  SyncPhase = "pushing"  // actively sending a batch to Hive
	PhaseBackoff  SyncPhase = "backoff"  // waiting after a push failure
	PhaseError    SyncPhase = "error"    // unrecoverable error; worker stopped
	PhaseHealthy  SyncPhase = "healthy"  // last push succeeded
	PhaseDisabled SyncPhase = "disabled" // KORVA_HIVE_DISABLE=1
)

// WorkerStatus is a point-in-time snapshot of the Hive worker state.
// It is safe to read from any goroutine via Worker.Status().
type WorkerStatus struct {
	Phase             SyncPhase  `json:"phase"`
	LastSyncAt        *time.Time `json:"last_sync_at,omitempty"`
	ConsecutiveErrors int        `json:"consecutive_errors"`
	BackoffUntil      *time.Time `json:"backoff_until,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
	PendingCount      int        `json:"pending_count"`
}
