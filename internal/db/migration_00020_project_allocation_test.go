package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
)

func TestMigrateProjectAllocation_AddsProjectAndDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "project-allocation.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.MigrateTo(conn, 19); err != nil {
		t.Fatalf("migrate to v19: %v", err)
	}

	var periodID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO period (start_date, end_date, cadence, anchor_date, target_hours_per_day)
		VALUES ('2026-06-01', '2026-06-14', 'bi-weekly', '2026-06-01', 8)
		RETURNING id
	`).Scan(&periodID); err != nil {
		t.Fatalf("insert period: %v", err)
	}

	var entryID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T14:00:00Z', '2026-06-01T15:00:00Z', 60, '2026-06-01', 'pre-migration', 'draft')
		RETURNING id
	`, periodID).Scan(&entryID); err != nil {
		t.Fatalf("insert time_entry: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var projectExists int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'project'
	`).Scan(&projectExists); err != nil {
		t.Fatalf("check project table: %v", err)
	}
	if projectExists != 1 {
		t.Fatalf("project exists = %d, want 1", projectExists)
	}

	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(project)`)
	if err != nil {
		t.Fatalf("pragma project: %v", err)
	}
	defer rows.Close()

	projectCols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan project pragma: %v", err)
		}
		projectCols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("project pragma rows: %v", err)
	}
	for _, want := range []string{
		"id", "name", "key", "color", "archived_at", "created_at", "updated_at",
	} {
		if !projectCols[want] {
			t.Fatalf("project missing column %q", want)
		}
	}

	var workType, billableStatus string
	var projectID any
	if err := conn.QueryRowContext(ctx, `
		SELECT work_type, project_id, billable_status FROM time_entry WHERE id = ?
	`, entryID).Scan(&workType, &projectID, &billableStatus); err != nil {
		t.Fatalf("read migrated entry: %v", err)
	}
	if workType != "worked" {
		t.Fatalf("work_type = %q, want worked", workType)
	}
	if billableStatus != "unset" {
		t.Fatalf("billable_status = %q, want unset", billableStatus)
	}
	if projectID != nil {
		t.Fatalf("project_id = %#v, want NULL", projectID)
	}
}

func TestMigrateProjectAllocation_RejectsInvalidEnums(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "project-allocation-enums.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var periodID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO period (start_date, end_date, cadence, anchor_date, target_hours_per_day)
		VALUES ('2026-06-01', '2026-06-14', 'bi-weekly', '2026-06-01', 8)
		RETURNING id
	`).Scan(&periodID); err != nil {
		t.Fatalf("insert period: %v", err)
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation, work_type
		) VALUES (?, '2026-06-01T14:00:00Z', '2026-06-01T15:00:00Z', 60, '2026-06-01', 'bad', 'draft', 'overtime')
	`, periodID)
	if err == nil {
		t.Fatal("expected invalid work_type to fail")
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation, billable_status
		) VALUES (?, '2026-06-01T14:00:00Z', '2026-06-01T15:00:00Z', 60, '2026-06-01', 'bad', 'draft', 'maybe')
	`, periodID)
	if err == nil {
		t.Fatal("expected invalid billable_status to fail")
	}
}
