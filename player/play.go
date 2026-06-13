package player

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed ani-skip.lua
var aniSkipLuaScript string

//go:embed pos-save.lua
var posSaveLuaScript string

type Player struct {
	Name string
}

// SkipInterval represents a skip segment (intro/outro) from AniSkip.
type SkipInterval struct {
	Start float64
	End   float64
	Type  string // "op" or "ed"
}

type Options struct {
	Title      string
	Subtitles  []string
	Referrer   string
	SkipTimes  []SkipInterval
	StartPos   float64 // resume position in seconds (0 = start from beginning)
	PosKey     string  // key for persisting playback position (e.g. "anime_title_ep_11")
}

var (
	Mpv        = &Player{Name: "mpv"}
	Vlc        = &Player{Name: "vlc"}
	Iina       = &Player{Name: "iina"}
	MpvAndroid = &Player{Name: "mpv-android"}
)

func (p *Player) String() string {
	return p.Name
}

func (p *Player) Launch(url string, opts Options) error {
	var args []string

	switch p.Name {
	case "mpv":
		args = p.mpvArgs(url, opts)
	case "vlc":
		args = p.vlcArgs(url, opts)
	case "iina":
		args = p.iinaArgs(url, opts)
	case "mpv-android":
		args = p.mpvAndroidArgs(url, opts)
	case "vlc-android":
		args = p.vlcAndroidArgs(url, opts)
	default:
		return fmt.Errorf("unknown player: %s", p.Name)
	}

	if p.Name == "mpv-android" || p.Name == "vlc-android" {
		// Start local proxy so the player can fetch via localhost
		localURL, stop, err := StartProxy(url, opts.Referrer)
		if err != nil {
			return fmt.Errorf("proxy start failed: %w", err)
		}
		// Keep proxy alive in background for 30 min
		go func() {
			time.Sleep(30 * time.Minute)
			stop()
		}()

		// Wait for proxy to be ready
		proxyAddr := strings.TrimPrefix(strings.TrimSuffix(localURL, "/video"), "http://")
		if err := waitForListen(proxyAddr, 2*time.Second); err != nil {
			stop()
			return fmt.Errorf("proxy not ready: %w", err)
		}

		// Replace -d URL arg with local proxy URL
		for i, a := range args {
			if a == "-d" && i+1 < len(args) {
				args[i+1] = localURL
				break
			}
		}

		// Try am start with -n flag first (works on real Android am)
		cmd := exec.Command("am", args...)
		_, err = cmd.CombinedOutput()
		if err != nil {
			// Fallback: try without -n (Termux am wrapper doesn't support -n)
			argsNoN := removeFlag(args, "-n", 1)
			cmd2 := exec.Command("am", argsNoN...)
			_, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				return fmt.Errorf("am start failed (both attempts): %w", err2)
			}
		}
		return nil
	}
	return exec.Command(p.Name, args...).Start()
}

// removeFlag removes a flag and its N following values from args.
func removeFlag(args []string, flag string, nValues int) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i += nValues // skip flag and its values
			continue
		}
		result = append(result, args[i])
	}
	return result
}

// waitForListen polls until the given address accepts a TCP connection.
func waitForListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
}

func (p *Player) mpvArgs(url string, opts Options) []string {
	args := []string{}

	if opts.Title != "" {
		args = append(args, "--force-media-title="+opts.Title)
	}

	if opts.Subtitles != nil {
		for _, sub := range opts.Subtitles {
			args = append(args, "--sub-file="+sub)
		}
	}

	if opts.Referrer != "" {
		args = append(args, "--referrer="+opts.Referrer)
	}

	// Resume from saved playback position if available
	if opts.StartPos > 0 {
		args = append(args, fmt.Sprintf("--start=%.1f", opts.StartPos))
	}

	// Save playback position on quit so it can be restored next time
	args = append(args, "--save-position-on-quit")

	if len(opts.SkipTimes) > 0 {
		scriptPath := ensureSkipScript()
		if scriptPath != "" {
			args = append(args, "--script="+scriptPath)
			args = append(args, "--script-opts=ani_skip_times="+formatSkipOpts(opts.SkipTimes))
		}
	}

	// Attach position-save script for resume functionality
	if opts.PosKey != "" {
		posScriptPath := ensurePosSaveScript()
		if posScriptPath != "" {
			args = append(args, "--script="+posScriptPath)
			args = append(args, "--script-opts=ani_pos_key="+opts.PosKey)
		}
	}

	args = append(args, url)
	return args
}

// ensureSkipScript writes the bundled Lua script to ~/.anilix/ani-skip.lua
// if it doesn't already exist. Returns the path, or empty string on failure.
func ensureSkipScript() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[anilix] cannot resolve home dir for skip script: %v\n", err)
		return ""
	}
	dir := filepath.Join(home, ".anilix")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create anilix dir: %v\n", err)
		return ""
	}

	path := filepath.Join(dir, "ani-skip.lua")
	if _, err := os.Stat(path); err == nil {
		return path // already exists
	}

	if err := os.WriteFile(path, []byte(aniSkipLuaScript), 0644); err != nil {
		log.Printf("[anilix] failed to write skip script: %v\n", err)
		return ""
	}
	return path
}

