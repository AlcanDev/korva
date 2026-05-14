package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/oklog/ulid/v2"
)

// ValidationError is returned by Validate / WriteAtomic when the config
// fails an invariant check. The Field is dotted (e.g. "vault.port").
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("config: %s: %s", e.Field, e.Message)
}

// ConflictError is returned by WriteAtomic when the on-disk content's hash
// does not match the ExpectedHash provided by the caller. This signals that
// the config was modified by someone else between read and write.
type ConflictError struct {
	ExpectedHash string
	ActualHash   string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("config: stale write — expected hash %q on disk, found %q",
		e.ExpectedHash, e.ActualHash)
}

// WriteOptions controls WriteAtomic's behavior.
type WriteOptions struct {
	// ExpectedHash, when non-empty, must match the SHA-256 of the on-disk file
	// before the write proceeds. Used to detect concurrent writes.
	ExpectedHash string
}

// WriteResult is returned by WriteAtomic on success.
type WriteResult struct {
	BeforeJSON      string   // raw bytes that were on disk (empty if file was new)
	AfterJSON       string   // raw bytes now on disk
	BeforeHash      string   // SHA-256 of BeforeJSON
	AfterHash       string   // SHA-256 of AfterJSON
	RestartRequired []string // dotted field paths whose change requires a vault restart
}

// restartRequiredFields lists the dotted-path fields whose change requires the
// vault server to restart for the new value to take effect.
var restartRequiredFields = []string{
	"vault.port",
	"sentinel.hooks",
	"hive.endpoint",
	"hive.enabled",
}

// Validate enforces invariants that JSON unmarshaling cannot — port range,
// known agent/country, well-formed URLs, non-empty version, etc.
//
// Returns the *first* problem found. Callers should surface it to the user as
// a 400 with the field name so the UI can highlight the offending input.
func Validate(cfg KorvaConfig) error {
	if cfg.Vault.Port != 0 && (cfg.Vault.Port < 1024 || cfg.Vault.Port > 65535) {
		return &ValidationError{Field: "vault.port", Message: "port must be between 1024 and 65535"}
	}
	if cfg.Beacon.Port != 0 && (cfg.Beacon.Port < 1024 || cfg.Beacon.Port > 65535) {
		return &ValidationError{Field: "beacon.port", Message: "port must be between 1024 and 65535"}
	}
	if cfg.Agent != "" {
		// Kept in sync with internal/harness.AllEditors. The config
		// package can't import harness without pulling in the
		// templates embed.FS, so we hardcode the list here; the
		// TestValidate_AgentMirrorsHarnessEditors check pins drift.
		switch cfg.Agent {
		case "claude", "cursor", "copilot", "windsurf", "continue", "aider", "codex":
			// ok
		default:
			return &ValidationError{Field: "agent",
				Message: fmt.Sprintf("agent must be one of: claude, cursor, copilot, windsurf, continue, aider, codex (got %q)", cfg.Agent)}
		}
	}
	if cfg.Country != "" && (len(cfg.Country) != 2 || strings.ToUpper(cfg.Country) != cfg.Country) {
		return &ValidationError{Field: "country", Message: "country must be a 2-letter ISO code (uppercase)"}
	}
	if cfg.Hive.Endpoint != "" {
		if _, err := url.Parse(cfg.Hive.Endpoint); err != nil {
			return &ValidationError{Field: "hive.endpoint", Message: "must be a valid URL"}
		}
	}
	if cfg.License.ActivationURL != "" {
		if _, err := url.Parse(cfg.License.ActivationURL); err != nil {
			return &ValidationError{Field: "license.activation_url", Message: "must be a valid URL"}
		}
	}
	for i, p := range cfg.Vault.PrivatePatterns {
		if strings.TrimSpace(p) == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("vault.private_patterns[%d]", i),
				Message: "pattern cannot be blank",
			}
		}
	}
	for i, scroll := range cfg.Lore.ActiveScrolls {
		if strings.TrimSpace(scroll) == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("lore.active_scrolls[%d]", i),
				Message: "scroll name cannot be blank",
			}
		}
	}
	if cfg.Lore.ScrollPriority != "" {
		switch cfg.Lore.ScrollPriority {
		case "private_first", "public_first":
			// ok
		default:
			return &ValidationError{Field: "lore.scroll_priority",
				Message: "must be one of: private_first, public_first"}
		}
	}
	for i, hook := range cfg.Sentinel.Hooks {
		switch hook {
		case "pre-commit", "pre-push", "commit-msg":
			// ok
		default:
			return &ValidationError{
				Field:   fmt.Sprintf("sentinel.hooks[%d]", i),
				Message: fmt.Sprintf("unknown hook %q (allowed: pre-commit, pre-push, commit-msg)", hook),
			}
		}
	}
	return nil
}

// WriteAtomic validates `cfg`, then writes it to `path` atomically:
//
//  1. Read current file (if any) and compute its hash.
//  2. If opts.ExpectedHash is non-empty and != current hash, return ConflictError.
//  3. Validate cfg.
//  4. Marshal to indented JSON.
//  5. Write to a sibling .tmp file, fsync, then rename(.tmp, path).
//
// On any error after step 4 the .tmp file is removed.
//
// The diff between before and after content is summarized in
// WriteResult.RestartRequired so the UI can show a "restart vault" banner.
func WriteAtomic(path string, cfg KorvaConfig, opts WriteOptions) (*WriteResult, error) {
	if path == "" {
		return nil, errors.New("config: WriteAtomic: path is required")
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	beforeBytes, _ := os.ReadFile(path)
	beforeStr := string(beforeBytes)
	beforeHash := sha256Hex(beforeStr)

	if opts.ExpectedHash != "" && opts.ExpectedHash != beforeHash {
		return nil, &ConflictError{ExpectedHash: opts.ExpectedHash, ActualHash: beforeHash}
	}

	newBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing config: %w", err)
	}
	afterStr := string(newBytes)
	afterHash := sha256Hex(afterStr)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ensuring config dir: %w", err)
	}

	tmp := path + ".tmp." + ulid.Make().String()
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening temp file: %w", err)
	}
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(newBytes); err != nil {
		_ = f.Close()
		cleanup()
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return nil, fmt.Errorf("fsync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return nil, fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	return &WriteResult{
		BeforeJSON:      beforeStr,
		AfterJSON:       afterStr,
		BeforeHash:      beforeHash,
		AfterHash:       afterHash,
		RestartRequired: diffRestartRequired(beforeStr, afterStr),
	}, nil
}

// diffRestartRequired returns the list of restart-sensitive fields whose value
// changed between the before and after JSON. It deserializes both into maps
// and compares dotted paths — robust to JSON whitespace differences.
func diffRestartRequired(before, after string) []string {
	if before == "" {
		// First write — no restart required, the values become the new baseline.
		return nil
	}
	beforeMap, _ := parseJSONToMap(before)
	afterMap, _ := parseJSONToMap(after)
	out := []string{}
	for _, dotted := range restartRequiredFields {
		bv, _ := pluckPath(beforeMap, dotted)
		av, _ := pluckPath(afterMap, dotted)
		if !equalAny(bv, av) {
			out = append(out, dotted)
		}
	}
	return out
}

func parseJSONToMap(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{}, err
	}
	return m, nil
}

func pluckPath(m map[string]any, dotted string) (any, bool) {
	parts := strings.Split(dotted, ".")
	cur := any(m)
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := mm[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func equalAny(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HashFile returns the SHA-256 of the file's bytes, or "" if missing.
func HashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return sha256Hex(string(data))
}
