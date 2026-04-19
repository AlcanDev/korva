package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/version"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for a newer version of Korva",
	Long: `Checks the GitHub Releases API for the latest Korva version and
compares it to the version currently installed.

No automatic download is performed — you are shown the upgrade command
to run if a newer version is available.`,
	RunE: runUpdate,
}

func runUpdate(_ *cobra.Command, _ []string) error {
	current := version.Version
	fmt.Printf("Current version: %s\n", current)
	fmt.Printf("Checking https://github.com/AlcanDev/korva/releases …\n\n")

	latest, releaseURL, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if current == "dev" {
		printInfo(fmt.Sprintf("Running a development build — latest release is %s", latest))
		return nil
	}

	if normalizeTag(latest) == normalizeTag(current) {
		printSuccess(fmt.Sprintf("You are up to date (%s)", current))
		return nil
	}

	fmt.Printf("  → New version available: %s\n", latest)
	fmt.Printf("  Release notes: %s\n\n", releaseURL)
	fmt.Println("Upgrade:")
	switch runtime.GOOS {
	case "darwin", "linux":
		fmt.Println("  # Homebrew")
		fmt.Println("  brew upgrade alcandev/tap/korva")
		fmt.Println("")
		fmt.Println("  # Shell installer")
		fmt.Println("  curl -fsSL https://korva.dev/install.sh | bash")
	case "windows":
		fmt.Println("  # PowerShell installer")
		fmt.Println("  irm https://korva.dev/install.ps1 | iex")
	default:
		fmt.Println("  curl -fsSL https://korva.dev/install.sh | bash")
	}
	return nil
}

// fetchLatestRelease queries the GitHub Releases API and returns the latest
// tag name and the HTML URL of the release.
func fetchLatestRelease() (tag, htmlURL string, err error) {
	const apiURL = "https://api.github.com/repos/AlcanDev/korva/releases/latest"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("korva-cli/%s (%s/%s)", version.Version, runtime.GOOS, runtime.GOARCH))

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("no releases found (repository may be empty)")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("decoding GitHub response: %w", err)
	}
	if release.TagName == "" {
		return "", "", fmt.Errorf("unexpected response: missing tag_name")
	}
	return release.TagName, release.HTMLURL, nil
}

// normalizeTag strips a leading "v" so "v1.2.3" and "1.2.3" compare equal.
func normalizeTag(s string) string {
	return strings.TrimPrefix(s, "v")
}
