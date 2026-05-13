package api

import (
	"context"
	"net/http"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/vault/internal/store"
)

// withAdminOrSessionAdmin returns middleware that accepts either:
//   - X-Admin-Key  validated against the server's admin.key file, or
//   - X-Session-Token belonging to a team member with role=admin.
//
// Phase 8.5: also accepts `?admin_key=…` / `?session_token=…` query params
// as a fallback for clients that can't set headers — notably the browser's
// native EventSource API used for the /admin/events SSE stream. Headers are
// always tried first; query params are only consulted when no header is set,
// and the value is promoted onto the request header so the rest of the
// pipeline sees a consistent shape.
//
// This allows every Korva developer with an admin-role session to access the
// admin panel of a shared vault (e.g. vault.korva.dev) without knowing the
// server's admin key.
func withAdminOrSessionAdmin(keyPath, keyOverride string, s *store.Store) func(http.Handler) http.Handler {
	rawAdminMW := admin.MiddlewareWithOverride(keyPath, keyOverride)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Promote query-param credentials onto the headers so downstream
			// handlers and the admin middleware only need to inspect headers.
			if r.Header.Get("X-Admin-Key") == "" {
				if v := r.URL.Query().Get("admin_key"); v != "" {
					r.Header.Set("X-Admin-Key", v)
				}
			}
			if r.Header.Get("X-Session-Token") == "" {
				if v := r.URL.Query().Get("session_token"); v != "" {
					r.Header.Set("X-Session-Token", v)
				}
			}

			hasKey := r.Header.Get("X-Admin-Key") != ""
			hasToken := r.Header.Get("X-Session-Token") != ""

			if !hasKey && !hasToken {
				writeError(w, http.StatusUnauthorized,
					"authentication required: provide X-Admin-Key or X-Session-Token (role=admin)")
				return
			}

			if hasKey {
				rawAdminMW(next).ServeHTTP(w, r)
				return
			}

			// Session token path: validate session and assert admin role.
			sess, ok := requireSession(s, w, r)
			if !ok {
				return
			}
			if !requireAdmin(sess, w) {
				return
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
