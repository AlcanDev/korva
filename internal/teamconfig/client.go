// Package teamconfig downloads and stores team configuration from the Korva
// cloud licensing server (licensing.korva.dev/v1/team/config/bundle).
//
// Authentication uses the raw license key stored in ~/.korva/team.key.
// The server encrypts all content per team; the client receives plaintext.
package teamconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBundleTimeout = 30 * time.Second

// BundleItem is a single config file returned by the bundle endpoint.
type BundleItem struct {
	Section   string `json:"section"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Version   int    `json:"version"`
	Hash      string `json:"hash"`
	UpdatedAt string `json:"updated_at"`
}

// Bundle is the full response from GET /v1/team/config/bundle.
type Bundle struct {
	LicenseID string       `json:"license_id"`
	Tier      string       `json:"tier"`
	Version   string       `json:"version"` // latest updated_at across all items
	Items     []BundleItem `json:"items"`
}

// Client is an HTTP client for the team config API.
type Client struct {
	baseURL    string
	licenseKey string
	httpClient *http.Client
}

// New creates a Client.
//
//   - baseURL: e.g. "https://licensing.korva.dev"
//   - licenseKey: raw license key (KORVA-XXXX-...) stored in ~/.korva/team.key
func New(baseURL, licenseKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		licenseKey: licenseKey,
		httpClient: &http.Client{Timeout: defaultBundleTimeout},
	}
}

// DownloadBundle fetches all team config items from the server.
// Returns ErrNoKey when licenseKey is empty.
// Returns ErrUnauthorized when the server rejects the key.
// Returns ErrNotEnabled when team config is not enabled on the server.
func (c *Client) DownloadBundle(ctx context.Context) (*Bundle, error) {
	if c.licenseKey == "" {
		return nil, ErrNoKey
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/team/config/bundle", nil)
	if err != nil {
		return nil, fmt.Errorf("teamconfig: build request: %w", err)
	}
	req.Header.Set("X-License-Key", c.licenseKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teamconfig: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 32 MiB max
	if err != nil {
		return nil, fmt.Errorf("teamconfig: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusPaymentRequired:
		return nil, ErrExpired
	case http.StatusServiceUnavailable:
		return nil, ErrNotEnabled
	default:
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("teamconfig: server error: %s (HTTP %d)", errResp.Error, resp.StatusCode)
		}
		return nil, fmt.Errorf("teamconfig: unexpected status %d", resp.StatusCode)
	}

	var bundle Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("teamconfig: parse bundle: %w", err)
	}
	return &bundle, nil
}

// ─── Errors ──────────────────────────────────────────────────────────────────

// Sentinel errors — callers can use errors.Is() for typed checks.
var (
	ErrNoKey       = fmt.Errorf("teamconfig: no license key stored (run 'korva connect --key KORVA-...')")
	ErrUnauthorized = fmt.Errorf("teamconfig: license key rejected — check key or run 'korva connect'")
	ErrExpired     = fmt.Errorf("teamconfig: license expired — please renew")
	ErrNotEnabled  = fmt.Errorf("teamconfig: team config is not enabled on this Korva server")
)
