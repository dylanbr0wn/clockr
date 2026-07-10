package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_precedenceDefaultFileEnv(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	if err := os.WriteFile(cfgFile, []byte("db:\n  path: /from/file.db\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHIET_DB", "")
	t.Setenv("SHIET_GOOGLE_CLIENT_ID", "")
	t.Setenv("SHIET_GOOGLE_CLIENT_SECRET", "")
	t.Setenv("SHIET_GOOGLE_AUTH_MODE", "")
	t.Setenv("SHIET_GOOGLE_BROKER_BASE_URL", "")

	// Defaults only (no config files passed).
	cfg, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.Path == "" {
		t.Fatal("expected non-empty default db path")
	}
	if cfg.DB.Path == "/from/file.db" {
		t.Fatal("default should not come from file")
	}
	defaultPath := cfg.DB.Path

	// File overrides default.
	cfg, err = load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.Path != "/from/file.db" {
		t.Fatalf("file path: got %q want %q", cfg.DB.Path, "/from/file.db")
	}

	// Env overrides file.
	t.Setenv("SHIET_DB", "/from/env.db")
	cfg, err = load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.Path != "/from/env.db" {
		t.Fatalf("env path: got %q want %q", cfg.DB.Path, "/from/env.db")
	}

	// Sanity: default was stable before overrides.
	if defaultPath == "/from/file.db" || defaultPath == "/from/env.db" {
		t.Fatalf("unexpected default path %q", defaultPath)
	}
}

func TestLoad_googleEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "google:\n  client_id: file-id\n  client_secret: file-secret\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHIET_GOOGLE_CLIENT_ID", "env-id")
	t.Setenv("SHIET_GOOGLE_CLIENT_SECRET", "env-secret")

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.ClientID != "env-id" {
		t.Fatalf("client_id: got %q want %q", cfg.Google.ClientID, "env-id")
	}
	if cfg.Google.ClientSecret != "env-secret" {
		t.Fatalf("client_secret: got %q want %q", cfg.Google.ClientSecret, "env-secret")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := expandHome("~/data/shiet.db")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "data", "shiet.db")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoad_googleAuthModeDefaultsToBroker(t *testing.T) {
	clearGoogleEnv(t)

	cfg, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.AuthMode != AuthModeBroker {
		t.Fatalf("auth_mode: got %q want %q", cfg.Google.AuthMode, AuthModeBroker)
	}
	if cfg.Google.BrokerBaseURL != defaultBrokerBaseURL {
		t.Fatalf("broker_base_url: got %q want %q", cfg.Google.BrokerBaseURL, defaultBrokerBaseURL)
	}
	if cfg.Google.ClientSecret != "" {
		t.Fatalf("public default must not embed client_secret, got %q", cfg.Google.ClientSecret)
	}
}

func TestLoad_githubAuthModeDefaultsToBrokerWithoutDesktopSecret(t *testing.T) {
	clearGoogleEnv(t)
	clearGitHubEnv(t)

	cfg, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHub.AuthMode != AuthModeBroker {
		t.Fatalf("auth_mode: got %q want %q", cfg.GitHub.AuthMode, AuthModeBroker)
	}
	if cfg.GitHub.BrokerBaseURL != defaultBrokerBaseURL {
		t.Fatalf("broker_base_url: got %q want %q", cfg.GitHub.BrokerBaseURL, defaultBrokerBaseURL)
	}
	if cfg.GitHub.ClientSecret != "" {
		t.Fatalf("public default must not embed client_secret, got %q", cfg.GitHub.ClientSecret)
	}
}

func TestLoad_githubLocalModeKeepsBYOCredentials(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "github:\n  auth_mode: local\n  client_id: byo-id\n  client_secret: byo-secret\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	clearGoogleEnv(t)
	clearGitHubEnv(t)

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHub.AuthMode != AuthModeLocal {
		t.Fatalf("auth_mode: got %q want %q", cfg.GitHub.AuthMode, AuthModeLocal)
	}
	if cfg.GitHub.ClientID != "byo-id" || cfg.GitHub.ClientSecret != "byo-secret" {
		t.Fatalf("local credentials: got client_id=%q client_secret=%q", cfg.GitHub.ClientID, cfg.GitHub.ClientSecret)
	}
}

func TestLoad_githubBrokerModeClearsDesktopSecret(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "github:\n  auth_mode: broker\n  broker_base_url: https://auth.shiet.app\n  client_id: should-be-cleared\n  client_secret: should-be-cleared\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	clearGoogleEnv(t)
	clearGitHubEnv(t)

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHub.ClientID != "" || cfg.GitHub.ClientSecret != "" {
		t.Fatalf("broker mode must clear desktop credentials, got client_id=%q client_secret=%q", cfg.GitHub.ClientID, cfg.GitHub.ClientSecret)
	}
}

func TestValidate_githubLocalModeRequiresBYOSecret(t *testing.T) {
	cfg := Config{}
	cfg.Google.AuthMode = AuthModeBroker
	cfg.Google.BrokerBaseURL = "https://auth.shiet.app"
	cfg.GitHub.AuthMode = AuthModeLocal
	cfg.GitHub.ClientID = "client-id"

	err := cfg.Validate()
	if err == nil || !errors.Is(err, ErrGitHubLocalCredentials) || !strings.Contains(err.Error(), "client_secret") {
		t.Fatalf("expected GitHub local client_secret error, got %v", err)
	}
}

func TestValidate_githubLocalModeAllowsPATOnly(t *testing.T) {
	cfg := Config{}
	cfg.Google.AuthMode = AuthModeBroker
	cfg.Google.BrokerBaseURL = "https://auth.shiet.app"
	cfg.GitHub.AuthMode = AuthModeLocal

	if err := cfg.Validate(); err != nil {
		t.Fatalf("PAT-only local mode should pass: %v", err)
	}
}

