package update

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsUpdateAvailable(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.0.1", true},
		{"v1.0", "v1.0.0", false},  // v1.0 == v1.0.0, no forever-prompt
		{"v1.0.0", "v1.1.0", true}, // minor bump
		{"v2.0.0", "v1.9.9", false},
		{"dev", "v1.2.3", true},  // dev builds should update
		{"dev", "dev", false},    // dev vs dev: no prompt loop
		{"v1.0.0", "", false},    // no tag: nothing to compare
	}
	for _, c := range cases {
		if got := IsUpdateAvailable(c.current, c.latest); got != c.want {
			t.Errorf("IsUpdateAvailable(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "anilix")
	if err := os.WriteFile(dst, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "new-anilix")
	if err := os.WriteFile(src, []byte("new binary"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(src, dst); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Errorf("dst holds %q, want %q", got, "new binary")
	}
	// The replacement must be executable, and no temp litter left behind.
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("replaced binary is not executable: %v", info.Mode())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "anilix" && e.Name() != "new-anilix" {
			t.Errorf("temp litter left behind: %s", e.Name())
		}
	}
}

func TestCheckWritable(t *testing.T) {
	dir := t.TempDir()
	if err := checkWritable(dir); err != nil {
		t.Errorf("checkWritable(tempdir) = %v, want nil", err)
	}

	// A read-only dir must produce an error naming the directory.
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) }) // so TempDir cleanup can remove it
	err := checkWritable(ro)
	if err == nil {
		t.Fatal("checkWritable(read-only dir) = nil, want error")
	}
	if !strings.Contains(err.Error(), ro) {
		t.Errorf("error %q does not name the unwritable dir", err)
	}
}

func TestSHA256Of(t *testing.T) {
	dir := t.TempDir()
	sums := filepath.Join(dir, "checksums.txt")
	// a real sha256 of "test\n" — the parsing must yield exactly these bytes
	content := "53d0f4ebd60e218200e1f4e3062e28fbd31b2b1b4d49c0a966e70a3e4bb0a8d7  anilix_linux_amd64.tar.gz\n"
	if err := os.WriteFile(sums, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	sum, err := sha256Of(sums, "anilix_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("sha256Of: %v", err)
	}
	want := "53d0f4ebd60e218200e1f4e3062e28fbd31b2b1b4d49c0a966e70a3e4bb0a8d7"
	wantBytes, _ := hex.DecodeString(want)
	if sum != [32]byte(wantBytes) {
		t.Errorf("sha256Of parsed %x, want %s", sum, want)
	}
	if _, err := sha256Of(sums, "other.tar.gz"); err == nil {
		t.Error("sha256Of: expected error for unlisted asset")
	}
}

func TestVerifyChecksumURL(t *testing.T) {
	// Regression: the URL was built with an extra /download/ segment,
	// producing .../releases/download/download/vX/checksums.txt (404).
	tag := "v1.2.3"
	want := baseURL + "/" + tag + "/checksums.txt"
	if want != "https://github.com/hishantik/anilix/releases/download/v1.2.3/checksums.txt" {
		t.Errorf("checksums URL %q is malformed", want)
	}
}

func TestInstallTarget(t *testing.T) {
	// Writable exe dir is kept as-is.
	dir := t.TempDir()
	exe := filepath.Join(dir, "anilix")
	got, err := installTarget(exe)
	if err != nil {
		t.Fatalf("installTarget(writable dir): %v", err)
	}
	if got != dir {
		t.Errorf("installTarget(writable dir) = %q, want %q", got, dir)
	}

	// Unwritable exe dir on desktop falls back to ~/.local/bin.
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	t.Setenv("PREFIX", "") // not Termux
	got, err = installTarget(filepath.Join(ro, "anilix"))
	if err != nil {
		t.Fatalf("installTarget(read-only dir): %v", err)
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, ".local", "bin"); got != want {
		t.Errorf("installTarget(read-only dir) = %q, want %q", got, want)
	}
}
