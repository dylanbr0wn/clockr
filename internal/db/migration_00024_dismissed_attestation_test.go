package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
)

func TestMigrateDismissedAttestation_AcceptsDismissedAndKeepsExisting(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dismissed-attestation.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.MigrateTo(conn, 23); err != nil {
		t.Fatalf("migrate to v23: %v", err)
	}

	var periodID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO period (start_date, end_date, cadence, anchor_date)
		VALUES ('2026-06-01', '2026-06-14', 'bi-weekly', '2026-06-01')
		RETURNING id
	`).Scan(&periodID); err != nil {
		t.Fatalf("insert period: %v", err)
	}

	var draftID, confirmedID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T14:00:00Z', '2026-06-01T15:00:00Z', 60, '2026-06-01', 'pre draft', 'draft')
		RETURNING id
	`, periodID).Scan(&draftID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T15:00:00Z', '2026-06-01T16:00:00Z', 60, '2026-06-01', 'pre confirmed', 'confirmed')
		RETURNING id
	`, periodID).Scan(&confirmedID); err != nil {
		t.Fatalf("insert confirmed: %v", err)
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T16:00:00Z', '2026-06-01T17:00:00Z', 60, '2026-06-01', 'too early', 'dismissed')
	`, periodID)
	if err == nil {
		t.Fatal("expected dismissed insert to fail before migration")
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var draftAttestation, confirmedAttestation string
	if err := conn.QueryRowContext(ctx, `SELECT attestation FROM time_entry WHERE id = ?`, draftID).Scan(&draftAttestation); err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if draftAttestation != "draft" {
		t.Fatalf("draft attestation = %q, want draft", draftAttestation)
	}
	if err := conn.QueryRowContext(ctx, `SELECT attestation FROM time_entry WHERE id = ?`, confirmedID).Scan(&confirmedAttestation); err != nil {
		t.Fatalf("read confirmed: %v", err)
	}
	if confirmedAttestation != "confirmed" {
		t.Fatalf("confirmed attestation = %q, want confirmed", confirmedAttestation)
	}

	var dismissedID int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T16:00:00Z', '2026-06-01T17:00:00Z', 60, '2026-06-01', 'soft reject', 'dismissed')
		RETURNING id
	`, periodID).Scan(&dismissedID); err != nil {
		t.Fatalf("insert dismissed: %v", err)
	}
	if dismissedID == 0 {
		t.Fatal("dismissed id = 0")
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO time_entry (
			period_id, start_instant, end_instant, duration_minutes,
			local_work_date, description, attestation
		) VALUES (?, '2026-06-01T17:00:00Z', '2026-06-01T18:00:00Z', 60, '2026-06-01', 'bad', 'rejected')
	`, periodID)
	if err == nil {
		t.Fatal("expected invalid attestation to fail")
	}
}
