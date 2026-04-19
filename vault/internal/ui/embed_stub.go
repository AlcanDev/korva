//go:build !embedui

package ui

import "io/fs"

// DistFS is nil when the binary is built without the embedui tag.
// The vault's UI handler will redirect to the Beacon dev server instead.
var DistFS fs.FS
