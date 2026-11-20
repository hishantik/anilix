package update

import (
	"encoding/hex"
	"os"
	"path/filepath"
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
