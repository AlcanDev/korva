package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/version"
)

var updateFlags struct {
	yes   bool // skip confirmation prompt
	check bool // only check, don't install
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Korva to the latest version",
	Long: `Checks GitHub Releases for the latest Korva version and installs it
in-place, replacing korva, korva-vault, and korva-sentinel in the same
directory as the current binary.

The release archive is verified against the official SHA256 checksums
before any binary is replaced.

Flags:
  --check   Only check for a newer version — do not download or install.
  --yes     Skip the confirmation prompt and install automatically.

Environment:
  KORVA_NO_UPDATE_CHECK=1   Disable automatic update hints at startup.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateFlags.yes, "yes", false, "skip confirmation prompt")
	updateCmd.Flags().BoolVar(&updateFlags.check, "check", false, "only check, do not install")
}

func runUpdate(_ *cobra.Command, _ []string) error {
	current := version.Version
	fmt.Printf("  Current version : %s\n", current)
	fmt.Printf("  Checking        : https://github.com/AlcanDev/korva/releases …\n\n")

	latest, releaseURL, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if current == "dev" {
		printInfo(fmt.Sprintf("Development build — latest release is %s", latest))
		fmt.Printf("  Release notes: %s\n", releaseURL)
		return nil
	}

	if normalizeTag(latest) == normalizeTag(current) {
		printSuccess(fmt.Sprintf("Already up to date (%s)", current))
		return nil
	}

	fmt.Printf("  → New version available: %s\n", latest)
	fmt.Printf("    Release notes: %s\n\n", releaseURL)

	if updateFlags.check {
		printUpgradeHint()
		return nil
	}

	if !updateFlags.yes {
		fmt.Printf("  Install %s → %s? [y/N] ", current, latest)
		var answer string
		fmt.Scanln(&answer) //nolint:errcheck
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			printInfo("Canceled. Run `korva update --yes` to skip this prompt.")
			return nil
		}
	}

	return installUpdate(latest)
}

// installUpdate downloads, verifies, and installs the latest release.
func installUpdate(tag string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ver := normalizeTag(tag)

	// Determine archive name — goreleaser uses lowercase OS, .zip for Windows.
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	archiveName := fmt.Sprintf("korva_%s_%s_%s.%s", ver, goos, goarch, ext)
	checksumName := fmt.Sprintf("korva_%s_checksums.txt", ver)

	baseURL := fmt.Sprintf("https://github.com/AlcanDev/korva/releases/download/%s", tag)
	archiveURL := baseURL + "/" + archiveName
	checksumURL := baseURL + "/" + checksumName

	fmt.Printf("  Downloading %s …\n", archiveName)

	// ── 1. Download checksums ────────────────────────────────────────────────
	checksums, err := downloadText(checksumURL)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}
	expectedHash, ok := parseChecksum(checksums, archiveName)
	if !ok {
		return fmt.Errorf("archive %q not found in checksums file", archiveName)
	}

	// ── 2. Download archive to temp file ─────────────────────────────────────
	tmpDir, err := os.MkdirTemp("", "korva-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(archiveURL, archivePath); err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}

	// ── 3. Verify SHA256 ─────────────────────────────────────────────────────
	fmt.Printf("  Verifying SHA256 …\n")
	actualHash, err := sha256File(archivePath)
	if err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actualHash, expectedHash)
	}
	printSuccess("Checksum OK")

	// ── 4. Extract binaries ───────────────────────────────────────────────────
	fmt.Printf("  Extracting …\n")
	binaries, err := extractBinaries(archivePath, tmpDir, goos)
	if err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	if len(binaries) == 0 {
		return fmt.Errorf("no korva binaries found in archive")
	}

	// ── 5. Locate install directory ───────────────────────────────────────────
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary path: %w", err)
	}
	// Resolve symlinks (e.g. Homebrew cellar symlinks)
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}
	installDir := filepath.Dir(execPath)

	// ── 6. Atomic replace ─────────────────────────────────────────────────────
	fmt.Printf("  Installing to %s …\n", installDir)
	installed := 0
	for name, newPath := range binaries {
		dst := filepath.Join(installDir, name)
		if err := atomicReplace(dst, newPath); err != nil {
			// Not fatal if a sibling binary is missing; korva itself is required.
			if name == "korva" || name == "korva.exe" {
				return fmt.Errorf("replacing %s: %w", name, err)
			}
			fmt.Fprintf(os.Stderr, "  ⚠  Skipping %s: %v\n", name, err)
			continue
		}
		printSuccess(fmt.Sprintf("Updated %s", dst))
		installed++
	}

	if installed == 0 {
		return fmt.Errorf("no binaries could be installed")
	}

	// ── 7. Persist the new version check cache ────────────────────────────────
	_ = saveVersionCache(normalizeTag(tag))

	fmt.Printf("\n  ✓ Korva %s installed successfully!\n\n", tag)
	fmt.Printf("    Run `korva version` to confirm.\n")
	return nil
}

// atomicReplace replaces dst with src atomically.
// On all platforms: write to a .new temp file, then rename into place.
// On Windows: if rename fails (binary in use), leave .new for manual swap.
func atomicReplace(dst, src string) error {
	// If dst doesn't exist yet, just copy directly.
	_, statErr := os.Stat(dst)
	if os.IsNotExist(statErr) {
		return copyFile(src, dst)
	}

	newPath := dst + ".new"
	if err := copyFile(src, newPath); err != nil {
		return err
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		os.Remove(newPath) //nolint:errcheck
		return err
	}

	// Atomic rename. On Windows, if dst is the running binary, this fails —
	// we leave the .new file for the user to rename manually.
	if err := os.Rename(newPath, dst); err != nil {
		if runtime.GOOS == "windows" {
			fmt.Fprintf(os.Stderr,
				"  ⚠  Could not replace %s while it is running.\n     Manual step: rename %s → %s\n",
				dst, newPath, dst)
			return nil // not fatal
		}
		os.Remove(newPath) //nolint:errcheck
		return err
	}
	return nil
}

// extractBinaries extracts korva, korva-vault, and korva-sentinel from
// the archive into destDir. Returns a map of basename → extracted path.
func extractBinaries(archivePath, destDir, goos string) (map[string]string, error) {
	targets := map[string]bool{
		"korva": true, "korva-vault": true, "korva-sentinel": true,
		"korva.exe": true, "korva-vault.exe": true, "korva-sentinel.exe": true,
	}
	result := make(map[string]string)

	if goos == "windows" {
		return extractZipBinaries(archivePath, destDir, targets)
	}
	return extractTarGzBinaries(archivePath, destDir, targets, result)
}

func extractTarGzBinaries(archivePath, destDir string, targets map[string]bool, result map[string]string) (map[string]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(hdr.Name)
		if !targets[name] || hdr.Typeflag != tar.TypeReg {
			continue
		}
		outPath := filepath.Join(destDir, name)
		if err := writeFromReader(outPath, tr, hdr.Size); err != nil {
			return nil, fmt.Errorf("extracting %s: %w", name, err)
		}
		result[name] = outPath
	}
	return result, nil
}

func extractZipBinaries(archivePath, destDir string, targets map[string]bool) (map[string]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	result := make(map[string]string)
	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if !targets[name] || f.FileInfo().IsDir() {
			continue
		}
		outPath := filepath.Join(destDir, name)
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		err = writeFromReader(outPath, rc, int64(f.UncompressedSize64))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("extracting %s: %w", name, err)
		}
		result[name] = outPath
	}
	return result, nil
}

func writeFromReader(dst string, r io.Reader, size int64) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	if size > 0 {
		_, err = io.CopyN(f, r, size)
	} else {
		_, err = io.Copy(f, r)
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// sha256File returns the lowercase hex SHA256 of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// parseChecksum finds the SHA256 hash for filename in a goreleaser checksums file.
// Format: "<hash>  <filename>\n" (two spaces between hash and name).
func parseChecksum(text, filename string) (string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filename {
			return parts[0], true
		}
	}
	return "", false
}

// downloadText fetches a URL and returns its body as a string.
func downloadText(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", fmt.Sprintf("korva-cli/%s", version.Version))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// downloadFile fetches url and writes it to path with a progress dot every 5 MB.
func downloadFile(url, path string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", fmt.Sprintf("korva-cli/%s", version.Version))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Print progress dots — one dot per 5 MB downloaded.
	const dotEvery = 5 * 1024 * 1024
	var written int64
	buf := make([]byte, 32*1024)
	fmt.Printf("    ")
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if written/dotEvery > (written-int64(n))/dotEvery {
				fmt.Printf(".")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	fmt.Println()
	return nil
}

// fetchLatestRelease queries GitHub Releases API.
// Returns the tag name (e.g. "v1.2.3") and the HTML release URL.
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
		return "", "", fmt.Errorf("no releases published yet")
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

// printUpgradeHint prints the platform-appropriate upgrade command.
func printUpgradeHint() {
	fmt.Println("  Upgrade options:")
	switch runtime.GOOS {
	case "darwin", "linux":
		fmt.Println("    brew upgrade alcandev/tap/korva   # Homebrew")
		fmt.Println("    korva update --yes                # in-place self-update")
	case "windows":
		fmt.Println("    irm https://korva.dev/install.ps1 | iex   # PowerShell")
		fmt.Println("    korva update --yes                         # in-place self-update")
	default:
		fmt.Println("    korva update --yes")
	}
}

// ── Startup version cache ─────────────────────────────────────────────────────
// The CLI checks for updates at most once every 24 h. The cached result is
// stored in ~/.korva/version.check (tiny JSON file).

type versionCache struct {
	LastCheck time.Time `json:"last_check"`
	Latest    string    `json:"latest"`
}

func versionCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".korva", "version.check"), nil
}

func loadVersionCache() (*versionCache, error) {
	path, err := versionCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c versionCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveVersionCache(latest string) error {
	path, err := versionCachePath()
	if err != nil {
		return err
	}
	c := versionCache{LastCheck: time.Now(), Latest: latest}
	data, _ := json.Marshal(c)
	return os.WriteFile(path, data, 0o600)
}

// CheckUpdateHint prints a one-line hint to stderr when a newer version exists.
// It reads/writes a 24-h cache at ~/.korva/version.check so it does not spam.
// Called as a goroutine from the CLI root — never blocks the command.
func CheckUpdateHint() {
	if os.Getenv("KORVA_NO_UPDATE_CHECK") == "1" {
		return
	}
	if version.Version == "dev" {
		return
	}

	// Check the cache first — refresh at most once per 24 h.
	if c, err := loadVersionCache(); err == nil {
		if time.Since(c.LastCheck) < 24*time.Hour {
			// Cache is fresh. Print hint if cached latest is newer.
			if c.Latest != "" && normalizeTag(c.Latest) != normalizeTag(version.Version) {
				fmt.Fprintf(os.Stderr, "\n  ℹ  Update available: %s → v%s  —  run `korva update`\n\n",
					version.Version, c.Latest)
			}
			return
		}
	}

	// Cache stale or missing — fetch from GitHub (best-effort, 5 s timeout).
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET",
		"https://api.github.com/repos/AlcanDev/korva/releases/latest", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("korva-cli/%s", version.Version))

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.TagName == "" {
		return
	}

	latest := normalizeTag(release.TagName)
	_ = saveVersionCache(latest)

	if latest != normalizeTag(version.Version) {
		fmt.Fprintf(os.Stderr, "\n  ℹ  Update available: %s → v%s  —  run `korva update`\n\n",
			version.Version, latest)
	}
}
