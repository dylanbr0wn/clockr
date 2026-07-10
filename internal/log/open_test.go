package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen_writesInfoLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shiet.log")

	logger, closer, err := Open(path, "info", false)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info().Msg("app starting")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"level":"info"`) {
		t.Fatalf("expected info level in log, got %q", data)
	}
	if !strings.Contains(string(data), "app starting") {
		t.Fatalf("expected startup message in log, got %q", data)
	}
}

func TestOpen_respectsLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shiet.log")

	logger, closer, err := Open(path, "error", false)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info().Msg("should be filtered")
	logger.Error().Msg("should appear")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, "should be filtered") {
		t.Fatalf("info line should be filtered at error level, got %q", body)
	}
	if !strings.Contains(body, "should appear") {
		t.Fatalf("expected error line, got %q", body)
	}
}

func TestParseLevel(t *testing.T) {
	lvl, err := parseLevel("")
	if err != nil || lvl.String() != "info" {
		t.Fatalf("empty: got %v %v", lvl, err)
	}
	_, err = parseLevel("nope")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}
