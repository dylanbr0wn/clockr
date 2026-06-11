// Package seed populates a Clockr database with sensible starter data:
// default categories, the default-gap category, app-setting defaults, and (in
// dev mode) a sample period + calendar. Idempotent — safe to run repeatedly.
package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
)

// defaultCategories is the starter bucket set. The bool marks the default gap
// category (exactly one). Users edit these freely once running.
var defaultCategories = []struct {
	name         string
	isDefaultGap bool
}{
	{"Meetings", false},
	{"Deep Work", true},
	{"Admin", false},
	{"Email & Comms", false},
	{"Breaks", false},
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
	"period.cadence":      `"bi-weekly"`,
	"period.target_hours": `8`,
	// Default working-window start (local time-of-day). Window length = target hours.
	"window.start": `"09:00"`,
	"ai.base_url":         `""`,
	"ai.model":           `""`,
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
				IsDefaultGap: gap,
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
	return nil
}

// Dev seeds Core plus a sample calendar + period for local development.
func Dev(ctx context.Context, conn *sql.DB) error {
	if err := Core(ctx, conn); err != nil {
		return err
	}
	q := sqlc.New(conn)

	cal, err := q.UpsertCalendar(ctx, sqlc.UpsertCalendarParams{
		GoogleCalendarID: "primary",
		Name:             "Primary",
		IsPrimary:        1,
	})
	if err != nil {
		return fmt.Errorf("seed calendar: %w", err)
	}
	if err := q.SetCalendarSelected(ctx, sqlc.SetCalendarSelectedParams{Selected: 1, ID: cal.ID}); err != nil {
		return fmt.Errorf("select calendar: %w", err)
	}

	// A bi-weekly sample period; created lazily in the real app, eager here.
	const start, end = "2026-06-01", "2026-06-14"
	if _, err := q.GetPeriodByRange(ctx, sqlc.GetPeriodByRangeParams{StartDate: start, EndDate: end}); errors.Is(err, sql.ErrNoRows) {
		p, err := q.CreatePeriod(ctx, sqlc.CreatePeriodParams{
			StartDate:         start,
			EndDate:           end,
			Cadence:           "bi-weekly",
			AnchorDate:        start,
			TargetHoursPerDay: 8,
		})
		if err != nil {
			return fmt.Errorf("seed period: %w", err)
		}
		if _, err := q.UpsertTzSegment(ctx, sqlc.UpsertTzSegmentParams{
			PeriodID:          p.ID,
			EffectiveFromDate: start,
			IanaTz:            "America/Toronto",
		}); err != nil {
			return fmt.Errorf("seed tz segment: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("lookup sample period: %w", err)
	}
	return nil
}
