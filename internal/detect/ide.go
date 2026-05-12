// Package detect probes the local machine for installed IDEs and reports
// whether each is configured to use Korva's MCP server.
//
// The detector is filesystem-first: a folder under the OS-conventional
// "user config" location (e.g. ~/Library/Application Support/Code/User on
// macOS) counts as evidence the IDE is installed. As a fallback the detector
// checks PATH for the IDE's CLI binary.
//
// Results are cached in-process for 60 seconds so the system-status endpoint
// can be hit frequently without re-walking the filesystem each time.
package detect

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CacheTTL controls how long detection results are cached.
const CacheTTL = 60 * time.Second

// IDE is one detected editor or CLI agent.
type IDE struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	ConfigPath  string `json:"config_path,omitempty"`
	HasKorvaMCP bool   `json:"has_korva_mcp"`
	IsDefault   bool   `json:"is_default"`
}

// Options lets callers (mostly tests) override OS-conventional paths.
type Options struct {
	HomeDir       string
	MacAppSupport string
	LinuxConfig   string
	WinAppData    string
	// LookPathFn replaces exec.LookPath for tests. When nil the real lookup runs.
	LookPathFn func(name string) (string, error)
	// NowFn replaces time.Now for cache TTL tests.
	NowFn func() time.Time
}

// IDEs returns the list of detected IDEs using OS defaults and a 60s cache.
func IDEs() []IDE {
	defaultDetector.mu.Lock()
	defer defaultDetector.mu.Unlock()

	now := time.Now()
	if defaultDetector.cached != nil && now.Before(defaultDetector.expiresAt) {
		return cloneIDEs(defaultDetector.cached)
	}
	out := Probe(Options{})
	defaultDetector.cached = out
	defaultDetector.expiresAt = now.Add(CacheTTL)
	return cloneIDEs(out)
}

// ResetCache clears the in-process cache. Used by tests.
func ResetCache() {
	defaultDetector.mu.Lock()
	defaultDetector.cached = nil
	defaultDetector.expiresAt = time.Time{}
	defaultDetector.mu.Unlock()
}

// Probe runs detection without using the cache. Tests inject Options to control
// filesystem and PATH lookups.
func Probe(opts Options) []IDE {
	o := normalizeOptions(opts)
	probes := buildProbes()

	out := make([]IDE, 0, len(probes))
	for _, p := range probes {
		ide, found := detectOne(p, o)
		if !found {
			continue
		}
		out = append(out, ide)
	}
	if len(out) > 0 {
		out[0].IsDefault = true
	}
	return out
}

// ── internal ────────────────────────────────────────────────────────────────

type detector struct {
	mu        sync.Mutex
	cached    []IDE
	expiresAt time.Time
}

var defaultDetector = &detector{}

// probe describes one IDE to look for.
type probe struct {
	Name string
	// ConfigCandidates is a list of OS-tagged paths to try in order. The first
	// existing path wins. Tags: "mac", "linux", "windows", "any" (any OS).
	ConfigCandidates []probeCandidate
	// BinaryName, when non-empty, is searched on PATH as a fallback if no
	// config dir was found.
	BinaryName string
	// MCPDetector inspects an existing ConfigPath to decide whether Korva is
	// configured. May be nil — in which case HasKorvaMCP stays false.
	MCPDetector func(configPath string) bool
}

type probeCandidate struct {
	OS      string // "mac" | "linux" | "windows" | "any"
	PathRel string // path interpreted relative to the OS-specific base dir
	Base    string // "home" | "macAppSupport" | "linuxConfig" | "winAppData"
}

func buildProbes() []probe {
	return []probe{
		{
			Name: "Claude Code",
			ConfigCandidates: []probeCandidate{
				{OS: "any", Base: "home", PathRel: ".claude"},
			},
			BinaryName:  "claude",
			MCPDetector: hasKorvaInClaudeSettings,
		},
		{
			Name: "Cursor",
			ConfigCandidates: []probeCandidate{
				{OS: "mac", Base: "macAppSupport", PathRel: "Cursor/User"},
				{OS: "linux", Base: "linuxConfig", PathRel: "Cursor/User"},
				{OS: "windows", Base: "winAppData", PathRel: "Cursor/User"},
			},
			BinaryName:  "cursor",
			MCPDetector: hasKorvaInVSCodeStyleConfig,
		},
		{
			Name: "VS Code",
			ConfigCandidates: []probeCandidate{
				{OS: "mac", Base: "macAppSupport", PathRel: "Code/User"},
				{OS: "linux", Base: "linuxConfig", PathRel: "Code/User"},
				{OS: "windows", Base: "winAppData", PathRel: "Code/User"},
			},
			BinaryName:  "code",
			MCPDetector: hasKorvaInVSCodeStyleConfig,
		},
		{
			Name: "JetBrains",
			ConfigCandidates: []probeCandidate{
				{OS: "mac", Base: "macAppSupport", PathRel: "JetBrains"},
				{OS: "linux", Base: "linuxConfig", PathRel: "JetBrains"},
				{OS: "windows", Base: "winAppData", PathRel: "JetBrains"},
			},
		},
		{
			Name: "Zed",
			ConfigCandidates: []probeCandidate{
				{OS: "mac", Base: "home", PathRel: ".config/zed"},
				{OS: "linux", Base: "home", PathRel: ".config/zed"},
				{OS: "windows", Base: "winAppData", PathRel: "Zed"},
			},
			BinaryName: "zed",
		},
		{
			Name:       "Neovim",
			BinaryName: "nvim",
		},
		{
			Name:       "Vim",
			BinaryName: "vim",
		},
	}
}

