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

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
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
	// artwork.renderer: auto | kitty | sixel | iterm2 | halfblock | chafa | ascii | none
	v.SetDefault("artwork.renderer", "auto")
	v.SetDefault("ui.sidebar", "expanded")
}

func setPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	configDir := filepath.Join(home, "."+AppName)
	v.AddConfigPath(configDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
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

// Save persists the current config to disk.
func Save() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[anilix] cannot resolve home dir: %v\n", err)
		return
	}
	dir := filepath.Join(home, "."+AppName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create config dir: %v\n", err)
		return
	}
	path := filepath.Join(dir, AppName+".toml")
	if err := v.WriteConfigAs(path); err != nil {
		log.Printf("[anilix] failed to save config: %v\n", err)
	}
}

// HistoryPath returns the path to the history file
func HistoryPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "~"
	}
	return filepath.Join(home, "."+AppName, "history.json")
}
