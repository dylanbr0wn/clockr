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
	defaultListenAddr        = ":8080"
	defaultStateTTL          = 5 * time.Minute
	defaultHandoffTTL        = 2 * time.Minute
	defaultDesktopHandoffURL = "shiet://oauth/google/handoff"
	defaultGoogleScope       = "https://www.googleapis.com/auth/calendar.readonly"
)

// Config holds the broker's environment-driven runtime configuration.
type Config struct {
	ListenAddr         string
	PublicOrigin       string
	GoogleClientID     string
	GoogleClientSecret string
	DesktopHandoffURL  string
	DatastoreDSN       string
	StateTTL           time.Duration
	HandoffTTL         time.Duration
	GoogleScopes       []string
}

// LoadFromEnv reads SHIET_BROKER_* environment variables and validates the
// result. The desktop app's local SHIET_* config is intentionally separate.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:         listenAddrFromEnv(),
		PublicOrigin:       os.Getenv("SHIET_BROKER_PUBLIC_ORIGIN"),
		GoogleClientID:     os.Getenv("SHIET_BROKER_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET"),
		DesktopHandoffURL:  getenv("SHIET_BROKER_DESKTOP_HANDOFF_URL", defaultDesktopHandoffURL),
		DatastoreDSN:       os.Getenv("SHIET_BROKER_DATASTORE_DSN"),
		StateTTL:           defaultStateTTL,
		HandoffTTL:         defaultHandoffTTL,
		GoogleScopes:       splitScopes(getenv("SHIET_BROKER_GOOGLE_SCOPES", defaultGoogleScope)),
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
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// RedirectURI returns the Google Web OAuth redirect URI configured for this
// broker deployment.
func (c Config) RedirectURI() string {
	u, err := c.publicOriginURL()
	if err != nil {
		return ""
	}
	u.Path = "/v1/google/oauth/callback"
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
	raw := strings.TrimSpace(c.DesktopHandoffURL)
	if raw == "" {
		return nil, errors.New("SHIET_BROKER_DESKTOP_HANDOFF_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("SHIET_BROKER_DESKTOP_HANDOFF_URL is invalid: %w", err)
	}
	if u.Scheme == "" {
		return nil, errors.New("SHIET_BROKER_DESKTOP_HANDOFF_URL must include a scheme")
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return nil, errors.New("SHIET_BROKER_DESKTOP_HANDOFF_URL must use the desktop handoff scheme, not http")
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
