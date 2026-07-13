package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestCreateTimeEntry_IsConfirmedAndListable(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	entry, err := s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10*60 + 30,
		Description:  "New block",
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.PeriodID != pid || entry.LocalWorkDate != "2026-06-01" || entry.Description != "New block" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Attestation != "confirmed" {
		t.Fatalf("want attestation confirmed, got %q", entry.Attestation)
	}
	if entry.Method != "" {
		t.Fatalf("user create should not stamp method, got %q", entry.Method)
	}
	if entry.DurationMinutes != 90 {
		t.Fatalf("want duration 90, got %d", entry.DurationMinutes)
	}

	wantStart := time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 6, 1, 14, 30, 0, 0, time.UTC)
	gotStart, err := time.Parse(time.RFC3339, entry.Start)
	if err != nil {
		t.Fatal(err)
	}
	gotEnd, err := time.Parse(time.RFC3339, entry.End)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Fatalf("unexpected UTC span: %s to %s", entry.Start, entry.End)
	}

	listed, err := s.ListTimeEntries(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != entry.ID {
		t.Fatalf("entry was not listed: %+v", listed)
	}

	got, err := s.GetTimeEntry(ctx, entry.ID, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != entry.ID || got.Attestation != "confirmed" {
		t.Fatalf("get mismatch: %+v", got)
	}
}

func TestCreateGapTimeEntry_StampsMethod(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	entry, err := s.CreateGapTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Description:  "Confirmed gap",
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Attestation != "confirmed" {
		t.Fatalf("want confirmed, got %q", entry.Attestation)
	}
	if entry.Method != "gap_fill" {
		t.Fatalf("want method gap_fill, got %q", entry.Method)
	}
}

func TestCreateTimeEntryValidation(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	_, err = s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 10 * 60,
		EndMinutes:   10 * 60,
	})
	if err == nil {
		t.Fatal("expected invalid range error")
	}

	_, err = s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-15",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err == nil {
		t.Fatal("expected out-of-period error")
	}
}

func TestCreateTimeEntry_AllocationDefaults(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	entry, err := s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.WorkType != "worked" {
		t.Fatalf("work_type = %q want worked", entry.WorkType)
	}
	if entry.BillableStatus != "unset" {
		t.Fatalf("billable_status = %q want unset", entry.BillableStatus)
	}
	if entry.ProjectID != nil {
		t.Fatalf("project_id = %v want nil", entry.ProjectID)
	}
}

func TestCreateAndUpdateTimeEntry_AllocationFields(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	project, err := s.CreateProject(ctx, service.CreateProjectInput{Name: "Apollo", Key: "apollo"})
	if err != nil {
		t.Fatal(err)
	}
	projectID := project.ID

	entry, err := s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:       pid,
		Day:            "2026-06-01",
		StartMinutes:   9 * 60,
		EndMinutes:     10 * 60,
		WorkType:       "paid_leave",
		ProjectID:      &projectID,
		BillableStatus: "non_billable",
		Description:    "PTO",
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.WorkType != "paid_leave" {
		t.Fatalf("work_type = %q want paid_leave", entry.WorkType)
	}
	if entry.BillableStatus != "non_billable" {
		t.Fatalf("billable_status = %q want non_billable", entry.BillableStatus)
	}
	if entry.ProjectID == nil || *entry.ProjectID != projectID {
		t.Fatalf("project_id = %v want %d", entry.ProjectID, projectID)
	}

	listed, err := s.ListTimeEntries(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].WorkType != "paid_leave" || listed[0].BillableStatus != "non_billable" {
		t.Fatalf("list allocation mismatch: %+v", listed)
	}
	if listed[0].ProjectID == nil || *listed[0].ProjectID != projectID {
		t.Fatalf("list project_id = %v", listed[0].ProjectID)
	}

	updated, err := s.UpdateTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: entry.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:       pid,
			Day:            "2026-06-01",
			StartMinutes:   9 * 60,
			EndMinutes:     10 * 60,
			WorkType:       "worked",
			ProjectID:      &projectID,
			BillableStatus: "billable",
			Description:    "Back to work",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.WorkType != "worked" || updated.BillableStatus != "billable" {
		t.Fatalf("update allocation: %+v", updated)
	}

	cleared, err := s.UpdateTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: entry.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:       pid,
			Day:            "2026-06-01",
			StartMinutes:   9 * 60,
			EndMinutes:     10 * 60,
			WorkType:       "break",
			BillableStatus: "unset",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.WorkType != "break" || cleared.BillableStatus != "unset" || cleared.ProjectID != nil {
		t.Fatalf("clear project: %+v", cleared)
	}
}

func TestCreateTimeEntry_InvalidAllocation(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	_, err = s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		WorkType:     "overtime",
	})
	if err == nil {
		t.Fatal("expected invalid work_type error")
	}

	_, err = s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:       pid,
		Day:            "2026-06-01",
		StartMinutes:   9 * 60,
		EndMinutes:     10 * 60,
		BillableStatus: "maybe",
	})
	if err == nil {
		t.Fatal("expected invalid billable_status error")
	}

	badID := int64(999999)
	_, err = s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		ProjectID:    &badID,
	})
	if err == nil {
		t.Fatal("expected unknown project_id error")
	}
}

func TestUpdateAndDeleteTimeEntry(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	entry, err := s.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Description:  "Temp",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: entry.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     pid,
			Day:          "2026-06-02",
			StartMinutes: 11 * 60,
			EndMinutes:   12 * 60,
			Description:  "Moved",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LocalWorkDate != "2026-06-02" || updated.Description != "Moved" || updated.Attestation != "confirmed" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	if err := s.DeleteTimeEntry(ctx, service.TimeEntryDeleteInput{ID: entry.ID, PeriodID: pid}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListTimeEntries(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("not deleted: %+v", listed)
	}
	err = s.DeleteTimeEntry(ctx, service.TimeEntryDeleteInput{ID: entry.ID, PeriodID: pid})
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
