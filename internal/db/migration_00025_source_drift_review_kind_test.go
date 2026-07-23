package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
)

func TestMigrateSourceDriftReviewKind_AcceptsSourceDrift(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "source-drift-kind.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.MigrateTo(conn, 24); err != nil {
		t.Fatalf("migrate to v24: %v", err)
	}

	var periodID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO period (start_date, end_date, cadence, anchor_date)
		VALUES ('2026-06-01', '2026-06-14', 'bi-weekly', '2026-06-01')
		RETURNING id
	`).Scan(&periodID); err != nil {
		t.Fatalf("insert period: %v", err)
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO review_item (period_id, kind, payload, conflict_key)
		VALUES (?, 'source_drift', '{}', 'too-early')
	`, periodID)
	if err == nil {
		t.Fatal("expected source_drift insert to fail before migration")
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var id int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO review_item (period_id, kind, payload, conflict_key)
		VALUES (?, 'source_drift', '{"time_entry_ids":[1]}', 'evt|source_drift')
		RETURNING id
	`, periodID).Scan(&id); err != nil {
		t.Fatalf("insert source_drift: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero review item id")
	}
}
