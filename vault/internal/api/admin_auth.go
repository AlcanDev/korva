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
// This allows every Korva developer with an admin-role session to access the
// admin panel of a shared vault (e.g. vault.korva.dev) without knowing the
// server's admin key.
func withAdminOrSessionAdmin(keyPath, keyOverride string, s *store.Store) func(http.Handler) http.Handler {
	rawAdminMW := admin.MiddlewareWithOverride(keyPath, keyOverride)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
