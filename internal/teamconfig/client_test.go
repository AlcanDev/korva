package teamconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ─── Client ──────────────────────────────────────────────────────────────────

func bundleServer(t *testing.T, bundle *Bundle, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/team/config/bundle" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-License-Key") == "" {
			http.Error(w, `{"error":"missing key"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if bundle != nil {
			json.NewEncoder(w).Encode(bundle) //nolint:errcheck
		}
	}))
}

func sampleBundle() *Bundle {
	return &Bundle{
		LicenseID: "lic_test001",
		Tier:      "teams",
		Version:   "2026-05-15T10:00:00Z",
		Items: []BundleItem{
			{Section: "scrolls", Name: "api-patterns.md", Content: "# API Patterns\nContent.", Version: 1, Hash: "abc123", UpdatedAt: "2026-05-15T10:00:00Z"},
			{Section: "rules", Name: "no-secrets.md", Content: "# No secrets rule", Version: 2, Hash: "def456", UpdatedAt: "2026-05-15T09:00:00Z"},
		},
	}
}

func TestClient_DownloadBundle_Success(t *testing.T) {
	srv := bundleServer(t, sampleBundle(), http.StatusOK)
	defer srv.Close()

	c := New(srv.URL, "KORVA-TEST-KEY-0001")
	bundle, err := c.DownloadBundle(context.Background())
	if err != nil {
		t.Fatalf("DownloadBundle: %v", err)
	}
	if bundle.LicenseID != "lic_test001" {
		t.Errorf("LicenseID: got %q, want lic_test001", bundle.LicenseID)
	}
	if len(bundle.Items) != 2 {
		t.Errorf("Items: got %d, want 2", len(bundle.Items))
	}
}

func TestClient_DownloadBundle_NoKey(t *testing.T) {
	c := New("http://localhost:9999", "")
	_, err := c.DownloadBundle(context.Background())
	if err != ErrNoKey {
		t.Fatalf("empty key: want ErrNoKey, got %v", err)
	}
}

func TestClient_DownloadBundle_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "KORVA-BAD-KEY-0000")
	_, err := c.DownloadBundle(context.Background())
	if err != ErrUnauthorized {
		t.Fatalf("bad key: want ErrUnauthorized, got %v", err)
	}
}

func TestClient_DownloadBundle_NotEnabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not enabled"}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New(srv.URL, "KORVA-TEST-KEY-0002")
	_, err := c.DownloadBundle(context.Background())
	if err != ErrNotEnabled {
		t.Fatalf("not enabled: want ErrNotEnabled, got %v", err)
	}
}

func TestClient_DownloadBundle_ContextCanceled(t *testing.T) {
	srv := bundleServer(t, sampleBundle(), http.StatusOK)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	c := New(srv.URL, "KORVA-TEST-KEY-0003")
	_, err := c.DownloadBundle(ctx)
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}

// ─── Store: WriteBundleToDisk ─────────────────────────────────────────────────

func TestWriteBundleToDisk_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	bundle := sampleBundle()

	result, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("WriteBundleToDisk: %v", err)
	}
	if result.Written != 2 {
		t.Errorf("Written: got %d, want 2", result.Written)
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped: got %d, want 0", result.Skipped)
	}

	checkFile(t, filepath.Join(dir, "scrolls", "api-patterns.md"), "# API Patterns\nContent.")
	checkFile(t, filepath.Join(dir, "rules", "no-secrets.md"), "# No secrets rule")
}

func TestWriteBundleToDisk_SkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	bundle := sampleBundle()

	// First write
	r1, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if r1.Written != 2 {
		t.Errorf("first write: Written=%d, want 2", r1.Written)
	}

	// Second write — same bundle, should skip all
	r2, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if r2.Written != 0 {
		t.Errorf("second write: Written=%d, want 0 (all skipped)", r2.Written)
	}
	if r2.Skipped != 2 {
		t.Errorf("second write: Skipped=%d, want 2", r2.Skipped)
	}
}

func TestWriteBundleToDisk_UpdatesChangedFile(t *testing.T) {
	dir := t.TempDir()
	bundle := sampleBundle()
	WriteBundleToDisk(dir, bundle) //nolint:errcheck

	// Change one item's content and hash
	bundle.Items[0].Content = "# Updated API Patterns"
	bundle.Items[0].Hash = "newhash999"

	r2, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("update write: %v", err)
	}
	if r2.Written != 1 {
		t.Errorf("update: Written=%d, want 1", r2.Written)
	}
	checkFile(t, filepath.Join(dir, "scrolls", "api-patterns.md"), "# Updated API Patterns")
}

func TestWriteBundleToDisk_PrunesRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	bundle := sampleBundle()
	WriteBundleToDisk(dir, bundle) //nolint:errcheck

	// Remove one item from bundle
	bundle.Items = bundle.Items[:1] // only keep the scroll
	r2, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("prune write: %v", err)
	}
	if r2.Deleted != 1 {
		t.Errorf("prune: Deleted=%d, want 1", r2.Deleted)
	}
	// The rule file must be gone
	if _, err := os.Stat(filepath.Join(dir, "rules", "no-secrets.md")); !os.IsNotExist(err) {
		t.Error("pruned file must not exist")
	}
}

func TestWriteBundleToDisk_EmptyBundle(t *testing.T) {
	dir := t.TempDir()
	bundle := &Bundle{LicenseID: "lic_x", Tier: "teams", Items: []BundleItem{}}
	result, err := WriteBundleToDisk(dir, bundle)
	if err != nil {
		t.Fatalf("empty bundle: %v", err)
	}
	if result.Written != 0 || result.Skipped != 0 {
		t.Errorf("empty bundle must write/skip 0 items: %+v", result)
	}
}

// ─── Store: SyncState ────────────────────────────────────────────────────────

func TestSyncState_RoundTrip(t *testing.T) {
	f := filepath.Join(t.TempDir(), "sync.json")
	want := SyncState{
		SyncedAt:      time.Now().UTC().Truncate(time.Second),
		BundleVersion: "2026-05-15T10:00:00Z",
		LicenseID:     "lic_abc",
		ItemCount:     5,
	}
	if err := SaveSyncState(f, want); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}
	got, err := LoadSyncState(f)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}
	if got.LicenseID != want.LicenseID || got.ItemCount != want.ItemCount {
		t.Errorf("state mismatch: got %+v, want %+v", got, want)
	}
}

func TestLoadSyncState_MissingFile(t *testing.T) {
	state, err := LoadSyncState("/tmp/korva-test-nonexistent-state.json")
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if !state.SyncedAt.IsZero() {
		t.Error("missing file must return zero-value state")
	}
}

// ─── Store: TeamKey ──────────────────────────────────────────────────────────

func TestTeamKey_RoundTrip(t *testing.T) {
	f := filepath.Join(t.TempDir(), "team.key")
	key := "KORVA-ABCD-EFGH-IJKL-MNOP"
	if err := SaveTeamKey(f, key); err != nil {
		t.Fatalf("SaveTeamKey: %v", err)
	}
	// Check file permissions (Windows does not enforce Unix mode bits)
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(f)
		if info.Mode().Perm() != 0600 {
			t.Errorf("team.key must be 0600, got %v", info.Mode().Perm())
		}
	}
	got, err := LoadTeamKey(f)
	if err != nil {
		t.Fatalf("LoadTeamKey: %v", err)
	}
	if got != key {
		t.Errorf("key mismatch: got %q, want %q", got, key)
	}
}

func TestLoadTeamKey_MissingFile(t *testing.T) {
	key, err := LoadTeamKey("/tmp/korva-nonexistent-team.key")
	if err != nil {
		t.Fatalf("missing key file must not error: %v", err)
	}
	if key != "" {
		t.Errorf("missing key file must return empty string, got %q", key)
	}
}

// ─── Path validation ─────────────────────────────────────────────────────────

func TestIsSafeSegment(t *testing.T) {
	cases := []struct {
		s     string
		valid bool
	}{
		{"scrolls", true},
		{"api-patterns", true},
		{"file_name", true},
		{"", false},
		{".", false},
		{"..", false},
		{"path/traversal", false}, // slashes not allowed in segment
		{"../escape", false},
	}
	for _, tc := range cases {
		if got := isSafeSegment(tc.s); got != tc.valid {
			t.Errorf("isSafeSegment(%q) = %v, want %v", tc.s, got, tc.valid)
		}
	}
}

func TestIsSafePath(t *testing.T) {
	cases := []struct {
		s     string
		valid bool
	}{
		{"api-patterns.md", true},
		{"file.md", true},
		{"", false},
		{"../evil", false},
		{"path/../escape", false},
	}
	for _, tc := range cases {
		if got := isSafePath(tc.s); got != tc.valid {
			t.Errorf("isSafePath(%q) = %v, want %v", tc.s, got, tc.valid)
		}
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func checkFile(t *testing.T, path, wantContent string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if string(data) != wantContent {
		t.Errorf("file %s:\n got  %q\n want %q", path, data, wantContent)
	}
}
