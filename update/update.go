package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
func IsUpdateAvailable(currentVersion, latestTag string) bool {
	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(latestTag, "v")
	return current != latest
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
