package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alcandev/korva/internal/config"
)

func TestBeaconPort_FallsBackTo5173(t *testing.T) {
	if got := beaconPort(config.KorvaConfig{}); got != 5173 {
		t.Errorf("beaconPort(empty) = %d, want 5173", got)
	}
	cfg := config.KorvaConfig{Beacon: config.BeaconConfig{Port: 4321}}
	if got := beaconPort(cfg); got != 4321 {
		t.Errorf("beaconPort(custom) = %d, want 4321", got)
	}
}

func TestIsBeaconDir(t *testing.T) {
	tmp := t.TempDir()

	// Empty dir — not Beacon.
	if isBeaconDir(tmp) {
		t.Error("empty dir should not look like Beacon")
	}

	// Dir with package.json but wrong name — not Beacon.
	stranger := filepath.Join(tmp, "stranger")
	if err := os.MkdirAll(stranger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stranger, "package.json"),
		[]byte(`{"name": "some-other-thing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if isBeaconDir(stranger) {
		t.Error("wrong-name package.json should not match")
	}

	// Real-looking Beacon source.
	beacon := filepath.Join(tmp, "beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beacon, "package.json"),
		[]byte(`{
  "name": "korva-beacon",
  "version": "1.0.0"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isBeaconDir(beacon) {
		t.Error("korva-beacon package.json should match")
	}
}

func TestResolveBeaconDevDir_ExplicitConfig(t *testing.T) {
	tmp := t.TempDir()
	beacon := filepath.Join(tmp, "myrepo", "beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beacon, "package.json"),
		[]byte(`{"name": "korva-beacon"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.KorvaConfig{Beacon: config.BeaconConfig{DevDir: beacon}}
	got, err := resolveBeaconDevDir(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "beacon") {
		t.Errorf("got = %q, want path ending in /beacon", got)
	}
}

func TestResolveBeaconDevDir_EnvVar(t *testing.T) {
	tmp := t.TempDir()
	beacon := filepath.Join(tmp, "envrepo", "beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beacon, "package.json"),
		[]byte(`{"name": "korva-beacon"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KORVA_BEACON_DIR", beacon)
	got, err := resolveBeaconDevDir(config.KorvaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "beacon") {
		t.Errorf("got = %q", got)
	}
}

func TestResolveBeaconDevDir_NotFound(t *testing.T) {
	// Run from an isolated tmp dir with no parent chain that contains Beacon.
	tmp := t.TempDir()
	t.Chdir(tmp)

	_, err := resolveBeaconDevDir(config.KorvaConfig{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "beacon source not found") {
		t.Errorf("error message lacks hint: %q", err.Error())
	}
}
