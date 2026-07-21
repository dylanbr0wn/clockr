package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	applog "github.com/dylanbr0wn/shiet/internal/log"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestLogErr_logsAndReturnsSameError(t *testing.T) {
	var buf bytes.Buffer
	app := &App{log: applog.New(&buf)}
	// Body/token-shaped text must never appear in the log line (DYL-180).
	want := errors.New(`connect failed: invalid_grant body={"token":"ghp_abcdefghijklmnopqrstuvwxyz"}`)

	got := app.logErr("google.connect", want)
	if !errors.Is(got, want) {
		t.Fatalf("returned error: got %v want %v", got, want)
	}

	out := buf.String()
	if !strings.Contains(out, `"level":"error"`) {
		t.Fatalf("expected error level, got %q", out)
	}
	if !strings.Contains(out, `"op":"google.connect"`) {
		t.Fatalf("expected op field, got %q", out)
	}
	if !strings.Contains(out, `"reason":"unauthorized"`) {
		t.Fatalf("expected reason code, got %q", out)
	}
	if !strings.Contains(out, "operation failed") {
		t.Fatalf("expected operation failed msg, got %q", out)
	}
	for _, leak := range []string{"invalid_grant", "ghp_", "connect failed", `"error":`} {
		if strings.Contains(out, leak) {
			t.Fatalf("raw error material leaked (%q): %q", leak, out)
		}
	}
}

func TestLogErr_nilIsNoop(t *testing.T) {
	var buf bytes.Buffer
	app := &App{log: applog.New(&buf)}
	if err := app.logErr("google.connect", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log for nil error, got %q", buf.String())
	}
}

func TestStartupShutdown_logLifecycle(t *testing.T) {
	var buf bytes.Buffer
	app := &App{log: applog.New(&buf)}

	app.startup(context.Background())
	app.shutdown(context.Background())

	out := buf.String()
	if !strings.Contains(out, `"op":"app.startup"`) || !strings.Contains(out, "startup success") {
		t.Fatalf("expected startup log, got %q", out)
	}
	if !strings.Contains(out, `"op":"app.shutdown"`) || !strings.Contains(out, "shutdown") {
		t.Fatalf("expected shutdown log, got %q", out)
	}
}

func TestWrapSyncPeriod_logsStartAndFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := applog.New(&buf)
	want := errors.New("calendar account needs re-authentication")

	sync := wrapSyncPeriod(logger, func(context.Context, int64) (service.SyncResult, error) {
		return service.SyncResult{}, want
	})

	_, err := sync(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("returned error: got %v want %v", err, want)
	}

	out := buf.String()
	if !strings.Contains(out, `"op":"calendar.sync_period"`) {
		t.Fatalf("expected sync op, got %q", out)
	}
	if !strings.Contains(out, "sync started") {
		t.Fatalf("expected sync started, got %q", out)
	}
	if !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, "operation failed") {
		t.Fatalf("expected sync failure log, got %q", out)
	}
	if !strings.Contains(out, `"reason":"unauthorized"`) {
		t.Fatalf("expected reason code, got %q", out)
	}
	if strings.Contains(out, "re-authentication") || strings.Contains(out, `"error":`) {
		t.Fatalf("raw error material leaked: %q", out)
	}
	if !strings.Contains(out, `"period_id":42`) {
		t.Fatalf("expected period_id, got %q", out)
	}
}

func TestWrapSyncPeriod_successLogsStartOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := applog.New(&buf)

	sync := wrapSyncPeriod(logger, func(context.Context, int64) (service.SyncResult, error) {
		return service.SyncResult{Added: 1}, nil
	})

	res, err := sync(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("added: got %d", res.Added)
	}

	out := buf.String()
	if !strings.Contains(out, "sync started") {
		t.Fatalf("expected sync started, got %q", out)
	}
	if strings.Contains(out, `"level":"error"`) {
		t.Fatalf("unexpected error log on success: %q", out)
	}
}
