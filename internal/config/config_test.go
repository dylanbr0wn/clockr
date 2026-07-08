package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_precedenceDefaultFileEnv(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "clockr.yaml")
	if err := os.WriteFile(cfgFile, []byte("db:\n  path: /from/file.db\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLOCKR_DB", "")
	t.Setenv("CLOCKR_GOOGLE_CLIENT_ID", "")
	t.Setenv("CLOCKR_GOOGLE_CLIENT_SECRET", "")

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
	t.Setenv("CLOCKR_DB", "/from/env.db")
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
	cfgFile := filepath.Join(dir, "clockr.yaml")
	content := "google:\n  client_id: file-id\n  client_secret: file-secret\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLOCKR_GOOGLE_CLIENT_ID", "env-id")
	t.Setenv("CLOCKR_GOOGLE_CLIENT_SECRET", "env-secret")

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

	got, err := expandHome("~/data/clockr.db")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "data", "clockr.db")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
