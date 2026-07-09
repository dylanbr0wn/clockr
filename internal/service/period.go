package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// EnsureCurrentPeriod returns the period containing today, creating it and its
// initial timezone segment when needed.
func (s *Service) EnsureCurrentPeriod(ctx context.Context, today string, ianaTz string) (Period, error) {
	if today == "" {
		return Period{}, fmt.Errorf("ensure current period: today is required")
	}
	if ianaTz == "" {
		ianaTz = "UTC"
	}

	todayDate, err := parseDate(today)
	if err != nil {
		return Period{}, fmt.Errorf("ensure current period: today: %w", err)
	}

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		return Period{}, err
	}
	for _, period := range periods {
		if periodContains(period, today) {
			if err := s.ensurePeriodTzSegment(ctx, period.ID, period.StartDate, ianaTz); err != nil {
				return Period{}, err
			}
			return period, nil
		}
	}

	cadence, err := s.periodCadence(ctx, periods)
	if err != nil {
		return Period{}, err
	}
	targetHours, err := s.periodTargetHours(ctx, periods)
	if err != nil {
		return Period{}, err
	}
	anchor := periodAnchor(periods, today)
	start, end := currentPeriodRange(todayDate, anchor, cadence)

	period, err := s.GetPeriodByRange(ctx, start, end)
	if err == nil {
		if err := s.ensurePeriodTzSegment(ctx, period.ID, period.StartDate, ianaTz); err != nil {
			return Period{}, err
		}
		return period, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Period{}, err
	}

	row, err := s.q.CreatePeriod(ctx, sqlc.CreatePeriodParams{
		StartDate:         start,
		EndDate:           end,
		Cadence:           cadence,
		AnchorDate:        anchor,
		TargetHoursPerDay: targetHours,
	})
	if err != nil {
		return Period{}, mapErr("ensure current period", err)
	}
	period = toPeriod(row)

	if err := s.ensurePeriodTzSegment(ctx, period.ID, period.StartDate, ianaTz); err != nil {
		return Period{}, err
	}
	return period, nil
}

func (s *Service) ensurePeriodTzSegment(ctx context.Context, periodID int64, startDate string, ianaTz string) error {
	segs, err := s.ListTzSegments(ctx, periodID)
	if err != nil {
		return err
	}
	if len(segs) > 0 {
		return nil
	}
	if _, err := s.q.UpsertTzSegment(ctx, sqlc.UpsertTzSegmentParams{
		PeriodID:          periodID,
		EffectiveFromDate: startDate,
		IanaTz:            ianaTz,
	}); err != nil {
		return mapErr("ensure period timezone", err)
	}
	return nil
}

func (s *Service) periodCadence(ctx context.Context, periods []Period) (string, error) {
	if len(periods) > 0 && periods[0].Cadence != "" {
		return periods[0].Cadence, nil
	}
	raw, err := s.GetSetting(ctx, "period.cadence")
	if err != nil {
		return "bi-weekly", nil
	}
	var cadence string
	if json.Unmarshal([]byte(raw), &cadence) != nil || cadence == "" {
		return "bi-weekly", nil
	}
	return cadence, nil
}

func (s *Service) periodTargetHours(ctx context.Context, periods []Period) (float64, error) {
	if len(periods) > 0 && periods[0].TargetHoursPerDay > 0 {
		return periods[0].TargetHoursPerDay, nil
	}
	raw, err := s.GetSetting(ctx, "period.target_hours")
	if err != nil {
		return 8, nil
	}
	var target float64
	if json.Unmarshal([]byte(raw), &target) != nil || target <= 0 {
		return 8, nil
	}
	return target, nil
}

func periodContains(period Period, day string) bool {
	return period.StartDate <= day && day <= period.EndDate
}

func periodAnchor(periods []Period, today string) string {
	if len(periods) == 0 || periods[0].AnchorDate == "" {
		return today
	}
	return periods[0].AnchorDate
}

func currentPeriodRange(today time.Time, anchorDate string, cadence string) (string, string) {
	switch cadence {
	case "weekly":
		return fixedDayPeriodRange(today, anchorDate, 7)
	case "bi-weekly":
		return fixedDayPeriodRange(today, anchorDate, 14)
	case "semi-monthly":
		if today.Day() <= 15 {
			return dateKey(today.Year(), today.Month(), 1), dateKey(today.Year(), today.Month(), 15)
		}
		return dateKey(today.Year(), today.Month(), 16), dateKey(today.Year(), today.Month(), daysInMonth(today.Year(), today.Month()))
	case "monthly":
		return dateKey(today.Year(), today.Month(), 1), dateKey(today.Year(), today.Month(), daysInMonth(today.Year(), today.Month()))
	default:
		return fixedDayPeriodRange(today, anchorDate, 14)
	}
}

func fixedDayPeriodRange(today time.Time, anchorDate string, length int) (string, string) {
	anchor, err := parseDate(anchorDate)
	if err != nil {
		anchor = today
	}
	daysSinceAnchor := int(today.Sub(anchor).Hours() / 24)
	offset := daysSinceAnchor % length
	if offset < 0 {
		offset += length
	}
	start := today.AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, length-1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func dateKey(year int, month time.Month, day int) string {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
