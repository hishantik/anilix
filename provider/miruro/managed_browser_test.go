package miruro

import (
	"strings"
	"testing"
)

func TestBrowserExecutableCandidatesWindowsIncludesChromeAndEdge(t *testing.T) {
	candidates := browserExecutableCandidates("windows", `C:\Program Files`, `C:\Program Files (x86)`, `C:\Users\Test\AppData\Local`)
	joined := strings.ToLower(strings.Join(candidates, "\n"))
	if !strings.Contains(joined, `google\chrome\application\chrome.exe`) {
		t.Fatalf("Chrome missing from candidates: %v", candidates)
	}
	if !strings.Contains(joined, `microsoft\edge\application\msedge.exe`) {
		t.Fatalf("Edge missing from candidates: %v", candidates)
	}
}

func TestBrowserExecutableCandidatesAndroidIsEmpty(t *testing.T) {
	if got := browserExecutableCandidates("android", "", "", ""); len(got) != 0 {
		t.Fatalf("candidates = %v, want none", got)
	}
}
