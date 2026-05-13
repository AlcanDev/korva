package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 10.3 — Session replay endpoint.
//
//   GET /admin/sessions/{id}/replay
//
// Reconstructs a chronological timeline for one session: the session
// boundaries (start + end), every observation saved during it, every
// interaction logged. Drives the dashboard's step-by-step replay UI.
//
// We sort everything by timestamp so the UI can render a clean vertical
// timeline; per-row kind tells the renderer which icon/colour to use.

// ReplayEntryKind classifies the timeline row.
type ReplayEntryKind string

const (
	ReplayKindSessionStart ReplayEntryKind = "session_start"
	ReplayKindSessionEnd   ReplayEntryKind = "session_end"
	ReplayKindObservation  ReplayEntryKind = "observation"
	ReplayKindInteraction  ReplayEntryKind = "interaction"
)

// ReplayEntry is one row in the timeline.
type ReplayEntry struct {
	Kind        ReplayEntryKind `json:"kind"`
	At          time.Time       `json:"at"`
	Title       string          `json:"title"`
	Project     string          `json:"project,omitempty"`
	Author      string          `json:"author,omitempty"`
	ObsType     string          `json:"obs_type,omitempty"`
	Model       string          `json:"model,omitempty"`
	Tokens      int64           `json:"tokens,omitempty"`
	DurationMs  int64           `json:"duration_ms,omitempty"`
	ID          string          `json:"id,omitempty"`
	ExcerptIn   string          `json:"excerpt_in,omitempty"`
	ExcerptOut  string          `json:"excerpt_out,omitempty"`
	Body        string          `json:"body,omitempty"`
}

// ReplayResponse is the wire shape of /admin/sessions/{id}/replay.
type ReplayResponse struct {
	SessionID string        `json:"session_id"`
	Project   string        `json:"project,omitempty"`
	Agent     string        `json:"agent,omitempty"`
	Goal      string        `json:"goal,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Entries   []ReplayEntry `json:"entries"`
	Total     int           `json:"total"`
}

func adminSessionReplay(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "session id is required")
			return
		}

		sess, err := s.GetSession(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sess == nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}

		entries := make([]ReplayEntry, 0, 16)
		entries = append(entries, ReplayEntry{
			Kind:    ReplayKindSessionStart,
			At:      sess.StartedAt,
			Title:   "Session started",
			Project: sess.Project,
			Author:  sess.Agent,
			Body:    sess.Goal,
		})

		// Observations saved during the session.
		obs, err := s.ListObservationsBySession(id)
		if err == nil {
			for _, o := range obs {
				entries = append(entries, ReplayEntry{
					Kind:    ReplayKindObservation,
					At:      o.CreatedAt,
					Title:   o.Title,
					Project: o.Project,
					Author:  o.Author,
					ObsType: string(o.Type),
					ID:      o.ID,
					Body:    o.Content,
				})
			}
		}

		// Interactions logged during the session.
		inter, err := s.ListInteractionsBySession(id)
		if err == nil {
			for _, in := range inter {
				entries = append(entries, ReplayEntry{
					Kind:       ReplayKindInteraction,
					At:         in.CreatedAt,
					Title:      truncateForReplay(in.PromptExcerpt, 80),
					Project:    in.Project,
					Author:     in.Agent,
					Model:      in.Model,
					Tokens:     in.InputTokens + in.OutputTokens,
					DurationMs: in.DurationMs,
					ID:         in.ID,
					ExcerptIn:  in.PromptExcerpt,
					ExcerptOut: in.ResponseExcerpt,
				})
			}
		}

		if sess.EndedAt != nil && !sess.EndedAt.IsZero() {
			entries = append(entries, ReplayEntry{
				Kind:    ReplayKindSessionEnd,
				At:      *sess.EndedAt,
				Title:   "Session ended",
				Project: sess.Project,
				Author:  sess.Agent,
				Body:    sess.Summary,
			})
		}

		// Strict chronological order. Ties (same timestamp) keep the
		// insertion order so session_start sits before any obs that lands
		// at the exact same instant.
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].At.Before(entries[j].At)
		})

		var endedAt *time.Time
		if sess.EndedAt != nil && !sess.EndedAt.IsZero() {
			endedAt = sess.EndedAt
		}
		writeJSON(w, http.StatusOK, ReplayResponse{
			SessionID: id,
			Project:   sess.Project,
			Agent:     sess.Agent,
			Goal:      sess.Goal,
			StartedAt: sess.StartedAt,
			EndedAt:   endedAt,
			Entries:   entries,
			Total:     len(entries),
		})
	}
}

func truncateForReplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
