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
