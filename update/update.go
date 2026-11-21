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

	// Replace current binary.
	//
	// The installer (install.sh detect_install_dir) installs without sudo
	// only when the target dir is user-writable, preferring /usr/local/bin
	// and falling back to ~/.local/bin. An install that went through sudo
	// leaves a root-owned binary that the updater cannot replace: fail
	// early with the same remedy the installer would use, before wasting
	// the download.
	if err := checkWritable(filepath.Dir(exePath)); err != nil {
		return err
	}
	if err := replaceBinary(extractedPath, exePath); err != nil {
		return fmt.Errorf("cannot replace binary: %w", err)
	}

	// Ensure executable permissions
	if err := os.Chmod(exePath, 0755); err != nil {
		return fmt.Errorf("cannot set permissions: %w", err)
	}

	return nil
}

// checkWritable verifies the install dir accepts a new binary, with an
// actionable error for the common sudo-installed case.
func checkWritable(dir string) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("install directory %s is missing", dir)
	}
	if !isDirWritable(dir) {
		return fmt.Errorf(
			"no write permission to %s (installed with sudo?): re-run install.sh to move anilix to ~/.local/bin, or update with: sudo anilix update",
			dir)
	}
	return nil
}

// isDirWritable reports whether the current user can create files in dir.
// os.Stat Mode is not enough: group/other write bits don't reflect ACLs or
// ownership, so probe with a real file create.
func isDirWritable(dir string) bool {
	probe, err := os.CreateTemp(dir, ".anilix-writable-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return true
}

// replaceBinary atomically swaps src over dst. Rename from /tmp (tmpfs)
// to the install dir (disk) fails with EXDEV, and writing the bytes of a
// running binary directly fails with ETXTBSY on Linux — so copy into a
// temp file beside dst (same filesystem, same dir) and rename over dst.
// The rename leaves the running process on the old inode untouched.
func replaceBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".anilix-new-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.ReadFrom(in); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
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
