// Package config loads shiet's app/runtime configuration from layered sources:
// baked-in defaults, an optional YAML file, and SHIET_* environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Google Calendar auth modes for the desktop app.
const (
	AuthModeBroker = "broker"
	AuthModeLocal  = "local"

	defaultBrokerBaseURL = "https://auth.shiet.app"
)

// Sentinel errors so callers can distinguish local credential gaps from broker
// configuration problems without parsing message text.
var (
	ErrLocalCredentials       = errors.New("local Google OAuth credentials are not configured")
	ErrBrokerConfig           = errors.New("Google OAuth broker is not configured")
	ErrGitHubBrokerConfig     = errors.New("GitHub OAuth broker is not configured")
	ErrGitHubLocalCredentials = errors.New("local GitHub OAuth credentials are not configured")
	ErrSlackBrokerConfig      = errors.New("Slack OAuth broker is not configured")
	ErrSlackLocalCredentials  = errors.New("local Slack OAuth credentials are not configured")
)

// Config holds typed app/runtime settings. User preferences (window start, AI
// toggles, etc.) live in SQLite — not here.
type Config struct {
	DB struct {
		Path string `koanf:"path"`
	} `koanf:"db"`
	Google struct {
		AuthMode      string `koanf:"auth_mode"`
		BrokerBaseURL string `koanf:"broker_base_url"`
		ClientID      string `koanf:"client_id"`
		ClientSecret  string `koanf:"client_secret"`
	} `koanf:"google"`
	GitHub struct {
		AuthMode      string `koanf:"auth_mode"`
		BrokerBaseURL string `koanf:"broker_base_url"`
		ClientID      string `koanf:"client_id"`
		ClientSecret  string `koanf:"client_secret"`
	} `koanf:"github"`
	Slack struct {
		AuthMode      string `koanf:"auth_mode"`
		BrokerBaseURL string `koanf:"broker_base_url"`
		ClientID      string `koanf:"client_id"`
		ClientSecret  string `koanf:"client_secret"`
	} `koanf:"slack"`
}

// envKeyMap maps legacy SHIET_* env vars to koanf dotted keys.
var envKeyMap = map[string]string{
	"SHIET_DB":                     "db.path",
	"SHIET_GOOGLE_AUTH_MODE":       "google.auth_mode",
	"SHIET_GOOGLE_BROKER_BASE_URL": "google.broker_base_url",
	"SHIET_GOOGLE_CLIENT_ID":       "google.client_id",
	"SHIET_GOOGLE_CLIENT_SECRET":   "google.client_secret",
	"SHIET_GITHUB_AUTH_MODE":       "github.auth_mode",
	"SHIET_GITHUB_BROKER_BASE_URL": "github.broker_base_url",
	"SHIET_GITHUB_CLIENT_ID":       "github.client_id",
	"SHIET_GITHUB_CLIENT_SECRET":   "github.client_secret",
	"SHIET_SLACK_AUTH_MODE":        "slack.auth_mode",
	"SHIET_SLACK_BROKER_BASE_URL":  "slack.broker_base_url",
	"SHIET_SLACK_CLIENT_ID":        "slack.client_id",
	"SHIET_SLACK_CLIENT_SECRET":    "slack.client_secret",
}

