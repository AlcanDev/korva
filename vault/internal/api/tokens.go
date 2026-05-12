package api

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

// repoBaselineEnvVar lets the operator override the directory used to estimate
// the "without Korva" baseline token cost. Defaults to CWD when unset.
const repoBaselineEnvVar = "KORVA_BASELINE_DIR"

// adminTokenStats handles GET /admin/tokens/stats — aggregated token usage with
// an estimated baseline so the UI can compute a reduction percentage.
func adminTokenStats(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from, _ := parseTime(q.Get("from"))
		to, _ := parseTime(q.Get("to"))

		stats, err := s.GetTokenStats(from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		baselineDir := os.Getenv(repoBaselineEnvVar)
		if baselineDir == "" {
			baselineDir, _ = os.Getwd()
		}
		baselineTokens := estimateBaselineTokens(baselineDir)

		// Reduction = 1 - (input_tokens we sent / baseline naive tokens).
		// Baseline of 0 disables the metric (avoid divide-by-zero).
		var reduction float64
		if baselineTokens > 0 && stats.InputTokens >= 0 {
			ratio := float64(stats.InputTokens) / float64(baselineTokens)
			if ratio > 1 {
				ratio = 1
			}
			reduction = 1 - ratio
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"totals": map[string]any{
				"input_tokens":       stats.InputTokens,
				"output_tokens":      stats.OutputTokens,
				"cache_read":         stats.CacheRead,
				"cache_creation":     stats.CacheCreation,
				"interactions_count": stats.InteractionsCount,
				"estimated_count":    stats.EstimatedCount,
			},
			"cache_hit_pct":           stats.CacheHitPct,
			"reduction_pct_estimated": reduction,
			"baseline_naive_tokens":   baselineTokens,
			"baseline_dir":            baselineDir,
			"by_model":                stats.ByModel,
			"by_project":              stats.ByProject,
			"daily":                   stats.Daily,
		})
	}
}

// estimateBaselineTokens walks `dir` summing source-file bytes (excluding common
// noise: .git, node_modules, dist, build, .venv, vendor) and divides by 4 to get
// a chars-per-token approximation. Returns 0 on any walk error.
//
// This is a deliberate proxy — Korva's value proposition is that it injects only
// what's relevant. The baseline answers "what would tokens look like if we sent
// the entire working tree?" and is presented to the UI labeled `estimated`.
func estimateBaselineTokens(dir string) int64 {
	if dir == "" {
		return 0
	}
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // intentionally skip unreadable entries, do not abort the walk
			return nil
		}
		if d.IsDir() {
			if isSkippableDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip very large binary-looking files.
		info, err := d.Info()
		if err != nil {
			//nolint:nilerr // intentionally skip entries whose info cannot be read
			return nil
		}
		if info.Size() > 5*1024*1024 {
			return nil
		}
		if isSkippableFile(d.Name()) {
			return nil
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0
	}
	return total / 4 // chars/token approximation
}

func isSkippableDir(name string) bool {
	switch name {
	case ".git", "node_modules", "dist", "build", ".venv", "venv",
		"vendor", "target", ".next", ".turbo", "out", "coverage":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func isSkippableFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".pdf", ".zip", ".tar", ".gz", ".7z",
		".woff", ".woff2", ".ttf", ".otf",
		".mp4", ".mov", ".webm",
		".sqlite", ".db",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