// ensurePosSaveScript writes the bundled position-save Lua script to
// ~/.anilix/pos-save.lua if it doesn't already exist.
func ensurePosSaveScript() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[anilix] cannot resolve home dir for pos-save script: %v\n", err)
		return ""
	}
	dir := filepath.Join(home, ".anilix")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create anilix dir: %v\n", err)
		return ""
	}

	path := filepath.Join(dir, "pos-save.lua")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	if err := os.WriteFile(path, []byte(posSaveLuaScript), 0644); err != nil {
		log.Printf("[anilix] failed to write pos-save script: %v\n", err)
		return ""
	}
	return path
}

// ReadPosition reads the saved playback position for a given key.
// Returns the position in seconds, or 0 if no saved position exists.
func ReadPosition(key string) float64 {
	if key == "" {
		return 0
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	path := filepath.Join(home, ".anilix", "watch-progress", key+".pos")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var pos float64
	if _, err := fmt.Sscanf(string(data), "%f", &pos); err != nil {
		return 0
	}
	return pos
}

// MakePosKey creates a sanitized key for persisting playback position.
func MakePosKey(animeTitle, episodeNum string) string {
	key := strings.ToLower(animeTitle)
	// Replace spaces and special characters with underscores
	var b strings.Builder
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else if c == ' ' || c == '-' {
			b.WriteRune('_')
		}
		// skip other characters
	}
	b.WriteString("_ep_")
	b.WriteString(episodeNum)
	return b.String()
}

// formatSkipOpts encodes skip intervals into mpv script-opts format.
// Uses # as interval separator (mpv treats : and , as delimiters).
// Result: "op=87.5-118.2#ed=1340.0-1370.5"
func formatSkipOpts(intervals []SkipInterval) string {
	var parts []string
	for _, iv := range intervals {
		parts = append(parts, fmt.Sprintf("%s=%.1f-%.1f", iv.Type, iv.Start, iv.End))
	}
	return strings.Join(parts, "#")
}

func (p *Player) vlcArgs(url string, opts Options) []string {
	args := []string{}

	if opts.Referrer != "" {
		args = append(args, "--http-referrer="+opts.Referrer)
	}

	if opts.Title != "" {
		args = append(args, "--meta-title="+opts.Title)
	}

	args = append(args, url)
	return args
}

func (p *Player) iinaArgs(url string, opts Options) []string {
	args := []string{"--no-playlist"}

	if opts.Title != "" {
		args = append(args, "--force-media-title="+opts.Title)
	}

	args = append(args, url)
	return args
}

func (p *Player) mpvAndroidArgs(url string, opts Options) []string {
	// Install skip script and write config for AniSkip support
	if len(opts.SkipTimes) > 0 {
		ensureAndroidSkipScript()
		writeAndroidSkipConfig(opts.SkipTimes)
	}

	// Termux am wrapper supports -a, -d, -t, --es, --user but NOT -n.
	// Use -a with component specified via -d intent data or package manager.
	// We try -n first (works on real am), fallback handled in Launch().
	args := []string{
		"start",
		"--user", "0",
		"-a", "android.intent.action.VIEW",
		"-t", "video/*",
		"-d", url,
		"-n", "is.xyz.mpv/.MPVActivity",
	}
	if opts.Title != "" {
		args = append(args, "--es", "title", opts.Title)
	}
	return args
}

// ensureAndroidSkipScript installs the persistent Lua skip script to
// mpv-android's scripts directory so it auto-loads on every start.
func ensureAndroidSkipScript() {
	dir := "/data/data/is.xyz.mpv/files/scripts"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create mpv scripts dir: %v\n", err)
		return
	}

	path := filepath.Join(dir, "anilix-skip.lua")
	if _, err := os.Stat(path); err == nil {
		return // already installed
	}
	if err := os.WriteFile(path, []byte(aniSkipLuaScript), 0644); err != nil {
		log.Printf("[anilix] failed to write android skip script: %v\n", err)
		return
	}
}

// writeAndroidSkipConfig writes skip times to a config file that the
// persistent Lua script reads on each file-loaded event.
func writeAndroidSkipConfig(skipTimes []SkipInterval) {
	dir := "/data/data/is.xyz.mpv/files"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create mpv config dir: %v\n", err)
		return
	}

	path := filepath.Join(dir, "anilix-skip.conf")
	content := formatSkipOpts(skipTimes)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("[anilix] failed to write skip config: %v\n", err)
	}
}

func (p *Player) vlcAndroidArgs(url string, opts Options) []string {
	mimeType := "video/*"
	lower := strings.ToLower(url)
	if strings.HasSuffix(lower, ".m3u8") || strings.Contains(lower, ".m3u8?") {
		mimeType = "application/x-mpegURL"
	}
	args := []string{
		"start",
		"--user", "0",
		"-a", "android.intent.action.VIEW",
		"-t", mimeType,
		"-d", url,
		"-n", "org.videolan.vlc/.gui.video.VideoPlayerActivity",
	}
	if opts.Title != "" {
		args = append(args, "--es", "title", opts.Title)
	}
	if opts.Referrer != "" {
		args = append(args, "--es", "http-referrer", opts.Referrer)
	}
	return args
}

// FromString returns a Player from name string
func FromString(name string) *Player {
	switch strings.ToLower(name) {
	case "mpv":
		return Mpv
	case "vlc":
		return Vlc
	case "iina":
		return Iina
	case "mpv-android":
		return MpvAndroid
	case "vlc-android":
		return VlcAndroid
	default:
		return Mpv // default to mpv
	}
}