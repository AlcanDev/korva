package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVaultBasePriority verifies the resolution order documented on
// vaultBase(): env var > config endpoint > config port > built-in default.
// Each case mutates only the inputs that matter so failures pinpoint the
// exact precedence rule that regressed.
func TestVaultBasePriority(t *testing.T) {
	// Sandbox HOME so the test doesn't touch the real ~/.korva/config.json.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".korva")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	writeCfg := func(t *testing.T, body string) {
		t.Helper()
		if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("env var beats everything", func(t *testing.T) {
		writeCfg(t, `{"vault":{"endpoint":"https://from-config.example","port":1234}}`)
		t.Setenv("KORVA_VAULT_ENDPOINT", "https://from-env.example/")
		got := vaultBase()
		want := "https://from-env.example"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("config endpoint beats config port", func(t *testing.T) {
		writeCfg(t, `{"vault":{"endpoint":"https://from-config.example/","port":1234}}`)
		t.Setenv("KORVA_VAULT_ENDPOINT", "")
		got := vaultBase()
		want := "https://from-config.example"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("config port falls back to local", func(t *testing.T) {
		writeCfg(t, `{"vault":{"port":9999}}`)
		t.Setenv("KORVA_VAULT_ENDPOINT", "")
		got := vaultBase()
		want := "http://127.0.0.1:9999"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("default 7437 when nothing set", func(t *testing.T) {
		writeCfg(t, `{}`)
		t.Setenv("KORVA_VAULT_ENDPOINT", "")
		got := vaultBase()
		want := "http://127.0.0.1:7437"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("trailing slash and whitespace are stripped", func(t *testing.T) {
		writeCfg(t, `{}`)
		t.Setenv("KORVA_VAULT_ENDPOINT", "  https://api.korva.dev/   ")
		got := vaultBase()
		want := "https://api.korva.dev"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestAuthTokenCommand exercises the token printer end-to-end at the
// function boundary (cobra runner not needed; runAuthToken takes the cmd
// only for OutOrStdout).
func TestAuthTokenCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := os.MkdirAll(filepath.Join(tmp, ".korva"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tokenPath := filepath.Join(tmp, ".korva", "session.token")

	// 1. Missing file → clear error pointing at the path + next step.
	err := runAuthToken(rootCmd, nil)
	if err == nil {
		t.Fatal("expected error when token file is missing")
	}
	if !contains(err.Error(), tokenPath) {
		t.Errorf("error should mention path %q; got %q", tokenPath, err.Error())
	}
	if !contains(err.Error(), "korva auth login") {
		t.Errorf("error should suggest `korva auth login`; got %q", err.Error())
	}

	// 2. Empty file → distinguishable error from missing file.
	if err := os.WriteFile(tokenPath, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runAuthToken(rootCmd, nil); err == nil {
		t.Fatal("expected error when token file is empty")
	} else if !contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error; got %q", err.Error())
	}

	// 3. Valid token → printed to the cobra command's writer.
	if err := os.WriteFile(tokenPath, []byte("  korva_session_abc123  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	buf := captureStdout(t, func() error {
		return runAuthToken(rootCmd, nil)
	})
	if buf != "korva_session_abc123\n" {
		t.Errorf("output = %q, want %q", buf, "korva_session_abc123\n")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
