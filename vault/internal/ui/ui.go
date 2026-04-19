// Package ui provides access to the embedded Beacon dashboard (React SPA).
//
// The Beacon dist is embedded at build time when the "embedui" build tag is set.
// Normal development builds use the stub (embed_stub.go) which leaves DistFS nil,
// causing the vault to redirect to the external Vite dev server instead.
//
// To build a vault binary that bundles the UI:
//
//	make vault-full   (builds beacon, copies dist, then builds vault with -tags embedui)
package ui

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler returns an http.Handler that serves the Beacon SPA.
//
// Routing rules:
//   - Files that exist in dist/ are served directly (JS, CSS, images, favicon…)
//   - Everything else falls back to index.html so React Router handles the path
//   - If DistFS is nil (built without embedui tag) a 302 to devURL is returned instead
func Handler(devURL string) http.Handler {
	if DistFS == nil {
		// Not embedded — redirect to the standalone Beacon dev server.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, devURL, http.StatusTemporaryRedirect)
		})
	}

	// Sub into the dist/ directory so paths like /assets/... work directly.
	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		panic("vault/internal/ui: unexpected embed layout: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path to prevent directory traversal.
		p := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))

		// Try to open the exact file; if it doesn't exist, serve index.html
		// so the React Router can handle the path client-side.
		f, err := sub.Open(strings.TrimPrefix(p, "/"))
		if err != nil {
			serveIndex(w, r, sub)
			return
		}

		// Peek at whether it's a directory (e.g. "/assets/") — also serve index.
		stat, err := f.Stat()
		_ = f.(io.Closer).Close()
		if err != nil || stat.IsDir() {
			serveIndex(w, r, sub)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

// serveIndex writes the content of dist/index.html to the response.
func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	f, err := sub.Open("index.html")
	if err != nil {
		http.Error(w, "Beacon UI not available", http.StatusInternalServerError)
		return
	}
	defer f.(io.Closer).Close()

	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
}
