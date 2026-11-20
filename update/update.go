package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hishantik/anilix/curl"
)

const (
	repo    = "hishantik/anilix"
	apiURL  = "https://api.github.com/repos/" + repo + "/releases/latest"
	baseURL = "https://github.com/" + repo + "/releases/download"
)

// ReleaseInfo holds the latest release metadata from GitHub.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []Asset
}

// Asset is a single release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdate queries GitHub for the latest release.
func CheckForUpdate(ctx context.Context) (*ReleaseInfo, error) {
	headers := map[string]string{
		"Accept": "application/vnd.github+json",
	}

	body, err := curl.Get(ctx, apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to check GitHub releases: %w", err)
	}

	var release ReleaseInfo
	if err := json.Unmarshal([]byte(body), &release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	if release.TagName == "" {
		return nil, fmt.Errorf("no release found")
	}

	return &release, nil
}

// IsUpdateAvailable compares the current version with the latest tag.
// Numeric-aware semver comparison: a string equality check marked every
// dev build and every v2.0-vs-v2.0.0 pair as outdated forever.
func IsUpdateAvailable(currentVersion, latestTag string) bool {
	return compareVersions(currentVersion, latestTag) < 0
}

// compareVersions parses "v1.2.3" into numeric parts and compares them.
// Non-numeric parts (e.g. "dev") compare as unknown: -2 sorts below any
// release so dev builds are treated as older, matching the old intent
// (a dev build should update) without mismatch spam between formats.
func compareVersions(a, b string) int {
	pa, oka := versionParts(a)
	pb, okb := versionParts(b)
	if !oka && !okb {
		return 0 // dev vs dev, or dev vs unknown: no update prompt
	}
	if !oka {
		return -2
	}
	if !okb {
		return 2
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// versionParts returns [major, minor, patch] and whether the string looked
// like a version at all.
func versionParts(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var nums [3]int
	if len(parts) == 0 || parts[0] == "" {
		return nums, false
	}
	ok := true
	for i := 0; i < 3; i++ {
		if i < len(parts) {
			n, err := strconv.Atoi(parts[i])
			if err != nil {
				ok = false
				break
			}
			nums[i] = n
		}
	}
	return nums, ok
}

// PerformUpdate downloads and installs the latest release binary.
func PerformUpdate(ctx context.Context, release *ReleaseInfo) error {
	assetName := assetNameForPlatform()
	if assetName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no asset found for %s in release %s", assetName, release.TagName)
	}

	// Find current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	// Create temp directory for download
	tmpDir, err := os.MkdirTemp("", "anilix-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)

	// Download archive
	dlCmd := exec.CommandContext(ctx, "curl", "-fsSL", "-o", archivePath, downloadURL)
	if output, err := dlCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("download failed: %s: %w", string(output), err)
	}

	// Verify the archive against the release's checksums.txt before it is
	// ever executed: a tampered release must not install.
	if err := verifyChecksum(ctx, tmpDir, release.TagName, assetName, archivePath); err != nil {
		return err
	}

	// Extract binary from archive
	binaryName := "anilix"
	if runtime.GOOS == "windows" {
		binaryName = "anilix.exe"
	}

	extractedPath := filepath.Join(tmpDir, binaryName)

	if strings.HasSuffix(assetName, ".zip") {
		unzipCmd := exec.CommandContext(ctx, "unzip", "-o", archivePath, binaryName, "-d", tmpDir)
		if output, err := unzipCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("unzip failed: %s: %w", string(output), err)
		}
	} else {
		tarCmd := exec.CommandContext(ctx, "tar", "xzf", archivePath, "-C", tmpDir, binaryName)
		if output, err := tarCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extract failed: %s: %w", string(output), err)
		}
	}

	// Verify extracted binary exists
	if _, err := os.Stat(extractedPath); err != nil {
		return fmt.Errorf("extracted binary not found at %s", extractedPath)
	}

	// Replace current binary
	// On some systems (PRoot), os.Rename across filesystems may fail.
	// Use copy + rename as fallback.
	if err := os.Rename(extractedPath, exePath); err != nil {
		// Fallback: copy bytes
		if err := copyFile(extractedPath, exePath); err != nil {
			return fmt.Errorf("cannot replace binary: %w", err)
		}
	}

	// Ensure executable permissions
	if err := os.Chmod(exePath, 0755); err != nil {
		return fmt.Errorf("cannot set permissions: %w", err)
	}

	return nil
}

// verifyChecksum downloads the release's checksums.txt and compares the
// sha256 of the downloaded archive against it.
func verifyChecksum(ctx context.Context, tmpDir, tag, assetName, archivePath string) error {
	sumsURL := baseURL + "/download/" + tag + "/checksums.txt"
	sumsPath := filepath.Join(tmpDir, "checksums.txt")
	sumCmd := exec.CommandContext(ctx, "curl", "-fsSL", "-o", sumsPath, sumsURL)
	if output, err := sumCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checksum download failed: %s: %w", string(output), err)
	}

	want, err := sha256Of(sumsPath, assetName)
	if err != nil {
		return fmt.Errorf("no checksum for %s: %w", assetName, err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	got := sha256.Sum256(data)
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: update aborted", assetName)
	}
	return nil
}

// sha256Of returns the recorded sha256 of name from a checksums.txt file.
func sha256Of(sumsPath, name string) ([32]byte, error) {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return [32]byte{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && filepath.Base(fields[1]) == name {
			sum, err := hex.DecodeString(fields[0])
			if err == nil && len(sum) == sha256.Size {
				return [32]byte(sum), nil
			}
		}
	}
	return [32]byte{}, fmt.Errorf("not listed in checksums.txt")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(in)
	return err
}

func assetNameForPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Detect Termux by checking PREFIX environment variable
	if isTermux() {
		return fmt.Sprintf("anilix_termux_%s.tar.gz", arch)
	}

	switch osName {
	case "windows":
		return fmt.Sprintf("anilix_%s_%s.zip", osName, arch)
	case "linux", "darwin":
		return fmt.Sprintf("anilix_%s_%s.tar.gz", osName, arch)
	default:
		return ""
	}
}

func isTermux() bool {
	prefix := os.Getenv("PREFIX")
	return strings.Contains(prefix, "com.termux")
}
