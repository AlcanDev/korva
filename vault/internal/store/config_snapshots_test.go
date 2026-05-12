package store

import (
	"strings"
	"testing"
)

func TestSaveConfigSnapshot_HappyPath(t *testing.T) {
	s := newTestStore(t)

	id, err := s.SaveConfigSnapshot(ConfigSnapshot{
		Actor:      "admin",
		Scope:      "local",
		FilePath:   "/tmp/korva.config.json",
		BeforeJSON: `{"vault":{"auto_start":true}}`,
		AfterJSON:  `{"vault":{"auto_start":false}}`,
	})
	if err != nil {
		t.Fatalf("SaveConfigSnapshot() error = %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := s.GetConfigSnapshot(id)
	if err != nil {
		t.Fatalf("GetConfigSnapshot() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.Scope != "local" {
		t.Errorf("Scope = %q, want %q", got.Scope, "local")
	}
	if got.BeforeHash == "" || got.AfterHash == "" {
		t.Error("hashes should be auto-computed when empty")
	}
	if got.BeforeHash == got.AfterHash {
		t.Error("hashes should differ when content differs")
	}
}

func TestSaveConfigSnapshot_RequiresScopeAndPath(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name string
		in   ConfigSnapshot
	}{
		{"missing scope", ConfigSnapshot{FilePath: "/tmp/x.json"}},
		{"missing path", ConfigSnapshot{Scope: "local"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.SaveConfigSnapshot(tc.in); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestListConfigSnapshots_NewestFirst(t *testing.T) {
	s := newTestStore(t)

	for i, scope := range []string{"local", "local", "global"} {
		if _, err := s.SaveConfigSnapshot(ConfigSnapshot{
			Actor:      "admin",
			Scope:      scope,
			FilePath:   "/tmp/x.json",
			AfterJSON:  `{"v":` + string(rune('0'+i)) + `}`,
			BeforeJSON: `{}`,
		}); err != nil {
			t.Fatalf("seed Save() error = %v", err)
		}
	}

	all, err := s.ListConfigSnapshots("", 10)
	if err != nil {
		t.Fatalf("ListConfigSnapshots() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all count = %d, want 3", len(all))
	}

	local, _ := s.ListConfigSnapshots("local", 10)
	if len(local) != 2 {
		t.Errorf("local scope count = %d, want 2", len(local))
	}
	for _, snap := range local {
		if snap.Scope != "local" {
			t.Errorf("filter leaked: scope = %q", snap.Scope)
		}
	}
}

func TestLatestConfigSnapshot(t *testing.T) {
	s := newTestStore(t)

	if got, _ := s.LatestConfigSnapshot("local"); got != nil {
		t.Error("expected nil for empty store")
	}

	if _, err := s.SaveConfigSnapshot(ConfigSnapshot{
		Actor: "admin", Scope: "local", FilePath: "/tmp/x.json",
		AfterJSON: `{"v":1}`,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := s.LatestConfigSnapshot("local")
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if !strings.Contains(got.AfterJSON, `"v":1`) {
		t.Errorf("AfterJSON unexpected: %q", got.AfterJSON)
	}
}

func TestSha256HexIsStable(t *testing.T) {
	// SHA-256 of "" is well-known.
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := sha256Hex(""); got != want {
		t.Errorf("sha256Hex(\"\") = %s, want %s", got, want)
	}
}
