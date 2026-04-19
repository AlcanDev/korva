package hive

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client posts batches to the Korva Hive HTTP API.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// NewClient builds a Hive client. endpoint should be the base URL
// (e.g. "https://hive.korva.dev"); apiKey is the per-installation token
// stored in ~/.korva/hive.key.
func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Health probes the server. Used by Worker as an online check.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/v1/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Hive-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hive health: status %d", resp.StatusCode)
	}
	return nil
}

// PostBatch sends a batch of anonymized observations.
// The body is gzipped JSON. Returns an error if the server rejects it.
func (c *Client) PostBatch(ctx context.Context, batch BatchRequest) (BatchResponse, error) {
	var resp BatchResponse

	raw, err := json.Marshal(batch)
	if err != nil {
		return resp, fmt.Errorf("hive: marshal batch: %w", err)
	}
	if len(raw) > 1<<20 {
		return resp, errors.New("hive: batch exceeds 1 MiB cap")
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return resp, fmt.Errorf("hive: gzip: %w", err)
	}
	zw.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/observations/batch", &buf)
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-Hive-Key", c.apiKey)

	httpResp, err := c.http.Do(req)
	if err != nil {
		return resp, err
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode >= 400 {
		return resp, fmt.Errorf("hive batch: status %d: %s", httpResp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return resp, fmt.Errorf("hive: parse response: %w", err)
	}
	return resp, nil
}

// Search queries the cloud brain. Used by hybrid search.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/v1/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Hive-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hive search: status %d: %s", resp.StatusCode, string(body))
	}
	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Source = "hive"
	}
	return results, nil
}
