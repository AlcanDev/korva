// Package api implements the Vault HTTP REST API on port 7437.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/vault/internal/store"
)

// Router builds the HTTP mux for the Vault API.
func Router(s *store.Store, adminKeyPath string) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /healthz", healthz)

	// Observations
	mux.HandleFunc("POST /api/v1/observations", withCORS(saveObservation(s)))
	mux.HandleFunc("GET /api/v1/observations/{id}", withCORS(getObservation(s)))
	mux.HandleFunc("GET /api/v1/search", withCORS(searchObservations(s)))
	mux.HandleFunc("GET /api/v1/context/{project}", withCORS(contextObservations(s)))
	mux.HandleFunc("GET /api/v1/timeline/{project}", withCORS(timeline(s)))

	// Sessions
	mux.HandleFunc("POST /api/v1/sessions", withCORS(startSession(s)))
	mux.HandleFunc("PUT /api/v1/sessions/{id}", withCORS(endSession(s)))

	// Summary and stats
	mux.HandleFunc("GET /api/v1/summary/{project}", withCORS(summary(s)))
	mux.HandleFunc("GET /api/v1/stats", withCORS(stats(s)))

	// Prompts
	mux.HandleFunc("POST /api/v1/prompts", withCORS(savePrompt(s)))

	// Sessions — all (admin-level listing)
	mux.HandleFunc("GET /api/v1/sessions/all", withCORS(listAllSessions(s)))

	// Admin endpoints — protected by X-Admin-Key
	adminMW := admin.Middleware(adminKeyPath)
	mux.Handle("POST /admin/purge", adminMW(withCORS(adminPurge(s))))
	mux.Handle("DELETE /admin/observations/{id}", adminMW(withCORS(adminDeleteObservation(s))))
	mux.Handle("GET /admin/stats", adminMW(withCORS(adminFullStats(s))))

	return mux
}

// --- handlers ---

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "korva-vault"})
}

func saveObservation(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var obs store.Observation
		if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := s.Save(obs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func getObservation(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		obs, err := s.Get(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if obs == nil {
			writeError(w, http.StatusNotFound, "observation not found")
			return
		}
		writeJSON(w, http.StatusOK, obs)
	}
}

func searchObservations(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		results, err := s.Search(q.Get("q"), store.SearchFilters{
			Project: q.Get("project"),
			Team:    q.Get("team"),
			Country: q.Get("country"),
			Type:    store.ObservationType(q.Get("type")),
			Limit:   20,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
	}
}

func contextObservations(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		results, err := s.Context(project, nil, 10)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"context": results, "project": project})
	}
}

func timeline(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		q := r.URL.Query()

		from := time.Now().Add(-7 * 24 * time.Hour)
		to := time.Now()

		if fromStr := q.Get("from"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				from = t
			}
		}
		if toStr := q.Get("to"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				to = t
			}
		}

		results, err := s.Timeline(project, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"timeline": results, "project": project})
	}
}

func startSession(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Project string `json:"project"`
			Team    string `json:"team"`
			Country string `json:"country"`
			Agent   string `json:"agent"`
			Goal    string `json:"goal"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		id, err := s.SessionStart(body.Project, body.Team, body.Country, body.Agent, body.Goal)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"session_id": id})
	}
}

func endSession(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Summary string `json:"summary"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if err := s.SessionEnd(id, body.Summary); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
	}
}

func summary(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		result, err := s.Summary(project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func stats(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.Stats()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func savePrompt(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name    string   `json:"name"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := s.SavePrompt(body.Name, body.Content, body.Tags); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "saved"})
	}
}

func listAllSessions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions, err := s.ListSessions(100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

// --- admin handlers ---

func adminPurge(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Intentionally minimal — purge is destructive
		writeJSON(w, http.StatusOK, map[string]string{"status": "purge not implemented in v1"})
	}
}

func adminDeleteObservation(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "delete not implemented in v1"})
	}
}

func adminFullStats(s *store.Store) http.HandlerFunc {
	return stats(s)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}