func detectOne(p probe, o Options) (IDE, bool) {
	for _, c := range p.ConfigCandidates {
		if !matchOS(c.OS) {
			continue
		}
		base := resolveBase(c.Base, o)
		if base == "" {
			continue
		}
		full := filepath.Join(base, c.PathRel)
		if !pathExists(full) {
			continue
		}
		ide := IDE{Name: p.Name, ConfigPath: full}
		if p.MCPDetector != nil {
			ide.HasKorvaMCP = p.MCPDetector(full)
		}
		return ide, true
	}
	if p.BinaryName != "" {
		if path, err := lookPath(p.BinaryName, o); err == nil && path != "" {
			return IDE{Name: p.Name, ConfigPath: ""}, true
		}
	}
	return IDE{}, false
}

func resolveBase(name string, o Options) string {
	switch name {
	case "home":
		return o.HomeDir
	case "macAppSupport":
		return o.MacAppSupport
	case "linuxConfig":
		return o.LinuxConfig
	case "winAppData":
		return o.WinAppData
	}
	return ""
}

func matchOS(tag string) bool {
	if tag == "any" {
		return true
	}
	switch runtime.GOOS {
	case "darwin":
		return tag == "mac"
	case "linux":
		return tag == "linux"
	case "windows":
		return tag == "windows"
	}
	// Fallback: use linux conventions for *BSD/etc.
	return tag == "linux"
}

func normalizeOptions(o Options) Options {
	if o.HomeDir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			o.HomeDir = h
		}
	}
	if o.MacAppSupport == "" && o.HomeDir != "" {
		o.MacAppSupport = filepath.Join(o.HomeDir, "Library", "Application Support")
	}
	if o.LinuxConfig == "" && o.HomeDir != "" {
		o.LinuxConfig = filepath.Join(o.HomeDir, ".config")
	}
	if o.WinAppData == "" {
		o.WinAppData = os.Getenv("APPDATA")
	}
	return o
}

func lookPath(name string, o Options) (string, error) {
	if o.LookPathFn != nil {
		return o.LookPathFn(name)
	}
	return exec.LookPath(name)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hasKorvaInClaudeSettings looks for `mcpServers.korva-vault` (or any key
// containing "korva") in ~/.claude/settings.json or ~/.claude/.claude.json.
func hasKorvaInClaudeSettings(configPath string) bool {
	for _, name := range []string{"settings.json", ".claude.json", "claude_desktop_config.json"} {
		if found, ok := jsonContainsKorva(filepath.Join(configPath, name)); ok && found {
			return true
		}
	}
	return false
}

// hasKorvaInVSCodeStyleConfig inspects mcp.json or settings.json (both used by
// VS Code and Cursor) for any reference to Korva.
func hasKorvaInVSCodeStyleConfig(configPath string) bool {
	for _, name := range []string{"mcp.json", "settings.json"} {
		if found, ok := jsonContainsKorva(filepath.Join(configPath, name)); ok && found {
			return true
		}
	}
	return false
}

func jsonContainsKorva(path string) (bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		// Fall back to substring scan when the JSON is invalid (e.g. has comments).
		return strings.Contains(strings.ToLower(string(data)), "korva"), true
	}
	return jsonValueContains(parsed, "korva"), true
}

func jsonValueContains(v any, needle string) bool {
	switch x := v.(type) {
	case string:
		return strings.Contains(strings.ToLower(x), needle)
	case map[string]any:
		for k, vv := range x {
			if strings.Contains(strings.ToLower(k), needle) {
				return true
			}
			if jsonValueContains(vv, needle) {
				return true
			}
		}
	case []any:
		for _, item := range x {
			if jsonValueContains(item, needle) {
				return true
			}
		}
	}
	return false
}

func cloneIDEs(in []IDE) []IDE {
	out := make([]IDE, len(in))
	copy(out, in)
	return out
}
