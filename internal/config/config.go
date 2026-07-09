// Package config loads shiet's app/runtime configuration from layered sources:
// baked-in defaults, an optional YAML file, and SHIET_* environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds typed app/runtime settings. User preferences (window start, AI
// toggles, etc.) live in SQLite — not here.
type Config struct {
	DB struct {
		Path string `koanf:"path"`
	} `koanf:"db"`
	Google struct {
		ClientID     string `koanf:"client_id"`
		ClientSecret string `koanf:"client_secret"`
	} `koanf:"google"`
}

// envKeyMap maps legacy SHIET_* env vars to koanf dotted keys.
var envKeyMap = map[string]string{
	"SHIET_DB":                  "db.path",
	"SHIET_GOOGLE_CLIENT_ID":     "google.client_id",
	"SHIET_GOOGLE_CLIENT_SECRET": "google.client_secret",
}

// Load reads configuration using the standard discovery order:
//
//  1. Defaults (OS user config dir for db.path)
//  2. Config files, when present (first match wins per path; later paths override):
//     - $XDG_CONFIG_HOME/shiet/config.yaml, or ~/.config/shiet/config.yaml
//     - <UserConfigDir>/shiet/config.yaml (e.g. ~/Library/Application Support/shiet on macOS)
//     - ./shiet.yaml in the process working directory
//  3. Environment variables (highest precedence)
//
// A missing config file is fine — defaults and env are enough.
func Load() (Config, error) {
	return load(discoverConfigFiles())
}

func load(configFiles []string) (Config, error) {
	defaultDB, err := defaultDBPath()
	if err != nil {
		return Config{}, err
	}

	k := koanf.New(".")

	if err := k.Load(confmap.Provider(map[string]any{
		"db": map[string]any{
			"path": defaultDB,
		},
	}, "."), nil); err != nil {
		return Config{}, fmt.Errorf("load defaults: %w", err)
	}

	for _, path := range configFiles {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return Config{}, fmt.Errorf("load config file %s: %w", path, err)
		}
	}

	applyEnv(k)

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	expanded, err := expandHome(cfg.DB.Path)
	if err != nil {
		return Config{}, err
	}
	cfg.DB.Path = expanded

	return cfg, nil
}

func discoverConfigFiles() []string {
	var paths []string
	seen := make(map[string]struct{})

	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		if _, err := os.Stat(p); err != nil {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		add(filepath.Join(xdg, "shiet", "config.yaml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".config", "shiet", "config.yaml"))
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		add(filepath.Join(cfgDir, "shiet", "config.yaml"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "shiet.yaml"))
	}

	return paths
}

func defaultDBPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(cfg, "shiet", "shiet.db"), nil
}

func applyEnv(k *koanf.Koanf) {
	for envKey, cfgKey := range envKeyMap {
		if v := os.Getenv(envKey); v != "" {
			_ = k.Set(cfgKey, v)
		}
	}
}

func expandHome(path string) (string, error) {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand home in path %q: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
