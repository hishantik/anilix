package player

import (
	"os/exec"
	"runtime"
)

type Detector struct{}

// IsAndroid returns true if running on an Android environment.
func IsAndroid() bool {
	return runtime.GOOS == "android"
}

var (
	VlcAndroid = &Player{Name: "vlc-android"}
)

func (d *Detector) Detect() []string {
	if IsAndroid() {
		players := []string{}
		if isMpvAndroidInstalled() {
			players = append(players, "mpv-android")
		}
		if isVlcAndroidInstalled() {
			players = append(players, "vlc-android")
		}
		if len(players) == 0 {
			return nil
		}
		return players
	}

	players := []string{"mpv", "vlc"}
	if runtime.GOOS == "darwin" {
		players = append(players, "iina")
	}

	var installed []string
	for _, name := range players {
		if IsAvailable(name) {
			installed = append(installed, name)
		}
	}

	return installed
}

func (d *Detector) Preferred() *Player {
	if IsAndroid() {
		if isMpvAndroidInstalled() {
			return MpvAndroid
		}
		if isVlcAndroidInstalled() {
			return VlcAndroid
		}
		return MpvAndroid // fallback, will fail on Launch
	}

	order := []string{"mpv", "vlc"}
	if runtime.GOOS == "darwin" {
		order = []string{"mpv", "vlc", "iina"}
	}

	for _, name := range order {
		if IsAvailable(name) {
			return FromString(name)
		}
	}

	return Mpv
}

func (d *Detector) PreferredForReferrer(needsReferrer bool) *Player {
	if IsAndroid() {
		if !needsReferrer && isMpvAndroidInstalled() {
			return MpvAndroid
		}
		if isVlcAndroidInstalled() {
			return VlcAndroid
		}
		if isMpvAndroidInstalled() {
			return MpvAndroid
		}
		return MpvAndroid
	}
	return d.Preferred()
}

func IsAvailable(name string) bool {
	cmd := exec.Command(name, "--version")
	return cmd.Run() == nil
}

func isMpvAndroidInstalled() bool {
	cmd := exec.Command("pm", "list", "packages", "is.xyz.mpv")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func isVlcAndroidInstalled() bool {
	cmd := exec.Command("pm", "list", "packages", "org.videolan.vlc")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}
