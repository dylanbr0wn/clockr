package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/clockr/internal/db"
)

func TestMigrateCategoryColor_IdempotentWhenColumnExists(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "category-color.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.MigrateTo(conn, 5); err != nil {
		t.Fatalf("migrate to v5: %v", err)
	}

	// Dev DBs that ran the pre-rebase color migration already have this column at v5.
	if _, err := conn.ExecContext(ctx, `
		ALTER TABLE category
		ADD COLUMN color TEXT NOT NULL DEFAULT '#64748B';
	`); err != nil {
		t.Fatalf("pre-add color column: %v", err)
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO category (name, is_default_gap, description, key, color)
		VALUES ('Meetings', 0, '', 'meetings', '#64748B');
	`); err != nil {
		t.Fatalf("insert category: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate with existing color column: %v", err)
	}

	var color string
	if err := conn.QueryRowContext(ctx, `
		SELECT color FROM category WHERE name = 'Meetings' LIMIT 1
	`).Scan(&color); err != nil {
		t.Fatalf("read seeded category color: %v", err)
	}
	if color != "#0EA5E9" {
		t.Fatalf("color = %q, want #0EA5E9", color)
	}
}