func TestLoad_implicitLocalWhenClientIDPresent(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "google:\n  client_id: byo-id\n  client_secret: byo-secret\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	clearGoogleEnv(t)

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.AuthMode != AuthModeLocal {
		t.Fatalf("auth_mode: got %q want %q (BYO credentials without explicit mode)", cfg.Google.AuthMode, AuthModeLocal)
	}
	if cfg.Google.ClientSecret != "byo-secret" {
		t.Fatalf("local mode should keep client_secret, got %q", cfg.Google.ClientSecret)
	}
}

func TestLoad_brokerModeClearsClientSecret(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "" +
		"google:\n" +
		"  auth_mode: broker\n" +
		"  broker_base_url: https://auth.shiet.app\n" +
		"  client_id: should-be-cleared\n" +
		"  client_secret: should-be-cleared\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	clearGoogleEnv(t)

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.ClientID != "" || cfg.Google.ClientSecret != "" {
		t.Fatalf("broker mode must clear desktop credentials, got client_id=%q client_secret=%q", cfg.Google.ClientID, cfg.Google.ClientSecret)
	}
}

func TestLoad_googleAuthModeFileAndEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "" +
		"google:\n" +
		"  auth_mode: local\n" +
		"  broker_base_url: https://file.example\n" +
		"  client_id: file-id\n" +
		"  client_secret: file-secret\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	clearGoogleEnv(t)

	cfg, err := load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.AuthMode != AuthModeLocal {
		t.Fatalf("file auth_mode: got %q want %q", cfg.Google.AuthMode, AuthModeLocal)
	}
	if cfg.Google.BrokerBaseURL != "https://file.example" {
		t.Fatalf("file broker_base_url: got %q", cfg.Google.BrokerBaseURL)
	}

	t.Setenv("SHIET_GOOGLE_AUTH_MODE", "broker")
	t.Setenv("SHIET_GOOGLE_BROKER_BASE_URL", "https://env.example")
	cfg, err = load([]string{cfgFile})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.AuthMode != AuthModeBroker {
		t.Fatalf("env auth_mode: got %q want %q", cfg.Google.AuthMode, AuthModeBroker)
	}
	if cfg.Google.BrokerBaseURL != "https://env.example" {
		t.Fatalf("env broker_base_url: got %q", cfg.Google.BrokerBaseURL)
	}
}

func TestValidate_brokerModeRequiresHTTPSBrokerURLNotClientSecret(t *testing.T) {
	cfg := Config{}
	cfg.Google.AuthMode = AuthModeBroker
	cfg.Google.BrokerBaseURL = "https://auth.shiet.app"
	cfg.Google.ClientSecret = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("broker mode with HTTPS URL and empty secret should pass: %v", err)
	}

	cfg.Google.BrokerBaseURL = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing broker_base_url")
	}
	if !errors.Is(err, ErrBrokerConfig) {
		t.Fatalf("want ErrBrokerConfig, got %v", err)
	}
	if !strings.Contains(err.Error(), "broker_base_url") && !strings.Contains(err.Error(), "SHIET_GOOGLE_BROKER_BASE_URL") {
		t.Fatalf("broker config error should mention broker URL: %v", err)
	}

	cfg.Google.BrokerBaseURL = "http://insecure.example"
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for non-HTTPS broker URL")
	}
	if !errors.Is(err, ErrBrokerConfig) {
		t.Fatalf("want ErrBrokerConfig, got %v", err)
	}
}

func TestValidate_localModeRequiresClientIDNotBrokerURL(t *testing.T) {
	cfg := Config{}
	cfg.Google.AuthMode = AuthModeLocal
	cfg.Google.ClientID = "desktop-client-id"
	cfg.Google.BrokerBaseURL = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local mode with client_id should pass: %v", err)
	}

	cfg.Google.ClientID = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing local client_id")
	}
	if !errors.Is(err, ErrLocalCredentials) {
		t.Fatalf("want ErrLocalCredentials, got %v", err)
	}
	if !strings.Contains(err.Error(), "client_id") && !strings.Contains(err.Error(), "SHIET_GOOGLE_CLIENT_ID") {
		t.Fatalf("local credential error should mention client_id: %v", err)
	}
}

func TestValidate_rejectsUnknownAuthMode(t *testing.T) {
	cfg := Config{}
	cfg.Google.AuthMode = "weird"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unknown auth_mode")
	}
	if !strings.Contains(err.Error(), "auth_mode") {
		t.Fatalf("error should mention auth_mode: %v", err)
	}
}

func TestLoad_validatesGoogleAuth(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "shiet.yaml")
	content := "google:\n  auth_mode: local\n  client_id: \"\"\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	clearGoogleEnv(t)

	_, err := load([]string{cfgFile})
	if err == nil {
		t.Fatal("expected load to fail validation for local mode without client_id")
	}
	if !errors.Is(err, ErrLocalCredentials) {
		t.Fatalf("want ErrLocalCredentials, got %v", err)
	}
}

func clearGoogleEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SHIET_DB", "")
	t.Setenv("SHIET_GOOGLE_CLIENT_ID", "")
	t.Setenv("SHIET_GOOGLE_CLIENT_SECRET", "")
	t.Setenv("SHIET_GOOGLE_AUTH_MODE", "")
	t.Setenv("SHIET_GOOGLE_BROKER_BASE_URL", "")
}

func clearGitHubEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SHIET_GITHUB_CLIENT_ID", "")
	t.Setenv("SHIET_GITHUB_CLIENT_SECRET", "")
	t.Setenv("SHIET_GITHUB_AUTH_MODE", "")
	t.Setenv("SHIET_GITHUB_BROKER_BASE_URL", "")
}
