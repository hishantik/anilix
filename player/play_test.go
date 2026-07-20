package player

import (
	"errors"
	"testing"
)

func TestDesktopLaunchReturnsPlayerExitError(t *testing.T) {
	want := errors.New("decoder rejected stream")
	player := &Player{Name: "mpv", run: func(string, ...string) error { return want }}
	if err := player.Launch("https://example.com/video.m3u8", Options{}); !errors.Is(err, want) {
		t.Fatalf("Launch error = %v, want %v", err, want)
	}
}

func TestPlayerString(t *testing.T) {
	tests := []struct {
		name     string
		player   *Player
		expected string
	}{
		{"mpv", Mpv, "mpv"},
		{"vlc", Vlc, "vlc"},
		{"iina", Iina, "iina"},
		{"mpv-android", MpvAndroid, "mpv-android"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.player.String() != tt.expected {
				t.Errorf("Player.String() = %v, want %v", tt.player.String(), tt.expected)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"mpv lowercase", "mpv", "mpv"},
		{"MPV uppercase", "MPV", "mpv"},
		{"vlc", "vlc", "vlc"},
		{"iina", "iina", "iina"},
		{"mpv-android", "mpv-android", "mpv-android"},
		{"MPV-ANDROID uppercase", "MPV-ANDROID", "mpv-android"},
		{"unknown defaults to mpv", "unknown", "mpv"},
		{"empty defaults to mpv", "", "mpv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := FromString(tt.input)
			if p.Name != tt.expected {
				t.Errorf("FromString(%q) = %v, want %v", tt.input, p.Name, tt.expected)
			}
		})
	}
}

func TestMpvArgs(t *testing.T) {
	p := &Player{Name: "mpv"}
	url := "https://example.com/video.m3u8"

	tests := []struct {
		name    string
		opts    Options
		wantURL bool
	}{
		{"basic", Options{}, true},
		{"with title", Options{Title: "Test Anime EP1"}, true},
		{"with referrer", Options{Referrer: "https://example.com"}, true},
		{"with subtitles", Options{Subtitles: []string{"sub1.vtt", "sub2.vtt"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := p.mpvArgs(url, tt.opts)
			found := false
			for _, arg := range args {
				if arg == url {
					found = true
					break
				}
			}
			if !found && tt.wantURL {
				t.Errorf("mpvArgs() missing URL in args: %v", args)
			}
		})
	}
}

func TestVlcArgs(t *testing.T) {
	p := &Player{Name: "vlc"}
	url := "https://example.com/video.m3u8"

	args := p.vlcArgs(url, Options{
		Title:    "Test",
		Referrer: "https://referrer.com",
	})

	foundTitle := false
	foundReferrer := false
	foundURL := false

	for _, arg := range args {
		if arg == "--meta-title=Test" {
			foundTitle = true
		}
		if arg == "--http-referrer=https://referrer.com" {
			foundReferrer = true
		}
		if arg == url {
			foundURL = true
		}
	}

	if !foundTitle {
		t.Error("vlcArgs() missing title arg")
	}
	if !foundReferrer {
		t.Error("vlcArgs() missing referrer arg")
	}
	if !foundURL {
		t.Error("vlcArgs() missing URL")
	}
}

func TestMpvAndroidArgs(t *testing.T) {
	p := &Player{Name: "mpv-android"}
	url := "https://example.com/video.m3u8"

	tests := []struct {
		name      string
		opts      Options
		wantTitle bool
	}{
		{"basic", Options{}, false},
		{"with referrer (ignored)", Options{Referrer: "https://example.com"}, false},
		{"with title", Options{Title: "Naruto - Episode 1"}, true},
		{"full", Options{Title: "Naruto - Episode 1", Referrer: "https://example.com"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := p.mpvAndroidArgs(url, tt.opts)

			// Verify am start structure
			if args[0] != "start" {
				t.Errorf("first arg should be 'start', got %q", args[0])
			}

			// Verify --user 0
			foundUser := false
			for i, arg := range args {
				if arg == "--user" && i+1 < len(args) && args[i+1] == "0" {
					foundUser = true
					break
				}
			}
			if !foundUser {
				t.Errorf("mpvAndroidArgs() missing --user 0 in args: %v", args)
			}

			// Verify action
			foundAction := false
			for i, arg := range args {
				if arg == "-a" && i+1 < len(args) && args[i+1] == "android.intent.action.VIEW" {
					foundAction = true
					break
				}
			}
			if !foundAction {
				t.Errorf("mpvAndroidArgs() missing -a android.intent.action.VIEW in args: %v", args)
			}

			// Verify data URI
			foundURL := false
			for i, arg := range args {
				if arg == "-d" && i+1 < len(args) && args[i+1] == url {
					foundURL = true
					break
				}
			}
			if !foundURL {
				t.Errorf("mpvAndroidArgs() missing -d %q in args: %v", url, args)
			}

			// Verify activity component
			foundComponent := false
			for i, arg := range args {
				if arg == "-n" && i+1 < len(args) && args[i+1] == "is.xyz.mpv/.MPVActivity" {
					foundComponent = true
					break
				}
			}
			if !foundComponent {
				t.Errorf("mpvAndroidArgs() missing -n is.xyz.mpv/.MPVActivity in args: %v", args)
			}

			// Verify referrer is NOT included (mpv-android doesn't support it)
			for i, arg := range args {
				if arg == "--es" && i+2 < len(args) && args[i+1] == "referrer" {
					t.Errorf("mpvAndroidArgs() should not include referrer extra (mpv-android ignores it)")
					break
				}
			}

			// Verify title
			if tt.wantTitle {
				found := false
				for i, arg := range args {
					if arg == "--es" && i+2 < len(args) && args[i+1] == "title" && args[i+2] == "Naruto - Episode 1" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("mpvAndroidArgs() missing --es title extra")
				}
			}
		})
	}
}
