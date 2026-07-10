// Package config loads configuration for shiet's deployable OAuth broker.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddr              = ":8080"
	defaultStateTTL                = 5 * time.Minute
	defaultHandoffTTL              = 2 * time.Minute
	defaultDesktopHandoffURL       = "shiet://oauth/google/handoff"
	defaultGitHubDesktopHandoffURL = "shiet://oauth/github/handoff"
	defaultGoogleScope             = "https://www.googleapis.com/auth/calendar.readonly"
	defaultGitHubScope             = "repo"
)

// Config holds the broker's environment-driven runtime configuration.
type Config struct {
	ListenAddr              string
	PublicOrigin            string
	GoogleClientID          string
	GoogleClientSecret      string
	DesktopHandoffURL       string
	GitHubClientID          string
	GitHubClientSecret      string
	GitHubDesktopHandoffURL string
	DatastoreDSN            string
	StateTTL                time.Duration
	HandoffTTL              time.Duration
	GoogleScopes            []string
	GitHubScopes            []string
	AuthDisabled            bool
	RefreshDisabled         bool
	DisabledAppVersions     []string
}

// LoadFromEnv reads SHIET_BROKER_* environment variables and validates the
// result. The desktop app's local SHIET_* config is intentionally separate.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:              listenAddrFromEnv(),
		PublicOrigin:            os.Getenv("SHIET_BROKER_PUBLIC_ORIGIN"),
		GoogleClientID:          os.Getenv("SHIET_BROKER_GOOGLE_CLIENT_ID"),
		GoogleClientSecret:      os.Getenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET"),
		DesktopHandoffURL:       getenv("SHIET_BROKER_DESKTOP_HANDOFF_URL", defaultDesktopHandoffURL),
		GitHubClientID:          os.Getenv("SHIET_BROKER_GITHUB_CLIENT_ID"),
		GitHubClientSecret:      os.Getenv("SHIET_BROKER_GITHUB_CLIENT_SECRET"),
		GitHubDesktopHandoffURL: getenv("SHIET_BROKER_GITHUB_DESKTOP_HANDOFF_URL", defaultGitHubDesktopHandoffURL),
		DatastoreDSN:            os.Getenv("SHIET_BROKER_DATASTORE_DSN"),
		StateTTL:                defaultStateTTL,
		HandoffTTL:              defaultHandoffTTL,
		GoogleScopes:            splitScopes(getenv("SHIET_BROKER_GOOGLE_SCOPES", defaultGoogleScope)),
		GitHubScopes:            splitScopes(getenv("SHIET_BROKER_GITHUB_SCOPES", defaultGitHubScope)),
		AuthDisabled:            envTruthy("SHIET_BROKER_AUTH_DISABLED"),
		RefreshDisabled:         envTruthy("SHIET_BROKER_REFRESH_DISABLED"),
		DisabledAppVersions:     splitCSV(os.Getenv("SHIET_BROKER_DISABLED_APP_VERSIONS")),
	}

	var err error
	if v := os.Getenv("SHIET_BROKER_STATE_TTL"); v != "" {
		cfg.StateTTL, err = time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIET_BROKER_STATE_TTL: %w", err)
		}
	}
	if v := os.Getenv("SHIET_BROKER_HANDOFF_TTL"); v != "" {
		cfg.HandoffTTL, err = time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIET_BROKER_HANDOFF_TTL: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that the broker can safely construct public OAuth URLs and
// short-lived coordination records.
func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.ListenAddr) == "" {
		problems = append(problems, "SHIET_BROKER_LISTEN_ADDR is required")
	}
	if strings.TrimSpace(c.GoogleClientID) == "" {
		problems = append(problems, "SHIET_BROKER_GOOGLE_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.GoogleClientSecret) == "" {
		problems = append(problems, "SHIET_BROKER_GOOGLE_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.DatastoreDSN) == "" {
		problems = append(problems, "SHIET_BROKER_DATASTORE_DSN is required")
	}
	if len(c.GoogleScopes) == 0 {
		problems = append(problems, "SHIET_BROKER_GOOGLE_SCOPES must include at least one scope")
	}
	githubID := strings.TrimSpace(c.GitHubClientID)
	githubSecret := strings.TrimSpace(c.GitHubClientSecret)
	if (githubID == "") != (githubSecret == "") {
		if githubID == "" {
			problems = append(problems, "SHIET_BROKER_GITHUB_CLIENT_ID is required when GitHub OAuth is configured")
		} else {
			problems = append(problems, "SHIET_BROKER_GITHUB_CLIENT_SECRET is required when GitHub OAuth is configured")
		}
	}
	if githubID != "" && len(c.GitHubScopes) == 0 {
		problems = append(problems, "SHIET_BROKER_GITHUB_SCOPES must include at least one scope")
	}
	if c.StateTTL <= 0 || c.StateTTL > 10*time.Minute {
		problems = append(problems, "SHIET_BROKER_STATE_TTL must be greater than 0 and at most 10m")
	}
	if c.HandoffTTL <= 0 || c.HandoffTTL > 5*time.Minute {
		problems = append(problems, "SHIET_BROKER_HANDOFF_TTL must be greater than 0 and at most 5m")
	}
	if _, err := c.publicOriginURL(); err != nil {
		problems = append(problems, err.Error())
	}
	if _, err := c.desktopHandoffURL(); err != nil {
		problems = append(problems, err.Error())
	}
	if githubID != "" {
		if _, err := parseDesktopHandoffURL(c.GitHubDesktopHandoffURL, "SHIET_BROKER_GITHUB_DESKTOP_HANDOFF_URL"); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// AppVersionDisabled reports whether the given desktop app version is blocked
// by the operator kill list.
func (c Config) AppVersionDisabled(appVersion string) bool {
	appVersion = strings.TrimSpace(appVersion)
	if appVersion == "" || len(c.DisabledAppVersions) == 0 {
		return false
	}
	for _, blocked := range c.DisabledAppVersions {
		if appVersion == blocked {
			return true
		}
	}
	return false
}

// RedirectURI returns the Google Web OAuth redirect URI configured for this
// broker deployment.
func (c Config) RedirectURI() string {
	return c.ProviderRedirectURI("google")
}

// GitHubRedirectURI returns the GitHub OAuth App callback URI.
func (c Config) GitHubRedirectURI() string {
	return c.ProviderRedirectURI("github")
}

// ProviderRedirectURI returns the broker callback URI for a registered provider.
func (c Config) ProviderRedirectURI(provider string) string {
	u, err := c.publicOriginURL()
	if err != nil {
		return ""
	}
	u.Path = "/v1/" + strings.TrimSpace(provider) + "/oauth/callback"
	return u.String()
}

func (c Config) publicOriginURL() (*url.URL, error) {
	raw := strings.TrimSpace(c.PublicOrigin)
	if raw == "" {
		return nil, errors.New("SHIET_BROKER_PUBLIC_ORIGIN is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("SHIET_BROKER_PUBLIC_ORIGIN is invalid: %w", err)
	}
	if u.Scheme != "https" {
		return nil, errors.New("SHIET_BROKER_PUBLIC_ORIGIN must use https")
	}
	if u.Host == "" {
		return nil, errors.New("SHIET_BROKER_PUBLIC_ORIGIN must include a host")
	}
	if u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("SHIET_BROKER_PUBLIC_ORIGIN must be an origin without path, query, or fragment")
	}
	return u, nil
}

func (c Config) desktopHandoffURL() (*url.URL, error) {
	return parseDesktopHandoffURL(c.DesktopHandoffURL, "SHIET_BROKER_DESKTOP_HANDOFF_URL")
}

func parseDesktopHandoffURL(value, envKey string) (*url.URL, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", envKey)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid: %w", envKey, err)
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("%s must include a scheme", envKey)
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return nil, fmt.Errorf("%s must use the desktop handoff scheme, not http", envKey)
	}
	return u, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func listenAddrFromEnv() string {
	if v := os.Getenv("SHIET_BROKER_LISTEN_ADDR"); v != "" {
		return v
	}
	if v := os.Getenv("PORT"); v != "" {
		return ":" + v
	}
	return defaultListenAddr
}

func splitScopes(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	scopes := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			scopes = append(scopes, field)
		}
	}
	return scopes
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envTruthy(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