// Load reads configuration using the standard discovery order:
//
//  1. Defaults (OS user config dir for db.path; broker base URL for Google)
//  2. Config files, when present (first match wins per path; later paths override):
//     - $XDG_CONFIG_HOME/shiet/config.yaml, or ~/.config/shiet/config.yaml
//     - <UserConfigDir>/shiet/config.yaml (e.g. ~/Library/Application Support/shiet on macOS)
//     - ./shiet.yaml in the process working directory
//  3. Environment variables (highest precedence)
	//  4. Auth mode resolution: explicit mode wins; otherwise local when a
	//     client_id is present, else broker (public-build default). Broker mode
	//     clears any desktop client_id/client_secret from the loaded config.
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
		"google": map[string]any{
			"broker_base_url": defaultBrokerBaseURL,
		},
		"github": map[string]any{
			"broker_base_url": defaultBrokerBaseURL,
		},
		"slack": map[string]any{
			"broker_base_url": defaultBrokerBaseURL,
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

	cfg.Google.AuthMode = strings.ToLower(strings.TrimSpace(cfg.Google.AuthMode))
	cfg.Google.BrokerBaseURL = strings.TrimSpace(cfg.Google.BrokerBaseURL)
	cfg.Google.ClientID = strings.TrimSpace(cfg.Google.ClientID)
	cfg.Google.ClientSecret = strings.TrimSpace(cfg.Google.ClientSecret)
	cfg.GitHub.AuthMode = strings.ToLower(strings.TrimSpace(cfg.GitHub.AuthMode))
	cfg.GitHub.BrokerBaseURL = strings.TrimSpace(cfg.GitHub.BrokerBaseURL)
	cfg.GitHub.ClientID = strings.TrimSpace(cfg.GitHub.ClientID)
	cfg.GitHub.ClientSecret = strings.TrimSpace(cfg.GitHub.ClientSecret)
	cfg.Slack.AuthMode = strings.ToLower(strings.TrimSpace(cfg.Slack.AuthMode))
	cfg.Slack.BrokerBaseURL = strings.TrimSpace(cfg.Slack.BrokerBaseURL)
	cfg.Slack.ClientID = strings.TrimSpace(cfg.Slack.ClientID)
	cfg.Slack.ClientSecret = strings.TrimSpace(cfg.Slack.ClientSecret)

	cfg.resolveGoogleAuthMode()
	cfg.resolveGitHubAuthMode()
	cfg.resolveSlackAuthMode()

	// Broker mode must not carry desktop OAuth credentials into runtime.
	if cfg.UsesBrokerAuth() {
		cfg.Google.ClientID = ""
		cfg.Google.ClientSecret = ""
	}
	if cfg.UsesGitHubBrokerAuth() {
		cfg.GitHub.ClientID = ""
		cfg.GitHub.ClientSecret = ""
	}
	if cfg.UsesSlackBrokerAuth() {
		cfg.Slack.ClientID = ""
		cfg.Slack.ClientSecret = ""
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// resolveAuthMode applies the public-build default: broker when unset, unless
// local/BYO credentials (client_id) are already present.
func resolveAuthMode(explicitMode, clientID string) string {
	if strings.TrimSpace(explicitMode) != "" {
		return strings.ToLower(strings.TrimSpace(explicitMode))
	}
	if strings.TrimSpace(clientID) != "" {
		return AuthModeLocal
	}
	return AuthModeBroker
}

// resolveGoogleAuthMode applies public-build defaults: broker when unset, unless
// local/BYO credentials are already present (dev/advanced-user escape hatch).
func (c *Config) resolveGoogleAuthMode() {
	c.Google.AuthMode = resolveAuthMode(c.Google.AuthMode, c.Google.ClientID)
}

// resolveGitHubAuthMode applies the public-build default. Explicit local mode
// keeps PAT and BYO credentials available for development and advanced users.
func (c *Config) resolveGitHubAuthMode() {
	c.GitHub.AuthMode = resolveAuthMode(c.GitHub.AuthMode, c.GitHub.ClientID)
}

func (c *Config) resolveSlackAuthMode() {
	c.Slack.AuthMode = resolveAuthMode(c.Slack.AuthMode, c.Slack.ClientID)
}

// Validate checks Google auth mode settings. Broker mode requires an HTTPS
// broker base URL and must not depend on a desktop Google client_secret.
// Local/BYO mode requires a desktop client_id and preserves existing OAuth
// credential fields for development and advanced users.
func (c Config) Validate() error {
	mode := strings.ToLower(strings.TrimSpace(c.Google.AuthMode))
	var err error
	switch mode {
	case AuthModeBroker:
		err = c.validateBrokerMode()
	case AuthModeLocal:
		err = c.validateLocalMode()
	case "":
		return fmt.Errorf("google.auth_mode is required (use %q or %q)", AuthModeBroker, AuthModeLocal)
	default:
		return fmt.Errorf("google.auth_mode %q is invalid (use %q or %q)", c.Google.AuthMode, AuthModeBroker, AuthModeLocal)
	}
	if err != nil {
		return err
	}
	if err := c.validateGitHubAuth(); err != nil {
		return err
	}
	return c.validateSlackAuth()
}

func (c Config) validateSlackAuth() error {
	switch strings.ToLower(strings.TrimSpace(c.Slack.AuthMode)) {
	case AuthModeBroker:
		if err := validateBrokerURL(c.Slack.BrokerBaseURL, "slack.broker_base_url", "SHIET_SLACK_BROKER_BASE_URL"); err != nil {
			return fmt.Errorf("%w: %v", ErrSlackBrokerConfig, err)
		}
		return nil
	case AuthModeLocal:
		clientID := strings.TrimSpace(c.Slack.ClientID)
		clientSecret := strings.TrimSpace(c.Slack.ClientSecret)
		if clientID == "" {
			return fmt.Errorf("%w: set slack.client_id or SHIET_SLACK_CLIENT_ID", ErrSlackLocalCredentials)
		}
		if clientSecret == "" {
			return fmt.Errorf("%w: set slack.client_secret or SHIET_SLACK_CLIENT_SECRET for local/BYO Slack OAuth; desktop apps cannot keep it confidential, so public builds must use broker mode", ErrSlackLocalCredentials)
		}
		return nil
	case "":
		return nil
	default:
		return fmt.Errorf("slack.auth_mode %q is invalid (use %q or %q)", c.Slack.AuthMode, AuthModeBroker, AuthModeLocal)
	}
}

// UsesBrokerAuth reports whether Google Calendar auth should go through the
// secret-only OAuth broker (ADR-0001).
func (c Config) UsesBrokerAuth() bool {
	return strings.EqualFold(strings.TrimSpace(c.Google.AuthMode), AuthModeBroker)
}

// UsesGitHubBrokerAuth reports whether GitHub connect should use the hosted
// secret-only OAuth broker. Local mode retains PAT/BYO access.
func (c Config) UsesGitHubBrokerAuth() bool {
	return strings.EqualFold(strings.TrimSpace(c.GitHub.AuthMode), AuthModeBroker)
}

// UsesSlackBrokerAuth reports whether Slack connect should use the hosted
// secret-only OAuth broker.
func (c Config) UsesSlackBrokerAuth() bool {
	return strings.EqualFold(strings.TrimSpace(c.Slack.AuthMode), AuthModeBroker)
}

func (c Config) validateGitHubAuth() error {
	switch strings.ToLower(strings.TrimSpace(c.GitHub.AuthMode)) {
	case AuthModeBroker:
		if err := validateBrokerURL(c.GitHub.BrokerBaseURL, "github.broker_base_url", "SHIET_GITHUB_BROKER_BASE_URL"); err != nil {
			return fmt.Errorf("%w: %v", ErrGitHubBrokerConfig, err)
		}
		return nil
	case AuthModeLocal:
		clientID := strings.TrimSpace(c.GitHub.ClientID)
		clientSecret := strings.TrimSpace(c.GitHub.ClientSecret)
		if clientID == "" && clientSecret == "" {
			return nil // PAT-only local mode.
		}
		if clientID == "" {
			return fmt.Errorf("%w: set github.client_id or SHIET_GITHUB_CLIENT_ID with the local client_secret", ErrGitHubLocalCredentials)
		}
		if clientSecret == "" {
			return fmt.Errorf("%w: set github.client_secret or SHIET_GITHUB_CLIENT_SECRET for local/BYO GitHub OAuth; desktop apps cannot keep it confidential, so public builds must use broker mode", ErrGitHubLocalCredentials)
		}
		return nil
	case "":
		// Config values constructed directly by tests and embedders predate the
		// GitHub section. Load always resolves this to broker or local.
		return nil
	default:
		return fmt.Errorf("github.auth_mode %q is invalid (use %q or %q)", c.GitHub.AuthMode, AuthModeBroker, AuthModeLocal)
	}
}

func (c Config) validateBrokerMode() error {
	if err := validateBrokerURL(c.Google.BrokerBaseURL, "google.broker_base_url", "SHIET_GOOGLE_BROKER_BASE_URL"); err != nil {
		return fmt.Errorf("%w: %v", ErrBrokerConfig, err)
	}
	return nil
}

func validateBrokerURL(raw, key, envKey string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("set %s or %s", key, envKey)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is invalid: %v", key, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s must use https", key)
	}
	if u.Host == "" {
		return fmt.Errorf("%s must include a host", key)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%s must not include query or fragment", key)
	}
	return nil
}

func (c Config) validateLocalMode() error {
	if strings.TrimSpace(c.Google.ClientID) == "" {
		return fmt.Errorf("%w: set google.client_id or SHIET_GOOGLE_CLIENT_ID for local/BYO Google OAuth", ErrLocalCredentials)
	}
	return nil
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
