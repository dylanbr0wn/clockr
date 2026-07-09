package connection_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
)

func newRegistry(t *testing.T) *connection.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return connection.NewRegistry(conn)
}

func TestRegistryUpsertListDisconnect(t *testing.T) {
	reg := newRegistry(t)
	ctx := context.Background()

	got, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:     "google",
		AccountLabel: "Work Google",
		AccountID:    "user@example.com",
		Scopes:       []string{"calendar.readonly"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != connection.StatusConnected {
		t.Fatalf("status: %q", got.Status)
	}

	all, err := reg.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 connection, got %d", len(all))
	}

	got, err = reg.Upsert(ctx, connection.UpsertInput{
		Provider:     "google",
		AccountLabel: "Work Google (updated)",
		AccountID:    "user@example.com",
		Scopes:       []string{"calendar.readonly", "email"},
		Status:       connection.StatusNeedsReauth,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountLabel != "Work Google (updated)" {
		t.Fatalf("label: %q", got.AccountLabel)
	}
	if got.Status != connection.StatusNeedsReauth {
		t.Fatalf("status: %q", got.Status)
	}

	if err := reg.Disconnect(ctx, "google", "user@example.com"); err != nil {
		t.Fatal(err)
	}
	_, err = reg.Get(ctx, "google", "user@example.com")
	if !errors.Is(err, connection.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestRegistrySetStatus(t *testing.T) {
	reg := newRegistry(t)
	ctx := context.Background()

	_, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:     "github",
		AccountLabel: "GitHub",
		AccountID:    "octocat",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reg.SetStatus(ctx, "github", "octocat", connection.StatusNeedsReauth); err != nil {
		t.Fatal(err)
	}

	got, err := reg.Get(ctx, "github", "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != connection.StatusNeedsReauth {
		t.Fatalf("status: %q", got.Status)
	}
}
