//go:build embedui

package ui

import "embed"

// DistFS holds the compiled Beacon SPA (dist/ directory).
// It is populated at build time when the embedui tag is provided.
//
// The dist/ directory must be present inside vault/internal/ui/ before
// building with this tag. The Makefile target `vault-full` handles this:
//
//	make vault-full   →  beacon npm build → cp beacon/dist vault/internal/ui/dist → go build -tags embedui
//
//go:embed all:dist
var DistFS embed.FS
