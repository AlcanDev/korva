// hive-mock is a stand-in Korva Hive server for local development and tests.
// In-memory only. Not for production use.
//
// Endpoints:
//
//	GET  /v1/health
//	POST /v1/observations/batch  (gzip JSON)
//	GET  /v1/search?q=...&limit=...
//
// Observations live in memory until the process exits. Run on :7438 by default.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type observation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	ProjectHash string    `json:"project_hash,omitempty"`
	TeamHash    string    `json:"team_hash,omitempty"`
	AuthorHash  string    `json:"author_hash,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type batchReq struct {
	ClientID     string        `json:"client_id"`
	BatchID      string        `json:"batch_id"`
	Schema       int           `json:"schema"`
	Observations []observation `json:"observations"`
}

type store struct {
	mu      sync.Mutex
	byID    map[string]observation
	byBatch map[string]bool
}

func newStore() *store {
	return &store{byID: make(map[string]observation), byBatch: make(map[string]bool)}
}

func (s *store) ingest(b batchReq) (accepted int, skipped []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byBatch[b.BatchID] {
		// Idempotent: a duplicate batch is a no-op success.
		return 0, nil
	}
	s.byBatch[b.BatchID] = true
	for _, o := range b.Observations {
		if _, exists := s.byID[o.ID]; exists {
			skipped = append(skipped, o.ID)
			continue
		}
		s.byID[o.ID] = o
		accepted++
	}
	return accepted, skipped
}

func (s *store) search(q string, limit int) []observation {
	s.mu.Lock()
	defer s.mu.Unlock()
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]observation, 0, limit)
	for _, o := range s.byID {
		if q == "" || strings.Contains(strings.ToLower(o.Title), q) || strings.Contains(strings.ToLower(o.Content), q) {
			out = append(out, o)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func main() {
	addr := flag.String("addr", ":7438", "listen address")
	flag.Parse()

	st := newStore()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	mux.HandleFunc("/v1/observations/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req batchReq
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		accepted, skipped := st.ingest(req)
		log.Printf("hive-mock: client=%s batch=%s accepted=%d skipped=%d", req.ClientID, req.BatchID, accepted, len(skipped))
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": accepted, "skipped": skipped})
	})
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		limit := 20
		fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
		results := st.search(q, limit)
		_ = json.NewEncoder(w).Encode(results)
	})

	log.Printf("hive-mock listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func decodeBody(r *http.Request) ([]byte, error) {
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}
	return io.ReadAll(r.Body)
}
