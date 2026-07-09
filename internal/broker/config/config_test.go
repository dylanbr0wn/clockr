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
