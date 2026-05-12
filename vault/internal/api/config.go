package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/vault/internal/store"
)

// configEndpoint exposes the project's `korva.config.json` over HTTP.
//
// Two endpoints are supported:
//
//   - GET  /admin/config?scope=local|global  — read current config + hash
//   - PUT  /admin/config                     — validate, snapshot, atomic write
//
// `local` resolves to the project-level file the vault was started with;
// `global` resolves to ~/.korva/config.json (PlatformPaths()).
type configEndpoint struct {
	store           *store.Store
	pathLocal       string // CWD-resolved korva.config.json
	pathGlobalCache string // memoized for PlatformPaths().ConfigFile
}

func newConfigEndpoint(s *store.Store, pathLocal string) *configEndpoint {
	c := &configEndpoint{store: s, pathLocal: pathLocal}
	if paths, err := config.PlatformPaths(); err == nil {
		c.pathGlobalCache = paths.ConfigFile
	}
	return c
}

// resolvePath returns the on-disk path for the given scope. Empty scope
// defaults to "local" so a missing query string still works.
func (c *configEndpoint) resolvePath(scope string) (string, string, error) {
	if scope == "" {
		scope = "local"
	}
	switch scope {
	case "local":
		if c.pathLocal == "" {
			return "", "", errors.New("local config path is not configured")
		}
		return c.pathLocal, "local", nil
	case "global":
		if c.pathGlobalCache == "" {
			return "", "", errors.New("could not resolve global config path")
		}
		return c.pathGlobalCache, "global", nil
	default:
		return "", "", fmt.Errorf("unknown scope %q (expected: local | global)", scope)
	}
}

// adminGetConfig handles GET /admin/config?scope=local|global.
func adminGetConfig(c *configEndpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope := r.URL.Query().Get("scope")
		path, scope, err := c.resolvePath(scope)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		cfg, err := config.Load(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		hash := config.HashFile(path)

		writeJSON(w, http.StatusOK, map[string]any{
			"scope":          scope,
			"path":           path,
			"hash":           hash,
			"config":         cfg,
			"schema_version": "1",
			"exists":         pathExists(path),
		})
	}
}

// putConfigRequest is the wire shape for PUT /admin/config.
type putConfigRequest struct {
	Scope        string             `json:"scope"`
	ExpectedHash string             `json:"expected_hash"`
	Config       config.KorvaConfig `json:"config"`
}

// adminPutConfig handles PUT /admin/config: validate, snapshot, atomic write.
//
// Errors:
//   400 — invalid body / unknown scope / validation failure
//   409 — on-disk hash differs from expected_hash (concurrent write)
//   500 — disk I/O failure
func adminPutConfig(c *configEndpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req putConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		path, scope, err := c.resolvePath(req.Scope)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		res, err := config.WriteAtomic(path, req.Config, config.WriteOptions{
			ExpectedHash: req.ExpectedHash,
		})
		if err != nil {
			var ve *config.ValidationError
			var ce *config.ConflictError
			switch {
			case errors.As(err, &ve):
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error":   ve.Message,
					"field":   ve.Field,
					"message": ve.Error(),
				})
				return
			case errors.As(err, &ce):
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":         "config changed on disk between read and write",
					"expected_hash": ce.ExpectedHash,
					"actual_hash":   ce.ActualHash,
				})
				return
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		// Persist a snapshot for rollback / audit. Errors here do not roll back
		// the on-disk write — we logged the file change as canonical and the
		// snapshot is for post-hoc inspection.
		snapID, snapErr := c.store.SaveConfigSnapshot(store.ConfigSnapshot{
			Actor:      "admin",
			Scope:      scope,
			FilePath:   path,
			BeforeHash: res.BeforeHash,
			AfterHash:  res.AfterHash,
			BeforeJSON: res.BeforeJSON,
			AfterJSON:  res.AfterJSON,
		})
		if snapErr != nil {
			// Soft-fail: include in the response so the UI can surface a warning.
			writeJSON(w, http.StatusOK, map[string]any{
				"status":            "saved",
				"snapshot_warning":  snapErr.Error(),
				"hash":              res.AfterHash,
				"restart_required":  res.RestartRequired,
				"path":              path,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "saved",
			"snapshot_id":      snapID,
			"hash":             res.AfterHash,
			"restart_required": res.RestartRequired,
			"path":             path,
		})
	}
}

// adminListConfigSnapshots handles GET /admin/config/snapshots — recent history
// of korva.config.json mutations for the rollback / diff UX.
func adminListConfigSnapshots(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope := r.URL.Query().Get("scope")
		snaps, err := s.ListConfigSnapshots(scope, 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"snapshots": snaps,
			"count":     len(snaps),
		})
	}
}

// pathExistsForRequest is a small helper that wraps pathExists so the config
// endpoint can be tested without bringing in the system_status helpers. It
// references os.Stat directly to avoid duplicating the file-level helper.
func pathExistsForRequest(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}
