package api

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdirAll creates the parent directory of `fullPath`.
func mkdirAll(fullPath string) error {
	return os.MkdirAll(filepath.Dir(fullPath), 0o755)
}

// writeFile creates a file with the given content for fixture setups.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}
