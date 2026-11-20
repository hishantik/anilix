package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	AppName = "anilix"
)

var v = viper.New()

func Setup() error {
	setName()
	setFs()
	setEnvs()
	setDefaults()
	setPaths()
	// The config holds the AniList OAuth token: only the owner may read it.
	v.SetConfigPermissions(0600)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// Viper applies the mode only on create: an existing 0644 file from an
	// older version keeps its loose permissions until chmod'ed here.
	if path := v.ConfigFileUsed(); path != "" {
		if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
			log.Printf("[anilix] failed to tighten config permissions: %v\n", err)
		}
	}

	return nil
}

func setName() {
	v.SetConfigName(AppName)
	v.SetConfigType("toml")
}

func setFs() {
	// Intentionally empty — afero.NewOsFs() is the default.
	// Previously had v.SetFs(nil) which crashes on PRoot/Android.
}

func setEnvs() {
	v.SetEnvPrefix(AppName)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func setDefaults() {
	v.SetDefault("player", "mpv")
	v.SetDefault("quality", "auto")
	v.SetDefault("source", "")
	v.SetDefault("history.enabled", true)
	v.SetDefault("aniskip.enabled", true)
	v.SetDefault("anilist.tracking.enabled", true)
	// artwork.renderer: auto | kitty | iterm2 | halfblock | chafa | ascii | none
	v.SetDefault("artwork.renderer", "auto")
	v.SetDefault("ui.sidebar", "expanded")
}

func setPaths() {
	// The home fallback below produces a usable path in the CWD rather
	// than a literal "~" that no shell expands.
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	configDir := filepath.Join(home, "."+AppName)
	v.AddConfigPath(configDir)

	if err := os.MkdirAll(configDir, 0700); err != nil {
		log.Printf("[anilix] failed to create config dir: %v\n", err)
	}
}

func Get(key string) interface{} {
	return v.Get(key)
}

func GetString(key string) string {
	return v.GetString(key)
}

func GetBool(key string) bool {
	return v.GetBool(key)
}

func Set(key string, value interface{}) {
	v.Set(key, value)
	Save()
}

// Save persists the current config to disk atomically: a crash mid-write
// must not truncate the file that holds the OAuth token.
func Save() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[anilix] cannot resolve home dir: %v\n", err)
		return
	}
	dir := filepath.Join(home, "."+AppName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("[anilix] failed to create config dir: %v\n", err)
		return
	}
	path := filepath.Join(dir, AppName+".toml")
	tmp := path + ".tmp"
	if err := v.WriteConfigAs(tmp); err != nil {
		log.Printf("[anilix] failed to save config: %v\n", err)
		os.Remove(tmp)
		return
	}
	// viper applies the mode only on create; keep an existing file at 0600.
	if err := os.Chmod(tmp, 0600); err != nil {
		log.Printf("[anilix] failed to tighten config permissions: %v\n", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("[anilix] failed to replace config: %v\n", err)
		os.Remove(tmp)
	}
}

// HistoryPath returns the path to the history file
func HistoryPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, "."+AppName, "history.json")
}
