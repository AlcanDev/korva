package harness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSpecDir_JoinedCorrectly(t *testing.T) {
	got := SpecDir("/tmp/x", "auth_layer")
	want := filepath.Join("/tmp/x", "specs", "auth_layer")
	if got != want {
		t.Errorf("SpecDir = %q, want %q", got, want)
	}
}

func TestSpecComplete_FalseWhenAnyMissing(t *testing.T) {
	dir := t.TempDir()
	if SpecComplete(dir, "feat-a") {
		t.Error("empty repo cannot have a complete spec")
	}

	// Create two of three files — still incomplete.
	specPath := filepath.Join(dir, "specs", "feat-a")
	if err := os.MkdirAll(specPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"requirements.md", "design.md"} {
		if err := os.WriteFile(filepath.Join(specPath, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if SpecComplete(dir, "feat-a") {
		t.Error("2 of 3 files should not be complete")
	}
}

func TestSpecComplete_TrueWhenAllPresent(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "specs", "feat-a")
	if err := os.MkdirAll(specPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range SpecFiles {
		if err := os.WriteFile(filepath.Join(specPath, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !SpecComplete(dir, "feat-a") {
		t.Error("3 of 3 files should be complete")
	}
}

func TestMaterializeSpec_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	feature := &Feature{ID: 1, Name: "auth_layer", Title: "Auth"}
	res, err := MaterializeSpec(dir, feature, false)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(res.Written) != 3 {
		t.Errorf("Written = %v, want 3 files", res.Written)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("Skipped should be empty on fresh write, got %v", res.Skipped)
	}
	for _, f := range SpecFiles {
		if !slices.Contains(res.Written, f) {
			t.Errorf("Written missing %s", f)
		}
		if _, err := os.Stat(filepath.Join(dir, "specs", "auth_layer", f)); err != nil {
			t.Errorf("file %s not on disk: %v", f, err)
		}
	}
}

func TestMaterializeSpec_TemplatesAreRendered(t *testing.T) {
	dir := t.TempDir()
	feature := &Feature{ID: 1, Name: "the_feature", Title: "x"}
	if _, err := MaterializeSpec(dir, feature, false); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	for _, f := range SpecFiles {
		body, err := os.ReadFile(filepath.Join(dir, "specs", "the_feature", f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(body), "{{.Project}}") {
			t.Errorf("%s still contains unrendered template directive", f)
		}
		if strings.Contains(string(body), "{{.FeatureName}}") {
			t.Errorf("%s still contains unrendered FeatureName directive", f)
		}
	}
}

func TestMaterializeSpec_IdempotentByDefault(t *testing.T) {
	dir := t.TempDir()
	feature := &Feature{ID: 1, Name: "x"}

	// First call: all written.
	if _, err := MaterializeSpec(dir, feature, false); err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	// Replace requirements.md with operator content.
	custom := []byte("OPERATOR-WROTE-THIS")
	reqPath := filepath.Join(dir, "specs", "x", "requirements.md")
	if err := os.WriteFile(reqPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	// Second call without overwrite: requirements.md is skipped.
	res, err := MaterializeSpec(dir, feature, false)
	if err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	if !slices.Contains(res.Skipped, "requirements.md") {
		t.Errorf("expected requirements.md to be skipped, got Skipped=%v Written=%v", res.Skipped, res.Written)
	}
	body, _ := os.ReadFile(reqPath)
	if string(body) != string(custom) {
		t.Errorf("operator content was overwritten without --overwrite: %s", body)
	}
}

func TestMaterializeSpec_OverwriteForcesReplace(t *testing.T) {
	dir := t.TempDir()
	feature := &Feature{ID: 1, Name: "x"}
	if _, err := MaterializeSpec(dir, feature, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	reqPath := filepath.Join(dir, "specs", "x", "requirements.md")
	if err := os.WriteFile(reqPath, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeSpec(dir, feature, true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	body, _ := os.ReadFile(reqPath)
	if string(body) == "OLD" {
		t.Error("expected requirements.md to be replaced when overwrite=true")
	}
}

func TestMaterializeSpec_RejectsEmptyName(t *testing.T) {
	_, err := MaterializeSpec(t.TempDir(), &Feature{ID: 1, Name: ""}, false)
	if err == nil {
		t.Error("expected error when feature.Name is empty")
	}
}

func TestMaterializeSpec_RejectsNilFeature(t *testing.T) {
	_, err := MaterializeSpec(t.TempDir(), nil, false)
	if err == nil {
		t.Error("expected error for nil feature")
	}
}

func TestMaterializeSpec_CompleteAfterCall(t *testing.T) {
	// Round-trip: SpecComplete reports false before MaterializeSpec
	// runs and true after.
	dir := t.TempDir()
	if SpecComplete(dir, "x") {
		t.Fatal("pre-condition broken: spec should not be complete")
	}
	if _, err := MaterializeSpec(dir, &Feature{ID: 1, Name: "x"}, false); err != nil {
		t.Fatal(err)
	}
	if !SpecComplete(dir, "x") {
		t.Error("SpecComplete should return true after MaterializeSpec")
	}
}
