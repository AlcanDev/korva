package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsKnownCIProvider(t *testing.T) {
	for _, p := range AllCIProviders {
		if !IsKnownCIProvider(p) {
			t.Errorf("IsKnownCIProvider(%q) = false, want true", p)
		}
	}
	if IsKnownCIProvider(CIProvider("circleci")) {
		t.Error("circleci should not be known yet")
	}
}

func TestInstallCI_GitHubActionsCreatesWorkflow(t *testing.T) {
	dir := t.TempDir()
	res, err := InstallCI(dir, CIGitHubActions, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if res.Provider != CIGitHubActions {
		t.Errorf("provider = %q", res.Provider)
	}
	if len(res.Written) == 0 {
		t.Errorf("Written should report at least one file")
	}
	dest := filepath.Join(dir, ".github", "workflows", "harness.yml")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("workflow not on disk: %v", err)
	}
	body, _ := os.ReadFile(dest)
	// Sanity: the GitHub Actions ${{ … }} expressions survived and were
	// not eaten by the Go template engine. If they were, the file would
	// either fail to write or contain {{ … }} with no $ prefix.
	for _, want := range []string{
		"${{ github.token }}",
		"${{ github.event.pull_request.number }}",
		"korva harness check --json",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("workflow missing %q\n--- got ---\n%s", want, string(body))
		}
	}
}

func TestInstallCI_GitLabCreatesYAML(t *testing.T) {
	dir := t.TempDir()
	res, err := InstallCI(dir, CIGitLab, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	dest := filepath.Join(dir, ".gitlab-ci.harness.yml")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("gitlab yml not on disk: %v", err)
	}
	if len(res.Written) == 0 {
		t.Errorf("Written empty: %+v", res)
	}
	body, _ := os.ReadFile(dest)
	for _, want := range []string{
		"$CI_PIPELINE_SOURCE",
		"merge_request_event",
		"korva harness check --json",
		"KORVA_GITLAB_TOKEN",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("gitlab yml missing %q\n--- got ---\n%s", want, string(body))
		}
	}
}

func TestInstallCI_RejectsUnknownProvider(t *testing.T) {
	_, err := InstallCI(t.TempDir(), CIProvider("circleci"), false)
	if err == nil || !strings.Contains(err.Error(), "unknown CI provider") {
		t.Errorf("expected unknown-provider error, got %v", err)
	}
}

func TestInstallCI_RequiresRoot(t *testing.T) {
	_, err := InstallCI("", CIGitHubActions, false)
	if err == nil || !strings.Contains(err.Error(), "root is required") {
		t.Errorf("expected root-required error, got %v", err)
	}
}

func TestInstallCI_IdempotentByDefault(t *testing.T) {
	// Operator-edited workflows must survive a re-run. Idempotent =
	// existing files appear in Skipped, body untouched.
	dir := t.TempDir()
	if _, err := InstallCI(dir, CIGitHubActions, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	dest := filepath.Join(dir, ".github", "workflows", "harness.yml")
	if err := os.WriteFile(dest, []byte("OPERATOR-EDITED"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := InstallCI(dir, CIGitHubActions, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(res.Skipped) == 0 {
		t.Errorf("expected the workflow file to land in Skipped on re-run, got %+v", res)
	}
	body, _ := os.ReadFile(dest)
	if string(body) != "OPERATOR-EDITED" {
		t.Errorf("operator content was overwritten: %s", body)
	}
}

func TestInstallCI_OverwriteFlagReplaces(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallCI(dir, CIGitHubActions, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	dest := filepath.Join(dir, ".github", "workflows", "harness.yml")
	if err := os.WriteFile(dest, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallCI(dir, CIGitHubActions, true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	body, _ := os.ReadFile(dest)
	if string(body) == "OLD" {
		t.Error("workflow should have been replaced when overwrite=true")
	}
}

func TestJoinCIProviders(t *testing.T) {
	got := joinCIProviders()
	for _, p := range AllCIProviders {
		if !strings.Contains(got, string(p)) {
			t.Errorf("joinCIProviders missing %q in %q", p, got)
		}
	}
}
