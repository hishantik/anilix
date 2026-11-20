package update

import "testing"

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
