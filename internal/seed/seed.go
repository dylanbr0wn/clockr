// Package seed populates a shiet database with sensible starter data:
// default categories, the default-gap category, app-setting defaults, and (in
// dev mode) a sample period + calendar. Idempotent — safe to run repeatedly.
package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// defaultCategories is the starter bucket set. The bool marks the default gap
// category (exactly one). Users edit these freely once running.
var defaultCategories = []struct {
	name         string
	description  string
	color        string
	isDefaultGap bool
}{
	{"Meetings", "Syncs, 1:1s, standups, and client calls", "#0EA5E9", false},
	{"Deep Work", "Focused individual work without meetings", "#8B5CF6", true},
	{"Admin", "Operations, planning, and internal admin tasks", "#F59E0B", false},
	{"Email & Comms", "Email, Slack, and async communication", "#14B8A6", false},
	{"Breaks", "Lunch, breaks, and personal time", "#64748B", false},
}

// defaultSettings are non-secret app defaults mirroring DESIGN.md. Values are
// JSON-encoded strings (the app_setting.value column is JSON text).
var defaultSettings = map[string]string{
	// Cloud AI payload field toggles (privacy floor). Description off by default.
	"privacy.fields":      `{"title":true,"attendees":true,"description":false,"location":false}`,
	"privacy.confirmed":   `false`,
	"events.declined":     `"exclude"`,
	"events.accepted":     `"include"`,
	"events.tentative":    `"flag"`,
	"events.all_day":      `"flag"`,
	"app.theme":      `"system"`,
	"period.cadence": `"bi-weekly"`,
	"ai.base_url":    `""`,
	"ai.model":       `""`,
	"ai.max_tokens":  `512`,
}

// Core seeds data that every install needs (categories + settings).
func Core(ctx context.Context, conn *sql.DB) error {
	q := sqlc.New(conn)

	existing, err := q.ListCategories(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	if len(existing) == 0 {
		for _, c := range defaultCategories {
			gap := int64(0)
			if c.isDefaultGap {
				gap = 1
			}
			if _, err := q.CreateCategory(ctx, sqlc.CreateCategoryParams{
				Name:         c.name,
				Description:  c.description,
				Key:          c.name,
				IsDefaultGap: gap,
				Color:        c.color,
			}); err != nil {
				return fmt.Errorf("create category %q: %w", c.name, err)
			}
		}
	}

	for k, v := range defaultSettings {
		if _, err := q.GetSetting(ctx, k); errors.Is(err, sql.ErrNoRows) {
			if err := q.SetSetting(ctx, sqlc.SetSettingParams{Key: k, Value: v}); err != nil {
				return fmt.Errorf("set setting %q: %w", k, err)
			}
		} else if err != nil {
			return fmt.Errorf("get setting %q: %w", k, err)
		}
	}

	if err := seedDefaultWorkSchedule(ctx, q); err != nil {
		return err
	}
	return nil
}

// seedDefaultWorkSchedule inserts the open-ended Mon–Fri 8h template when none exist.
func seedDefaultWorkSchedule(ctx context.Context, q *sqlc.Queries) error {
	n, err := q.CountWorkSchedules(ctx)
	if err != nil {
		return fmt.Errorf("count work schedules: %w", err)
	}
	if n > 0 {
		return nil
	}

	sched, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "America/Toronto",
		WorkweekStart: "monday",
		EffectiveFrom: "1970-01-01",
		EffectiveTo:   sql.NullString{},
	})
	if err != nil {
		return fmt.Errorf("create default work schedule: %w", err)
	}

	weekdays := []struct {
		name    string
		minutes int64
		window  bool
	}{
		{"monday", 480, true},
		{"tuesday", 480, true},
		{"wednesday", 480, true},
		{"thursday", 480, true},
		{"friday", 480, true},
		{"saturday", 0, false},
		{"sunday", 0, false},
	}
	const windowStart, windowEnd = int64(9 * 60), int64(9*60 + 480)
	for _, day := range weekdays {
		row, err := q.CreateWorkScheduleDay(ctx, sqlc.CreateWorkScheduleDayParams{
			WorkScheduleID:  sched.ID,
			Weekday:         day.name,
			ExpectedMinutes: day.minutes,
		})
		if err != nil {
			return fmt.Errorf("create schedule day %q: %w", day.name, err)
		}
		if !day.window {
			continue
		}
		if _, err := q.CreateWorkScheduleWindow(ctx, sqlc.CreateWorkScheduleWindowParams{
			WorkScheduleDayID: row.ID,
			StartMinutes:      windowStart,
			EndMinutes:        windowEnd,
		}); err != nil {
			return fmt.Errorf("create schedule window for %q: %w", day.name, err)
		}
	}
	return nil
}

