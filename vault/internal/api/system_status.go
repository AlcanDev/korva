package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/detect"
	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/store"
)

// systemStatusInputs bundles all the moving parts the handler needs so the
// router can wire them once and the handler stays pure.
type systemStatusInputs struct {
	Store      *store.Store
	HiveWorker *hive.Worker
	License    *license.License
	StartedAt  time.Time
	Version    string
	Port       int
	// ConfigPathLocal points at the project-level korva.config.json. When the
	// file does not exist the Sentinel section reports best-effort defaults.
	ConfigPathLocal string
}

// adminSystemStatus handles GET /admin/system-status — the Observatory
// dashboard's "single fetch, render everything" endpoint.
func adminSystemStatus(in systemStatusInputs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"ide":      detect.IDEs(),
			"vault":    buildVaultStatus(in),
			"hive":     buildHiveStatus(in.HiveWorker),
			"sentinel": buildSentinelStatus(in.ConfigPathLocal),
			"lore":     buildLoreStatus(in.ConfigPathLocal),
			"skills":   buildSkillsStatus(in.Store),
			"license":  buildLicenseStatus(in.License),
		}

		// Counts are best-effort — degrade gracefully if the store has issues.
		stats, err := in.Store.Stats()
		if err == nil && stats != nil {
			resp["sessions"] = map[string]any{
				"total":      stats.TotalSessions,
				"active_24h": activeSessionsCount(in.Store, 24*time.Hour),
			}
			resp["observations"] = map[string]any{
				"total":   stats.TotalObservations,
				"by_type": stats.ByType,
			}
			resp["prompts"] = map[string]any{"total": stats.TotalPrompts}
		} else {
			resp["sessions"] = map[string]any{"total": 0, "active_24h": 0}
			resp["observations"] = map[string]any{"total": 0, "by_type": map[string]int{}}
			resp["prompts"] = map[string]any{"total": 0}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func buildVaultStatus(in systemStatusInputs) map[string]any {
	uptime := int64(0)
	if !in.StartedAt.IsZero() {
		uptime = int64(time.Since(in.StartedAt).Seconds())
	}
	return map[string]any{
		"running":    true, // we are handling the request, by definition.
		"port":       in.Port,
		"pid":        os.Getpid(),
		"uptime_sec": uptime,
		"version":    in.Version,
	}
}

func buildHiveStatus(w *hive.Worker) map[string]any {
	if w == nil {
		return map[string]any{
			"enabled":            false,
			"phase":              "disabled",
			"pending_outbox":     0,
			"consecutive_errors": 0,
		}
	}
	st := w.Status()
	out := map[string]any{
		"enabled":            true,
		"phase":              st.Phase,
		"pending_outbox":     st.PendingCount,
		"consecutive_errors": st.ConsecutiveErrors,
		"pull_count":         st.PullCount,
	}
	if st.LastSyncAt != nil {
		out["last_sync_at"] = st.LastSyncAt.UTC().Format(time.RFC3339)
	}
	if st.LastError != "" {
		out["last_error"] = st.LastError
	}
	return out
}

// buildSentinelStatus reads the local korva.config.json (when present) and
// reports Sentinel's enabled flag, configured hooks, and rules path. Whether
// the hooks are actually installed is checked on disk (.git/hooks/<name>).
func buildSentinelStatus(configPath string) map[string]any {
	out := map[string]any{
		"enabled":         false,
		"hooks_installed": []string{},
		"rules_total":     0,
		"builtin_count":   0,
		"custom_count":    0,
		"rules_path":      "",
		"profile":         "standard",
	}

	cfg, err := loadConfigBestEffort(configPath)
	if err != nil {
		return out
	}

	out["enabled"] = cfg.Sentinel.Enabled
	out["rules_path"] = cfg.Sentinel.RulesPath
	out["builtin_count"] = sentinelBuiltinCount(out["profile"].(string))

	configured := cfg.Sentinel.Hooks
	installed := make([]string, 0, len(configured))
	for _, name := range configured {
		hookPath := filepath.Join(filepath.Dir(configPath), ".git", "hooks", name)
		if pathExists(hookPath) {
			installed = append(installed, name)
		}
	}
	out["hooks_installed"] = installed

	custom := countCustomRules(cfg.Sentinel.RulesPath, filepath.Dir(configPath))
	out["custom_count"] = custom
	out["rules_total"] = out["builtin_count"].(int) + custom
	return out
}

// sentinelBuiltinCount returns the count of built-in rules active for the
// given profile. The numbers match the validator's RulesForProfile().
func sentinelBuiltinCount(profile string) int {
	switch profile {
	case "minimal":
		return 1
	case "strict":
		return 10
	case "custom":
		return 0
	default: // standard
		return 4
	}
}

// countCustomRules returns the number of rules in a sentinel YAML file at
// rulesPath (relative to repoRoot when not absolute). Best effort — returns 0
// on any error.
func countCustomRules(rulesPath, repoRoot string) int {
	if rulesPath == "" {
		return 0
	}
	full := rulesPath
	if !filepath.IsAbs(full) {
		full = filepath.Join(repoRoot, rulesPath)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return 0
	}
	// Cheap: count `^- id:` lines without parsing YAML. The rules editor
	// produces a known shape — when the file is hand-edited and the count is
	// imprecise, the dashboard simply shows a slightly off number, never a 500.
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- id:") || strings.HasPrefix(t, "-id:") {
			count++
		}
	}
	return count
}

