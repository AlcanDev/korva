package detect

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProbe_DetectsConfigDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixtures use POSIX paths")
	}

	home := t.TempDir()
	macSupport := filepath.Join(home, "Library", "Application Support")
	linuxCfg := filepath.Join(home, ".config")

	// Create config dirs as if VS Code, Cursor and Claude Code were installed.
	mustMkdir(t, filepath.Join(macSupport, "Code", "User"))
	mustMkdir(t, filepath.Join(macSupport, "Cursor", "User"))
	mustMkdir(t, filepath.Join(home, ".claude"))

	got := Probe(Options{
		HomeDir:       home,
		MacAppSupport: macSupport,
		LinuxConfig:   linuxCfg,
		LookPathFn:    func(string) (string, error) { return "", errors.New("not found") },
	})

	names := namesOf(got)
	if !contains(names, "VS Code") {
		t.Errorf("expected VS Code detected, got %v", names)
	}
	if !contains(names, "Cursor") {
		t.Errorf("expected Cursor detected, got %v", names)
	}
	if !contains(names, "Claude Code") {
		t.Errorf("expected Claude Code detected, got %v", names)
	}
	if len(got) > 0 && !got[0].IsDefault {
		t.Errorf("expected first IDE to be marked default, got %+v", got[0])
	}
}

func TestProbe_FallbackToBinary(t *testing.T) {
	// No config dirs exist; only a fake `nvim` binary in PATH.
	got := Probe(Options{
		HomeDir:       t.TempDir(),
		MacAppSupport: "/nonexistent",
		LinuxConfig:   "/nonexistent",
		LookPathFn: func(name string) (string, error) {
			if name == "nvim" {
				return "/usr/local/bin/nvim", nil
			}
			return "", errors.New("not found")
		},
	})

	if !contains(namesOf(got), "Neovim") {
		t.Errorf("expected Neovim via PATH fallback, got %v", namesOf(got))
	}
}

func TestProbe_DetectsKorvaMCP_ClaudeSettings(t *testing.T) {
	home := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".claude"))
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{
		"mcpServers": {
			"korva-vault": {"command": "korva-vault"}
		}
	}`)

	got := Probe(Options{
		HomeDir:       home,
		MacAppSupport: filepath.Join(home, "AppSupport"),
		LinuxConfig:   filepath.Join(home, ".config"),
		LookPathFn:    func(string) (string, error) { return "", errors.New("not found") },
	})

	for _, ide := range got {
		if ide.Name == "Claude Code" {
			if !ide.HasKorvaMCP {
				t.Errorf("expected HasKorvaMCP=true, got %+v", ide)
			}
			return
		}
	}
	t.Errorf("Claude Code not detected: %v", namesOf(got))
}

func TestProbe_DetectsKorvaMCP_VSCodeMCPJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX-style fixture path")
	}
	home := t.TempDir()
	macSupport := filepath.Join(home, "Library", "Application Support")
	codeUser := filepath.Join(macSupport, "Code", "User")
	mustMkdir(t, codeUser)
	mustWriteFile(t, filepath.Join(codeUser, "mcp.json"), `{
		"servers": {
			"korva": {"command": "korva-vault", "args": []}
		}
	}`)

	got := Probe(Options{
		HomeDir:       home,
		MacAppSupport: macSupport,
		LinuxConfig:   filepath.Join(home, ".config"),
		LookPathFn:    func(string) (string, error) { return "", errors.New("not found") },
	})

	for _, ide := range got {
		if ide.Name == "VS Code" {
			if !ide.HasKorvaMCP {
				t.Errorf("expected HasKorvaMCP=true, got %+v", ide)
			}
			return
		}
	}
	t.Errorf("VS Code not detected: %v", namesOf(got))
}

func TestProbe_NoIDEsFound(t *testing.T) {
	got := Probe(Options{
		HomeDir:       "/nonexistent-home",
		MacAppSupport: "/nonexistent-as",
		LinuxConfig:   "/nonexistent-cfg",
		LookPathFn:    func(string) (string, error) { return "", errors.New("not found") },
	})
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d IDEs: %v", len(got), namesOf(got))
	}
}

func TestIDEs_CachesResults(t *testing.T) {
	ResetCache()
	first := IDEs()
	second := IDEs()
	if len(first) != len(second) {
		t.Errorf("cache returned different result on second call")
	}
}

func TestJsonContainsKorva_Substring(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.json")
	mustWriteFile(t, path, `{ /* comment */ "mcpServers": { "korva": {} } }`)
	found, ok := jsonContainsKorva(path)
	if !ok {
		t.Fatal("expected ok=true on substring fallback")
	}
	if !found {
		t.Error("expected found=true via substring scan")
	}
}

func TestJsonContainsKorva_MissingFile(t *testing.T) {
	found, ok := jsonContainsKorva(filepath.Join(t.TempDir(), "missing.json"))
	if ok {
		t.Error("expected ok=false for missing file")
	}
	if found {
		t.Error("expected found=false for missing file")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func namesOf(ides []IDE) []string {
	out := make([]string, len(ides))
	for i, ide := range ides {
		out[i] = ide.Name
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// Compile-time guard that `time` package is referenced (silences unused import
// when test layout shifts).
var _ = time.Time{}

// Compile-time guard for strings package.
var _ = strings.Contains
