package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
)

func TestMigrateTimeEntry_ReplacesGapFill(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "time-entry.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.MigrateTo(conn, 17); err != nil {
		t.Fatalf("migrate to v17: %v", err)
	}

	var gapFillExists int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'gap_fill'
	`).Scan(&gapFillExists); err != nil {
		t.Fatalf("check gap_fill before cutover: %v", err)
	}
	if gapFillExists != 1 {
		t.Fatalf("gap_fill exists before cutover = %d, want 1", gapFillExists)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var timeEntryExists, gapFillAfter int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'time_entry'
	`).Scan(&timeEntryExists); err != nil {
		t.Fatalf("check time_entry: %v", err)
	}
	if timeEntryExists != 1 {
		t.Fatalf("time_entry exists = %d, want 1", timeEntryExists)
	}

	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'gap_fill'
	`).Scan(&gapFillAfter); err != nil {
		t.Fatalf("check gap_fill after cutover: %v", err)
	}
	if gapFillAfter != 0 {
		t.Fatalf("gap_fill exists after cutover = %d, want 0", gapFillAfter)
	}

	// Required columns + optional provenance columns exist.
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(time_entry)`)
	if err != nil {
		t.Fatalf("pragma time_entry: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("pragma rows: %v", err)
	}

	for _, want := range []string{
		"id", "period_id", "start_instant", "end_instant",
		"duration_minutes", "local_work_date", "category_id", "description",
		"attestation", "source_kind", "source_id", "source_revision", "method",
		"created_at", "updated_at",
	} {
		if !cols[want] {
			t.Fatalf("time_entry missing column %q", want)
		}
	}
}
