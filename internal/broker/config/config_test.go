package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRequiresPublicHTTPSOriginAndSecrets(t *testing.T) {
	cfg := Config{
		ListenAddr:         ":8080",
		PublicOrigin:       "http://auth.shiet.app",
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		DesktopHandoffURL:  "shiet://oauth/google/handoff",
		DatastoreDSN:       "file:broker.db",
		StateTTL:           5 * time.Minute,
		HandoffTTL:         2 * time.Minute,
		GoogleScopes:       []string{defaultGoogleScope},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("expected https validation error, got %q", err)
	}

	cfg.PublicOrigin = "https://auth.shiet.app"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
	if got, want := cfg.RedirectURI(), "https://auth.shiet.app/v1/google/oauth/callback"; got != want {
		t.Fatalf("redirect uri: got %q want %q", got, want)
	}
}

func TestValidateRejectsOverlongTTLs(t *testing.T) {
	cfg := Config{
		ListenAddr:         ":8080",
		PublicOrigin:       "https://auth.shiet.app",
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		DesktopHandoffURL:  "shiet://oauth/google/handoff",
		DatastoreDSN:       "file:broker.db",
		StateTTL:           11 * time.Minute,
		HandoffTTL:         6 * time.Minute,
		GoogleScopes:       []string{defaultGoogleScope},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "STATE_TTL") || !strings.Contains(err.Error(), "HANDOFF_TTL") {
		t.Fatalf("expected ttl validation errors, got %q", err)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SHIET_BROKER_PUBLIC_ORIGIN", "https://auth.shiet.app")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("SHIET_BROKER_DATASTORE_DSN", "file:broker.db")
	t.Setenv("SHIET_BROKER_STATE_TTL", "3m")
	t.Setenv("SHIET_BROKER_GOOGLE_SCOPES", "scope-a,scope-b")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StateTTL != 3*time.Minute {
		t.Fatalf("state ttl: got %s", cfg.StateTTL)
	}
	if got := strings.Join(cfg.GoogleScopes, ","); got != "scope-a,scope-b" {
		t.Fatalf("scopes: got %q", got)
	}
}

func TestLoadFromEnvGitHubProviderConfig(t *testing.T) {
	t.Setenv("SHIET_BROKER_PUBLIC_ORIGIN", "https://auth.shiet.app")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET", "google-client-secret")
	t.Setenv("SHIET_BROKER_GITHUB_CLIENT_ID", "github-client-id")
	t.Setenv("SHIET_BROKER_GITHUB_CLIENT_SECRET", "github-client-secret")
	t.Setenv("SHIET_BROKER_GITHUB_SCOPES", "repo,read:user")
	t.Setenv("SHIET_BROKER_DATASTORE_DSN", "file:broker.db")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHubClientID != "github-client-id" || cfg.GitHubClientSecret != "github-client-secret" {
		t.Fatalf("github credentials not loaded: %+v", cfg)
	}
	if got := strings.Join(cfg.GitHubScopes, ","); got != "repo,read:user" {
		t.Fatalf("github scopes: got %q", got)
	}
	if got, want := cfg.GitHubRedirectURI(), "https://auth.shiet.app/v1/github/oauth/callback"; got != want {
		t.Fatalf("github redirect uri: got %q want %q", got, want)
	}
	if got, want := cfg.GitHubDesktopHandoffURL, "shiet://oauth/github/handoff"; got != want {
		t.Fatalf("github handoff url: got %q want %q", got, want)
	}
}

func TestValidateRejectsPartialGitHubCredentials(t *testing.T) {
	cfg := Config{
		ListenAddr:              ":8080",
		PublicOrigin:            "https://auth.shiet.app",
		GoogleClientID:          "google-client-id",
		GoogleClientSecret:      "google-client-secret",
		DesktopHandoffURL:       "shiet://oauth/google/handoff",
		GitHubClientID:          "github-client-id",
		GitHubDesktopHandoffURL: "shiet://oauth/github/handoff",
		DatastoreDSN:            "file:broker.db",
		StateTTL:                5 * time.Minute,
		HandoffTTL:              2 * time.Minute,
		GoogleScopes:            []string{defaultGoogleScope},
		GitHubScopes:            []string{defaultGitHubScope},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "SHIET_BROKER_GITHUB_CLIENT_SECRET") {
		t.Fatalf("expected partial GitHub credential error, got %v", err)
	}
}

func TestLoadFromEnvUsesRailwayPortWhenListenAddrUnset(t *testing.T) {
	t.Setenv("SHIET_BROKER_PUBLIC_ORIGIN", "https://auth.shiet.app")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("SHIET_BROKER_DATASTORE_DSN", "file:broker.db")
	t.Setenv("SHIET_BROKER_LISTEN_ADDR", "")
	t.Setenv("PORT", "7654")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":7654" {
		t.Fatalf("listen addr: got %q want %q", cfg.ListenAddr, ":7654")
	}
}

func TestLoadFromEnvKillSwitches(t *testing.T) {
	t.Setenv("SHIET_BROKER_PUBLIC_ORIGIN", "https://auth.shiet.app")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("SHIET_BROKER_DATASTORE_DSN", "file:broker.db")
	t.Setenv("SHIET_BROKER_AUTH_DISABLED", "true")
	t.Setenv("SHIET_BROKER_REFRESH_DISABLED", "1")
	t.Setenv("SHIET_BROKER_DISABLED_APP_VERSIONS", "0.1.0, bad-build ")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AuthDisabled {
		t.Fatal("expected AuthDisabled")
	}
	if !cfg.RefreshDisabled {
		t.Fatal("expected RefreshDisabled")
	}
	if got := strings.Join(cfg.DisabledAppVersions, ","); got != "0.1.0,bad-build" {
		t.Fatalf("disabled versions: got %q", got)
	}
	if !cfg.AppVersionDisabled("0.1.0") {
		t.Fatal("expected 0.1.0 disabled")
	}
	if cfg.AppVersionDisabled("0.2.0") {
		t.Fatal("did not expect 0.2.0 disabled")
	}
}

func TestLoadFromEnvMetricsToken(t *testing.T) {
	t.Setenv("SHIET_BROKER_PUBLIC_ORIGIN", "https://auth.shiet.app")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("SHIET_BROKER_GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("SHIET_BROKER_DATASTORE_DSN", "file:broker.db")
	t.Setenv("SHIET_BROKER_METRICS_TOKEN", "metrics-secret")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsToken != "metrics-secret" {
		t.Fatalf("MetricsToken: got %q want %q", cfg.MetricsToken, "metrics-secret")
	}
}
