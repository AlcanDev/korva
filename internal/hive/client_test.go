package hive

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com", "key123")
	if c.endpoint != "https://example.com" {
		t.Errorf("endpoint = %q", c.endpoint)
	}
	if c.apiKey != "key123" {
		t.Errorf("apiKey = %q", c.apiKey)
	}
	if c.http == nil {
		t.Error("http client is nil")
	}
}

func TestHealth_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			http.NotFound(w, r)
			return
		}
		if key := r.Header.Get("X-Hive-Key"); key != "testkey" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "testkey")
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestHealth_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestHealth_Unreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "k") // nothing listening
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so it fails fast
	if err := c.Health(ctx); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestPostBatch_Success(t *testing.T) {
	var received BatchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/observations/batch" || r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body, _ := io.ReadAll(gz)
		json.Unmarshal(body, &received)

		json.NewEncoder(w).Encode(BatchResponse{Accepted: 1})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	batch := BatchRequest{
		ClientID:     "client-1",
		BatchID:      "batch-1",
		Schema:       1,
		Observations: []any{map[string]string{"type": "pattern"}},
	}
	resp, err := c.PostBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("PostBatch: %v", err)
	}
	if resp.Accepted != 1 {
		t.Errorf("accepted = %d, want 1", resp.Accepted)
	}
	if received.ClientID != "client-1" {
		t.Errorf("received client_id = %q", received.ClientID)
	}
}

func TestPostBatch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.PostBatch(context.Background(), BatchRequest{Schema: 1, Observations: []any{}})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}

func TestSearch_Success(t *testing.T) {
	results := []SearchResult{
		{ID: "obs-1", Type: "pattern", Title: "T1", Content: "C1"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(results)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.Search(context.Background(), "pattern", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Source != "hive" {
		t.Errorf("source = %q, want 'hive'", got[0].Source)
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "20" {
			http.Error(w, "wrong limit", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode([]SearchResult{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	// limit=0 should default to 20
	if _, err := c.Search(context.Background(), "q", 0); err != nil {
		t.Fatalf("Search default limit: %v", err)
	}
}

func TestSearch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.Search(context.Background(), "q", 5); err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}
