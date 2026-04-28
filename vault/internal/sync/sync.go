// Package sync provides Git-based export and import of Vault observations.
//
// Format:
//
//	.korva-sync/
//	├── manifest.json           ← metadata: version, last export, ULID watermark
//	└── chunks/
//	    ├── 2024-01.jsonl.gz    ← observations for that month (newline-delimited JSON, gzipped)
//	    └── 2024-02.jsonl.gz
//
// Design:
//   - Export appends only new observations (ULID-ordered, incremental)
//   - Import is idempotent: duplicate IDs are silently skipped
//   - Each chunk contains one observation per line (JSONL) for easy diffing
//   - Private patterns are applied BEFORE export (privacy filter)
//   - admin.key and local DB are NEVER written to the sync directory
package sync

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

const (
	manifestFile = "manifest.json"
	chunksDir    = "chunks"
	syncVersion  = 1
)

// Manifest tracks the sync state.
type Manifest struct {
	Version        int       `json:"version"`
	LastExportedAt time.Time `json:"last_exported_at"`
	LastImportedAt time.Time `json:"last_imported_at,omitempty"`
	// LastID is the ULID of the most recently exported observation.
	// Used as a watermark for incremental exports.
	LastID string `json:"last_id,omitempty"`
	// TotalExported is the cumulative count exported to this directory.
	TotalExported int `json:"total_exported"`
}

// Syncer manages export and import between a Store and a sync directory.
type Syncer struct {
	store   *store.Store
	syncDir string
}

// New creates a Syncer that writes to syncDir.
func New(s *store.Store, syncDir string) *Syncer {
	return &Syncer{store: s, syncDir: syncDir}
}

// Export writes observations added since the last export into monthly chunks.
// Returns the number of newly exported observations.
func (sy *Syncer) Export() (int, error) {
	if err := os.MkdirAll(filepath.Join(sy.syncDir, chunksDir), 0750); err != nil {
		return 0, fmt.Errorf("creating sync chunks dir: %w", err)
	}

	manifest, err := sy.loadManifest()
	if err != nil {
		return 0, err
	}

	// Fetch all observations newer than the last exported ID
	observations, err := sy.store.Search("", store.SearchFilters{Limit: 10000})
	if err != nil {
		return 0, fmt.Errorf("fetching observations for export: %w", err)
	}

	// Filter to only observations after the watermark
	var toExport []store.Observation
	for _, obs := range observations {
		if manifest.LastID == "" || obs.ID > manifest.LastID {
			toExport = append(toExport, obs)
		}
	}

	if len(toExport) == 0 {
		return 0, nil
	}

	// Sort by ID (ULID = time-ordered)
	sort.Slice(toExport, func(i, j int) bool {
		return toExport[i].ID < toExport[j].ID
	})

	// Group by month
	byMonth := make(map[string][]store.Observation)
	for _, obs := range toExport {
		month := obs.CreatedAt.UTC().Format("2006-01")
		byMonth[month] = append(byMonth[month], obs)
	}

	written := 0
	for month, obs := range byMonth {
		n, err := sy.appendToChunk(month, obs)
		if err != nil {
			return written, fmt.Errorf("writing chunk %s: %w", month, err)
		}
		written += n
	}

	// Update manifest
	manifest.LastExportedAt = time.Now().UTC()
	manifest.LastID = toExport[len(toExport)-1].ID
	manifest.TotalExported += written

	return written, sy.saveManifest(manifest)
}

// Import reads all chunks from syncDir and inserts observations into the store.
// Duplicate IDs are silently skipped (idempotent).
// Returns the number of newly imported observations.
func (sy *Syncer) Import() (int, error) {
	chunksPath := filepath.Join(sy.syncDir, chunksDir)
	entries, err := os.ReadDir(chunksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading chunks dir: %w", err)
	}

	manifest, err := sy.loadManifest()
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl.gz") {
			continue
		}

		chunkPath := filepath.Join(chunksPath, entry.Name())
		n, err := sy.importChunk(chunkPath)
		if err != nil {
			return imported, fmt.Errorf("importing chunk %s: %w", entry.Name(), err)
		}
		imported += n
	}

	manifest.LastImportedAt = time.Now().UTC()
	return imported, sy.saveManifest(manifest)
}

// Status returns the current manifest without modifying anything.
func (sy *Syncer) Status() (*Manifest, error) {
	return sy.loadManifest()
}

// --- internal ---

func (sy *Syncer) appendToChunk(month string, observations []store.Observation) (int, error) {
	chunkPath := filepath.Join(sy.syncDir, chunksDir, month+".jsonl.gz")

	// Read existing IDs in this chunk to avoid duplicates on re-export
	existing := make(map[string]bool)
	if _, err := os.Stat(chunkPath); err == nil {
		if err := readChunkIDs(chunkPath, existing); err != nil {
			return 0, err
		}
	}

	// Open for append (create if not exists)
	f, err := os.OpenFile(chunkPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return 0, fmt.Errorf("opening chunk file: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	written := 0
	for _, obs := range observations {
		if existing[obs.ID] {
			continue
		}
		line, err := json.Marshal(obs)
		if err != nil {
			return written, fmt.Errorf("marshaling observation %s: %w", obs.ID, err)
		}
		if _, err := gz.Write(append(line, '\n')); err != nil {
			return written, err
		}
		written++
	}

	return written, nil
}

func (sy *Syncer) importChunk(chunkPath string) (int, error) {
	f, err := os.Open(chunkPath)
	if err != nil {
		return 0, fmt.Errorf("opening chunk: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	imported := 0
	dec := json.NewDecoder(gz)
	for dec.More() {
		var obs store.Observation
		if err := dec.Decode(&obs); err != nil {
			return imported, fmt.Errorf("decoding observation: %w", err)
		}

		// Skip if already exists
		existing, _ := sy.store.Get(obs.ID)
		if existing != nil {
			continue
		}

		if _, err := sy.store.Save(obs); err != nil {
			return imported, fmt.Errorf("saving observation %s: %w", obs.ID, err)
		}
		imported++
	}

	return imported, nil
}

func readChunkIDs(chunkPath string, ids map[string]bool) error {
	f, err := os.Open(chunkPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	dec := json.NewDecoder(gz)
	for dec.More() {
		var obs struct {
			ID string `json:"id"`
		}
		if err := dec.Decode(&obs); err != nil {
			return err
		}
		ids[obs.ID] = true
	}
	return nil
}

func (sy *Syncer) manifestPath() string {
	return filepath.Join(sy.syncDir, manifestFile)
}

func (sy *Syncer) loadManifest() (*Manifest, error) {
	data, err := os.ReadFile(sy.manifestPath())
	if os.IsNotExist(err) {
		return &Manifest{Version: syncVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

func (sy *Syncer) saveManifest(m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sy.manifestPath(), data, 0640)
}
