package player

import (
	"testing"
)

func TestDetectorDetect(t *testing.T) {
	d := &Detector{}
	players := d.Detect()

	if len(players) == 0 {
		t.Log("No players detected (expected in test environment)")
	}

	for _, name := range players {
		if name != "mpv" && name != "vlc" && name != "iina" && name != "mpv-android" {
			t.Errorf("Unexpected player detected: %s", name)
		}
	}
}

func TestDetectorPreferred(t *testing.T) {
	d := &Detector{}
	preferred := d.Preferred()

	if preferred == nil {
		t.Error("Preferred() returned nil")
	}

	validNames := map[string]bool{"mpv": true, "vlc": true, "iina": true, "mpv-android": true}
	if !validNames[preferred.Name] {
		t.Errorf("Preferred() returned invalid player: %s", preferred.Name)
	}
}
