package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/seed"
)

func TestMigrateWorkSchedule_DropsFlatTargetAndSeedsDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "work-schedule.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := seed.Core(ctx, conn); err != nil {
		t.Fatalf("seed: %v", err)
	}

	periodCols := pragmaColumns(t, ctx, conn, "period")
	if periodCols["target_hours_per_day"] {
		t.Fatal("period still has target_hours_per_day")
	}

	for _, key := range []string{"period.target_hours", "window.start"} {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_setting WHERE key = ?`, key).Scan(&n); err != nil {
			t.Fatalf("count setting %q: %v", key, err)
		}
		if n != 0 {
			t.Fatalf("setting %q present after seed, want absent", key)
		}
	}

	var (
		schedID                                int64
		timezone, workweekStart, effectiveFrom string
		effectiveTo                            sql.NullString
	)
	err = conn.QueryRowContext(ctx, `
		SELECT id, timezone, workweek_start, effective_from, effective_to
		FROM work_schedule
	`).Scan(&schedID, &timezone, &workweekStart, &effectiveFrom, &effectiveTo)
	if err != nil {
		t.Fatalf("default work_schedule: %v", err)
	}
	if timezone != "America/Toronto" {
		t.Fatalf("timezone = %q, want America/Toronto", timezone)
	}
	if workweekStart != "monday" {
		t.Fatalf("workweek_start = %q, want monday", workweekStart)
	}
	if effectiveFrom == "" {
		t.Fatal("effective_from empty")
	}
	if effectiveTo.Valid {
		t.Fatalf("effective_to = %q, want NULL (open-ended)", effectiveTo.String)
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT d.weekday, d.expected_minutes,
			(SELECT COUNT(*) FROM work_schedule_window w WHERE w.work_schedule_day_id = d.id),
			COALESCE((SELECT MIN(w.start_minutes) FROM work_schedule_window w WHERE w.work_schedule_day_id = d.id), -1),
			COALESCE((SELECT MIN(w.end_minutes) FROM work_schedule_window w WHERE w.work_schedule_day_id = d.id), -1)
		FROM work_schedule_day d
		WHERE d.work_schedule_id = ?
		ORDER BY CASE d.weekday
			WHEN 'monday' THEN 1 WHEN 'tuesday' THEN 2 WHEN 'wednesday' THEN 3
			WHEN 'thursday' THEN 4 WHEN 'friday' THEN 5 WHEN 'saturday' THEN 6
			WHEN 'sunday' THEN 7 END
	`, schedID)
	if err != nil {
		t.Fatalf("list days: %v", err)
	}
	defer rows.Close()

	type dayRow struct {
		weekday         string
		expectedMinutes int
		windowCount     int
		startMinutes    int
		endMinutes      int
	}
	var days []dayRow
	for rows.Next() {
		var d dayRow
		if err := rows.Scan(&d.weekday, &d.expectedMinutes, &d.windowCount, &d.startMinutes, &d.endMinutes); err != nil {
			t.Fatalf("scan day: %v", err)
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("day rows: %v", err)
	}
	if len(days) != 7 {
		t.Fatalf("day count = %d, want 7", len(days))
	}

	wantWeekdays := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for i, want := range wantWeekdays {
		d := days[i]
		if d.weekday != want {
			t.Fatalf("day[%d].weekday = %q, want %q", i, d.weekday, want)
		}
		if i < 5 {
			if d.expectedMinutes != 480 {
				t.Fatalf("%s expected_minutes = %d, want 480", d.weekday, d.expectedMinutes)
			}
			if d.windowCount != 1 || d.startMinutes != 9*60 || d.endMinutes != 9*60+480 {
				t.Fatalf("%s window = count=%d start=%d end=%d, want 1× 09:00→+8h",
					d.weekday, d.windowCount, d.startMinutes, d.endMinutes)
			}
		} else if d.expectedMinutes != 0 || d.windowCount != 0 {
			t.Fatalf("%s expected=%d windows=%d, want 0/0", d.weekday, d.expectedMinutes, d.windowCount)
		}
	}

	var exceptionCount int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schedule_exception`).Scan(&exceptionCount); err != nil {
		t.Fatalf("count exceptions: %v", err)
	}
	if exceptionCount != 0 {
		t.Fatalf("exception count = %d, want 0", exceptionCount)
	}
}

func pragmaColumns(t *testing.T, ctx context.Context, conn *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatalf("pragma %s: %v", table, err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("pragma rows: %v", err)
	}
	return cols
}
