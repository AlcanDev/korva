package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// notifyWebhook fires a POST to webhookURL with the saved observation as JSON.
// It runs in a goroutine so it never blocks the save response.
// Errors are logged but never returned — webhooks are best-effort.
//
// Payload shape:
//
//	{
//	  "event":       "observation.created",
//	  "observation": { ...full Observation... },
//	  "ts":          "2026-04-19T12:34:56Z"
//	}
func notifyWebhook(webhookURL string, obs store.Observation) {
	if webhookURL == "" {
		return
	}
	go func() {
		payload, err := json.Marshal(map[string]any{
			"event":       "observation.created",
			"observation": obs,
			"ts":          time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
		if err != nil {
			log.Printf("webhook: build request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "korva-vault/"+version.Version)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("webhook: POST %s failed: %v", webhookURL, err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("webhook: POST %s returned %d", webhookURL, resp.StatusCode)
		}
	}()
}