func buildLoreStatus(configPath string) map[string]any {
	out := map[string]any{
		"active_scrolls":          []string{},
		"available_scrolls_count": 0,
	}
	cfg, err := loadConfigBestEffort(configPath)
	if err == nil {
		out["active_scrolls"] = cfg.Lore.ActiveScrolls
	}
	// Count public + private scrolls under ~/.korva/lore/ (best effort).
	home, err := os.UserHomeDir()
	if err == nil {
		out["available_scrolls_count"] = countScrollFiles(filepath.Join(home, ".korva", "lore"))
	}
	return out
}

func countScrollFiles(dir string) int {
	if !pathExists(dir) {
		return 0
	}
	count := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			//nolint:nilerr // intentionally swallow walk errors; counting scrolls is best-effort
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			count++
		}
		return nil
	})
	return count
}

// buildSkillsStatus surfaces a coarse skills overview without coupling to the
// admin/team listing logic. Returns counts and last sync timestamp when present.
func buildSkillsStatus(s *store.Store) map[string]any {
	out := map[string]any{
		"installed_count": 0,
		"last_sync_at":    nil,
		"sync_status":     "ok",
	}
	if s == nil {
		return out
	}
	count, err := skillsCount(s)
	if err == nil {
		out["installed_count"] = count
	}
	if last, err := lastSkillSyncAt(s); err == nil && last != nil {
		out["last_sync_at"] = last.UTC().Format(time.RFC3339)
	}
	return out
}

func buildLicenseStatus(lic *license.License) map[string]any {
	if lic == nil {
		return map[string]any{
			"tier":          "community",
			"expiration_at": nil,
			"seats_used":    0,
			"seats_total":   0,
		}
	}
	return map[string]any{
		"tier":          string(lic.Tier),
		"expiration_at": lic.ExpiresAt.UTC().Format(time.RFC3339),
		"seats_used":    0, // populated by separate /admin/teams data; left at 0 here.
		"seats_total":   lic.Seats,
	}
}

// loadConfigBestEffort reads korva.config.json and returns a zero-value config
// (with safe defaults) when the file is missing or unreadable.
func loadConfigBestEffort(path string) (config.KorvaConfig, error) {
	if path == "" {
		return config.KorvaConfig{}, nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.KorvaConfig{}, err
	}
	return cfg, nil
}

// pathExists is duplicated in the api package to avoid coupling the API layer
// to the detect package's internals.
func pathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// activeSessionsCount returns the number of sessions that started within the
// given window. Returns 0 on any error.
func activeSessionsCount(s *store.Store, window time.Duration) int {
	cutoff := time.Now().Add(-window).UTC().Format("2006-01-02 15:04:05")
	row := s.DB().QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE started_at >= ?`, cutoff,
	)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0
	}
	return n
}

func skillsCount(s *store.Store) (int, error) {
	row := s.DB().QueryRow(`SELECT COUNT(*) FROM skills`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func lastSkillSyncAt(s *store.Store) (*time.Time, error) {
	row := s.DB().QueryRow(`SELECT MAX(synced_at) FROM skill_sync_log`)
	var raw *string
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", *raw)
	if err != nil {
		// Treat unparseable timestamps as "no sync yet" rather than failing
		// the whole status endpoint — the column may have been written by an
		// older binary with a different format.
		return nil, fmt.Errorf("parsing skill sync timestamp %q: %w", *raw, err)
	}
	return &t, nil
}

// Compile-time guard so json import is always referenced (some helpers will
// stringify configs in follow-up changes).
var _ = json.Marshal
