package browser

import "testing"

func TestCommandForOS(t *testing.T) {
	tests := []struct {
		goos string
		name string
	}{
		{"windows", "rundll32"},
		{"darwin", "open"},
		{"linux", "xdg-open"},
	}
	for _, test := range tests {
		name, _, err := commandForOS(test.goos, "https://example.com")
		if err != nil {
			t.Fatalf("commandForOS(%q): %v", test.goos, err)
		}
		if name != test.name {
			t.Fatalf("commandForOS(%q) = %q, want %q", test.goos, name, test.name)
		}
	}
}

func TestCommandForOSRejectsUnsupportedPlatform(t *testing.T) {
	if _, _, err := commandForOS("plan9", "https://example.com"); err == nil {
		t.Fatal("commandForOS succeeded on unsupported platform")
	}
}
