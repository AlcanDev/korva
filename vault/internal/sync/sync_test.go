package sync

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func saveObs(t *testing.T, s *store.Store, title, content string) string {
	t.Helper()
	id, err := s.Save(store.Observation{
		Type:    "decision",
		Title:   title,
		Content: content,
		Project: "test-project",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Status — empty sync dir
// ---------------------------------------------------------------------------

func TestStatus_EmptySyncDir(t *testing.T) {
	dir := t.TempDir()
	sy := New(newTestStore(t), dir)

	m, err := sy.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if m.Version != syncVersion {
		t.Errorf("expected version %d, got %d", syncVersion, m.Version)
	}
	if m.TotalExported != 0 {
		t.Errorf("expected 0 total exported, got %d", m.TotalExported)
	}
	if m.LastID != "" {
		t.Errorf("expected empty LastID, got %q", m.LastID)
	}
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func TestExport_EmptyStore(t *testing.T) {
	dir := t.TempDir()
	sy := New(newTestStore(t), dir)

	n, err := sy.Export()
	if err != nil {
		t.Fatalf("Export on empty store: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 exported from empty store, got %d", n)
	}
}

func TestExport_SingleObservation(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	saveObs(t, s, "Decision: use hexagonal", "We chose hexagonal architecture.")

	sy := New(s, dir)
	n, err := sy.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 exported, got %d", n)
	}

	// manifest should be updated
	m, _ := sy.Status()
	if m.TotalExported != 1 {
		t.Errorf("manifest total_exported should be 1, got %d", m.TotalExported)
	}
	if m.LastID == "" {
		t.Error("manifest last_id should be set after export")
	}
	if m.LastExportedAt.IsZero() {
		t.Error("manifest last_exported_at should be set")
	}
}

func TestExport_MultipleObservations(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		saveObs(t, s, "obs", "content")
		time.Sleep(time.Millisecond) // ensure distinct ULIDs
	}

	sy := New(s, dir)
	n, err := sy.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 exported, got %d", n)
	}
}

func TestExport_Incremental(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	sy := New(s, dir)

	// First batch
	saveObs(t, s, "first", "content1")
	n1, err := sy.Export()
	if err != nil {
		t.Fatalf("Export #1: %v", err)
	}
	if n1 != 1 {
		t.Errorf("first export: expected 1, got %d", n1)
	}

	// Second batch — only new observations
	time.Sleep(time.Millisecond)
	saveObs(t, s, "second", "content2")
	saveObs(t, s, "third", "content3")
	n2, err := sy.Export()
	if err != nil {
		t.Fatalf("Export #2: %v", err)
	}
	if n2 != 2 {
		t.Errorf("second export: expected 2 new, got %d", n2)
	}

	m, _ := sy.Status()
	if m.TotalExported != 3 {
		t.Errorf("expected total 3, got %d", m.TotalExported)
	}
}

func TestExport_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	saveObs(t, s, "obs", "content")
	sy := New(s, dir)

	n1, _ := sy.Export()
	n2, _ := sy.Export() // nothing new since first export

	if n1 != 1 {
		t.Errorf("first export: expected 1, got %d", n1)
	}
	if n2 != 0 {
		t.Errorf("second export with no new obs: expected 0, got %d", n2)
	}
}

func TestExport_CreatesChunkFile(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	saveObs(t, s, "test", "content")

	sy := New(s, dir)
	_, err := sy.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Verify chunks directory exists with at least one .jsonl.gz file
	entries, err := os.ReadDir(dir + "/chunks")
	if err != nil {
		t.Fatalf("chunks dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one chunk file after export")
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl.gz") {
			t.Errorf("unexpected file in chunks dir: %q", e.Name())
		}
	}
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

func TestImport_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sy := New(newTestStore(t), dir)

	n, err := sy.Import()
	if err != nil {
		t.Fatalf("Import on empty dir: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 imported from empty dir, got %d", n)
	}
}

func TestImport_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Source store: 3 observations
	src := newTestStore(t)
	for i := 0; i < 3; i++ {
		saveObs(t, src, "obs", "content")
		time.Sleep(time.Millisecond)
	}
	sy1 := New(src, dir)
	exported, err := sy1.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if exported != 3 {
		t.Fatalf("expected 3 exported, got %d", exported)
	}

	// Destination store: import into a fresh DB
	dst := newTestStore(t)
	sy2 := New(dst, dir)
	imported, err := sy2.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if imported != 3 {
		t.Errorf("expected 3 imported, got %d", imported)
	}

	// Verify observations are in the destination store
	results, err := dst.Search("", store.SearchFilters{Limit: 100})
	if err != nil {
		t.Fatalf("Search after import: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 observations in dst, got %d", len(results))
	}
}

func TestImport_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Export from source
	src := newTestStore(t)
	saveObs(t, src, "obs", "content")
	sy1 := New(src, dir)
	sy1.Export()

	// Import twice into the same destination
	dst := newTestStore(t)
	sy2 := New(dst, dir)
	n1, err := sy2.Import()
	if err != nil {
		t.Fatalf("Import #1: %v", err)
	}
	n2, err := sy2.Import()
	if err != nil {
		t.Fatalf("Import #2: %v", err)
	}

	if n1 != 1 {
		t.Errorf("first import: expected 1, got %d", n1)
	}
	if n2 != 0 {
		t.Errorf("second import (idempotent): expected 0, got %d", n2)
	}
}

func TestImport_UpdatesManifestTimestamp(t *testing.T) {
	dir := t.TempDir()

	src := newTestStore(t)
	saveObs(t, src, "obs", "content")
	sy1 := New(src, dir)
	sy1.Export()

	dst := newTestStore(t)
	sy2 := New(dst, dir)
	sy2.Import()

	m, _ := sy2.Status()
	if m.LastImportedAt.IsZero() {
		t.Error("last_imported_at should be set after import")
	}
}