// Dev seeds Core plus a sample calendar + period for local development.
func Dev(ctx context.Context, conn *sql.DB) error {
	if err := Core(ctx, conn); err != nil {
		return err
	}
	q := sqlc.New(conn)

	cal, err := q.UpsertCalendar(ctx, sqlc.UpsertCalendarParams{
		Provider:   "google",
		ExternalID: "primary",
		Name:       "Primary",
		IsPrimary:  1,
		Column5:    int64(1),
	})
	if err != nil {
		return fmt.Errorf("seed calendar: %w", err)
	}
	if err := q.SetCalendarSelected(ctx, sqlc.SetCalendarSelectedParams{Selected: 1, ID: cal.ID}); err != nil {
		return fmt.Errorf("select calendar: %w", err)
	}

	// A bi-weekly sample period; created lazily in the real app, eager here.
	const start, end = "2026-06-01", "2026-06-14"
	var period sqlc.Period
	if _, err := q.GetPeriodByRange(ctx, sqlc.GetPeriodByRangeParams{StartDate: start, EndDate: end}); errors.Is(err, sql.ErrNoRows) {
		p, err := q.CreatePeriod(ctx, sqlc.CreatePeriodParams{
			StartDate:  start,
			EndDate:    end,
			Cadence:    "bi-weekly",
			AnchorDate: start,
		})
		if err != nil {
			return fmt.Errorf("seed period: %w", err)
		}
		period = p
		if _, err := q.UpsertTzSegment(ctx, sqlc.UpsertTzSegmentParams{
			PeriodID:          p.ID,
			EffectiveFromDate: start,
			IanaTz:            "America/Toronto",
		}); err != nil {
			return fmt.Errorf("seed tz segment: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("lookup sample period: %w", err)
	} else {
		period, err = q.GetPeriodByRange(ctx, sqlc.GetPeriodByRangeParams{StartDate: start, EndDate: end})
		if err != nil {
			return fmt.Errorf("load sample period: %w", err)
		}
	}

	if err := seedDevEvents(ctx, q, period.ID, cal.ID); err != nil {
		return err
	}
	return nil
}

func seedDevEvents(ctx context.Context, q *sqlc.Queries, periodID, calendarID int64) error {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return fmt.Errorf("load dev timezone: %w", err)
	}

	events := []struct {
		externalID string
		title      string
		day        string
		startMin   int
		endMin     int
	}{
		{
			externalID: "dev-sprint-planning",
			title:      "Sprint planning",
			day:        "2026-06-02",
			startMin:   8*60 + 30,
			endMin:     10 * 60,
		},
		{
			externalID: "dev-design-review",
			title:      "Design review",
			day:        "2026-06-02",
			startMin:   9*60 + 15,
			endMin:     10*60 + 30,
		},
		{
			externalID: "dev-vendor-call",
			title:      "Vendor call",
			day:        "2026-06-04",
			startMin:   7 * 60,
			endMin:     8 * 60,
		},
	}

	for _, event := range events {
		start := localMinute(event.day, event.startMin, loc)
		end := localMinute(event.day, event.endMin, loc)
		if _, err := q.UpsertEvent(ctx, sqlc.UpsertEventParams{
			PeriodID:   periodID,
			CalendarID: calendarID,
			Provider:   "google",
			ExternalID: event.externalID,
			IcalUid:    event.externalID + "@shiet.dev",
			Title:      event.title,
			Attendees:  "[]",
			Status:     "accepted",
			StartUtc: sql.NullString{
				String: start.UTC().Format(time.RFC3339),
				Valid:  true,
			},
			EndUtc: sql.NullString{
				String: end.UTC().Format(time.RFC3339),
				Valid:  true,
			},
			OriginalTz: "America/Toronto",
			SourceHash: "dev-seed-v1:" + event.externalID,
		}); err != nil {
			return fmt.Errorf("seed dev event %q: %w", event.externalID, err)
		}
	}

	return nil
}

func localMinute(day string, minute int, loc *time.Location) time.Time {
	date, err := time.ParseInLocation("2006-01-02", day, loc)
	if err != nil {
		return time.Time{}
	}

	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		0,
		minute,
		0,
		0,
		loc,
	)
}
