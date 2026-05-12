package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

// ingestInteractionRequest is the wire shape for POST /api/v1/interactions.
//
// Tokens fields are optional. When all four are zero the server falls back to
// an estimated count based on prompt+response character length and tags the row
// with `estimated=true`.
type ingestInteractionRequest struct {
	SessionID  string                 `json:"session_id"`
	Project    string                 `json:"project"`
	Team       string                 `json:"team"`
	Agent      string                 `json:"agent"`
	Model      string                 `json:"model"`
	Prompt     string                 `json:"prompt"`
	Response   string                 `json:"response"`
	Usage      *ingestInteractionUse  `json:"usage,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
	ToolCalls  json.RawMessage        `json:"tool_calls,omitempty"`
	Status     string                 `json:"status"`
	ErrorMsg   string                 `json:"error_msg"`
}

// ingestInteractionUse mirrors the `usage` object Anthropic returns on each
// /v1/messages response. Only the four token fields are persisted.
type ingestInteractionUse struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// ingestInteraction is the handler for POST /api/v1/interactions.
// Public endpoint — protected only by the global rate limiter and CORS.
func ingestInteraction(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestInteractionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Project == "" {
			writeError(w, http.StatusBadRequest, "project is required")
			return
		}
		if req.Agent == "" {
			writeError(w, http.StatusBadRequest, "agent is required")
			return
		}

		in := store.Interaction{
			SessionID:       req.SessionID,
			Project:         req.Project,
			Team:            req.Team,
			Agent:           req.Agent,
			Model:           req.Model,
			PromptExcerpt:   req.Prompt,
			ResponseExcerpt: req.Response,
			DurationMs:      req.DurationMs,
			ToolCalls:       req.ToolCalls,
			Status:          req.Status,
			ErrorMsg:        req.ErrorMsg,
		}
		if req.Usage != nil {
			in.InputTokens = req.Usage.InputTokens
			in.OutputTokens = req.Usage.OutputTokens
			in.CacheRead = req.Usage.CacheReadInputTokens
			in.CacheCreation = req.Usage.CacheCreationInputTokens
		}

		id, err := s.SaveInteraction(in)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Re-read to surface the canonical estimated flag in the response.
		saved, _ := s.GetInteraction(id)
		estimated := saved != nil && saved.Estimated

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":        id,
			"estimated": estimated,
		})
	}
}
