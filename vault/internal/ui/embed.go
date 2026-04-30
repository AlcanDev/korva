//go:build embedui

package ui

import (
	"embed"
	"io/fs"
)

// distFS holds the compiled Beacon SPA (dist/ directory).
// It is populated at build time when the embedui tag is provided.
//
// The dist/ directory must be present inside vault/internal/ui/ before
// building with this tag. The Makefile target `vault-full` handles this:
//
//	make vault-full   →  beacon npm build → cp beacon/dist vault/internal/ui/dist → go build -tags embedui
//
//go:embed all:dist
var distFS embed.FS

// DistFS exposes the embedded SPA via the fs.FS interface so callers
// can use the same type whether or not the embedui tag is set.
var DistFS fs.FS = distFS
