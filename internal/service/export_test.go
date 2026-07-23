package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestListExportTemplates_SeedsMatrixBuiltin(t *testing.T) {
	s := newSvc(t)
	tmpls, err := s.ListExportTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpls) == 0 {
		t.Fatal("want at least one export template")
	}
	foundMatrix := false
	foundFlat := false
	foundDetail := false
	foundText := false
	for _, tmpl := range tmpls {
		switch tmpl.Key {
		case service.ExportTemplateMatrixCSV:
			foundMatrix = true
			if !tmpl.Builtin {
				t.Fatal("matrix_csv should be builtin")
			}
			if tmpl.Format != "csv" {
				t.Fatalf("format = %q want csv", tmpl.Format)
			}
		case service.ExportTemplateFlatDailyCSV:
			foundFlat = true
			if !tmpl.Builtin {
				t.Fatal("flat_daily_csv should be builtin")
			}
			if tmpl.Format != "csv" {
				t.Fatalf("format = %q want csv", tmpl.Format)
			}
		case service.ExportTemplateDetailEntriesCSV:
			foundDetail = true
			if !tmpl.Builtin {
				t.Fatal("detail_entries_csv should be builtin")
			}
			if tmpl.Format != "csv" {
				t.Fatalf("format = %q want csv", tmpl.Format)
			}
		case service.ExportTemplateTextSummary:
			foundText = true
			if !tmpl.Builtin {
				t.Fatal("text_summary should be builtin")
			}
			if tmpl.Format != "text" {
				t.Fatalf("format = %q want text", tmpl.Format)
			}
			if strings.TrimSpace(tmpl.Body) == "" {
				t.Fatal("text_summary body should contain text/template")
			}
		}
	}
	if !foundMatrix {
		t.Fatal("matrix_csv builtin missing")
	}
	if !foundFlat {
		t.Fatal("flat_daily_csv builtin missing")
	}
	if !foundDetail {
		t.Fatal("detail_entries_csv builtin missing")
	}
	if !foundText {
		t.Fatal("text_summary builtin missing")
	}
}

func TestBuildPeriodExport_EntriesAndRollups(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings, deepWork service.Category
	for _, c := range cats {
		switch c.Name {
		case "Meetings":
			meetings = c
		case "Deep Work":
			deepWork = c
		}
	}
	if meetings.ID == 0 || deepWork.ID == 0 {
		t.Fatal("seeded Meetings/Deep Work categories missing")
	}

	// Calendar event remains busy evidence but is not payable.
	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}

	// Confirmed entries: 13:00–15:00 EDT day 1 + 10:00–12:00 EDT day 2 → 4h Deep Work
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "Focus", false)
	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T14:00:00Z", "2026-06-02T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "Planning", false)

	model, err := e.svc.BuildPeriodExport(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}

	if model.StartDate != "2026-06-01" || model.EndDate != "2026-06-02" {
		t.Fatalf("dates = %s..%s", model.StartDate, model.EndDate)
	}
	if len(model.Days) != 2 {
		t.Fatalf("days = %v want 2", model.Days)
	}
	if model.ActualMinutes != 240 {
		t.Fatalf("actualMinutes = %d want 240 (confirmed only)", model.ActualMinutes)
	}
	if model.TargetMinutes != 8*60*2 {
		t.Fatalf("targetMinutes = %d want %d", model.TargetMinutes, 8*60*2)
	}
	if len(model.Entries) != 2 {
		t.Fatalf("entries = %d want 2", len(model.Entries))
	}

	byName := map[string]service.ExportCategoryMinutes{}
	for _, total := range model.PeriodTotals {
		byName[total.Category.Name] = total
		if total.Category.Key == "" {
			t.Fatalf("category %q missing key", total.Category.Name)
		}
	}
	if _, ok := byName["Meetings"]; ok {
		t.Fatalf("Meetings must not be payable from calendar: %+v", byName["Meetings"])
	}
	if byName["Deep Work"].Minutes != 240 {
		t.Fatalf("Deep Work total = %d want 240", byName["Deep Work"].Minutes)
	}

	if len(model.DailyTotals) != 2 {
		t.Fatalf("dailyTotals len = %d", len(model.DailyTotals))
	}
	if model.DailyTotals[0].TargetMinutes != 480 || model.DailyTotals[1].TargetMinutes != 480 {
		t.Fatalf("weekday daily targets = %d, %d want 480 each", model.DailyTotals[0].TargetMinutes, model.DailyTotals[1].TargetMinutes)
	}
	day1 := categoryMinutesByName(model.DailyTotals[0])
	if day1["Deep Work"] != 120 || day1["Meetings"] != 0 {
		t.Fatalf("day1 categories = %+v", day1)
	}
	day2 := categoryMinutesByName(model.DailyTotals[1])
	if day2["Deep Work"] != 120 {
		t.Fatalf("day2 categories = %+v", day2)
	}
}

func TestBuildPeriodExport_OnlyConfirmedTimeEntriesArePayable(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	if deepWork.ID == 0 {
		t.Fatal("seeded Deep Work category missing")
	}
	catID := sql.NullInt64{Int64: deepWork.ID, Valid: true}

	// Raw calendar event must not count as payable.
	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: catID,
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}

	insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T18:00:00Z", catID, "Draft proposal", "draft", false)
	insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01", "2026-06-01T18:00:00Z", "2026-06-01T19:00:00Z", catID, "Dismissed proposal", "dismissed", false)
	insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01", "2026-06-01T19:00:00Z", "2026-06-01T20:00:00Z", catID, "Confirmed work", "confirmed", false)

	model, err := e.svc.BuildPeriodExport(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}

	if model.ActualMinutes != 60 {
		t.Fatalf("actualMinutes = %d want 60 (confirmed only)", model.ActualMinutes)
	}
	if len(model.Entries) != 1 {
		t.Fatalf("entries = %d want 1 confirmed", len(model.Entries))
	}
	got := model.Entries[0]
	if got.Source != "time_entry" || got.Title != "Confirmed work" || got.Minutes != 60 {
		t.Fatalf("payable entry = %+v", got)
	}
	if len(model.PeriodTotals) != 1 || model.PeriodTotals[0].Category.Name != "Deep Work" || model.PeriodTotals[0].Minutes != 60 {
		t.Fatalf("periodTotals = %+v", model.PeriodTotals)
	}
	if model.DailyTotals[0].ActualMinutes != 60 {
		t.Fatalf("day actual = %d want 60", model.DailyTotals[0].ActualMinutes)
	}
}

func TestBuildPeriodExport_WeekendDaysHaveZeroTarget(t *testing.T) {
	// Fri–Sun: only Friday carries seeded expected minutes.
	e := newGapEnv(t, "2026-06-05", "2026-06-07", "America/Toronto")
	model, err := e.svc.BuildPeriodExport(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if model.TargetMinutes != 480 {
		t.Fatalf("targetMinutes = %d want 480 (Friday only)", model.TargetMinutes)
	}
	if len(model.DailyTotals) != 3 {
		t.Fatalf("dailyTotals len = %d", len(model.DailyTotals))
	}
	if model.DailyTotals[0].TargetMinutes != 480 {
		t.Fatalf("Friday target = %d want 480", model.DailyTotals[0].TargetMinutes)
	}
	if model.DailyTotals[1].TargetMinutes != 0 || model.DailyTotals[2].TargetMinutes != 0 {
		t.Fatalf("weekend targets = %d, %d want 0", model.DailyTotals[1].TargetMinutes, model.DailyTotals[2].TargetMinutes)
	}
}

func TestRenderPeriodExport_MatrixCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings, deepWork service.Category
	for _, c := range cats {
		switch c.Name {
		case "Meetings":
			meetings = c
		case "Deep Work":
			deepWork = c
		}
	}

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)
	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T14:00:00Z", "2026-06-02T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateMatrixCSV)
	if err != nil {
		t.Fatal(err)
	}
	if render.Filename != "shiet-2026-06-01-to-2026-06-02.csv" {
		t.Fatalf("filename = %q", render.Filename)
	}
	if render.Format != "csv" {
		t.Fatalf("format = %q", render.Format)
	}

	want := strings.Join([]string{
		"Category,2026-06-01,2026-06-02,Total",
		"Deep Work,2.00,2.00,4.00",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_FlatDailyCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings, deepWork service.Category
	for _, c := range cats {
		switch c.Name {
		case "Meetings":
			meetings = c
		case "Deep Work":
			deepWork = c
		}
	}

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)
	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T14:00:00Z", "2026-06-02T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateFlatDailyCSV)
	if err != nil {
		t.Fatal(err)
	}
	if render.Filename != "shiet-2026-06-01-to-2026-06-02.csv" {
		t.Fatalf("filename = %q", render.Filename)
	}
	if render.Format != "csv" {
		t.Fatalf("format = %q", render.Format)
	}

	want := strings.Join([]string{
		"Date,Category,Key,Hours",
		"2026-06-01,Deep Work," + deepWork.Key + ",2.00",
		"2026-06-02,Deep Work," + deepWork.Key + ",2.00",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_DetailEntriesCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings, deepWork service.Category
	for _, c := range cats {
		switch c.Name {
		case "Meetings":
			meetings = c
		case "Deep Work":
			deepWork = c
		}
	}

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z", "Google notes")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "User notes", false)
	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T14:00:00Z", "2026-06-02T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "Planning", false)

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateDetailEntriesCSV)
	if err != nil {
		t.Fatal(err)
	}
	if render.Filename != "shiet-2026-06-01-to-2026-06-02.csv" {
		t.Fatalf("filename = %q", render.Filename)
	}
	if render.Format != "csv" {
		t.Fatalf("format = %q", render.Format)
	}

	want := strings.Join([]string{
		"Start,End,Category,Key,Hours,Title,Description,Work type,Project,Project key,Billable",
		"2026-06-01T13:00,2026-06-01T15:00,Deep Work," + deepWork.Key + ",2.00,User notes,User notes,worked,,,unset",
		"2026-06-02T10:00,2026-06-02T12:00,Deep Work," + deepWork.Key + ",2.00,Planning,Planning,worked,,,unset",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestBuildPeriodExport_AllocationFieldsOnTimeEntriesOnly(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	if deepWork.ID == 0 {
		t.Fatal("seeded Deep Work category missing")
	}

	project, err := e.svc.CreateProject(ctx, service.CreateProjectInput{Name: "Apollo", Key: "apollo"})
	if err != nil {
		t.Fatal(err)
	}
	projectID := project.ID
	catID := deepWork.ID

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:       e.periodID,
		Day:            "2026-06-01",
		StartMinutes:   13 * 60,
		EndMinutes:     15 * 60,
		CategoryID:     &catID,
		Description:    "Feature work",
		WorkType:       "worked",
		ProjectID:      &projectID,
		BillableStatus: "billable",
	}); err != nil {
		t.Fatal(err)
	}

	model, err := e.svc.BuildPeriodExport(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Entries) != 1 {
		t.Fatalf("entries = %d want 1 confirmed time entry", len(model.Entries))
	}

	timeEntry := model.Entries[0]
	if timeEntry.Source != "time_entry" {
		t.Fatalf("source = %q want time_entry", timeEntry.Source)
	}
	if timeEntry.WorkType != "worked" {
		t.Fatalf("time_entry work_type = %q want worked", timeEntry.WorkType)
	}
	if timeEntry.ProjectName != "Apollo" || timeEntry.ProjectKey != "apollo" {
		t.Fatalf("time_entry project = %q/%q want Apollo/apollo", timeEntry.ProjectName, timeEntry.ProjectKey)
	}
	if timeEntry.BillableStatus != "billable" {
		t.Fatalf("time_entry billable_status = %q want billable", timeEntry.BillableStatus)
	}
}

func TestRenderPeriodExport_DetailEntriesCSVAllocationColumns(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	if deepWork.ID == 0 {
		t.Fatal("seeded Deep Work category missing")
	}

	project, err := e.svc.CreateProject(ctx, service.CreateProjectInput{Name: "Apollo", Key: "apollo"})
	if err != nil {
		t.Fatal(err)
	}
	projectID := project.ID
	catID := deepWork.ID

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z", "Calendar notes")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:       e.periodID,
		Day:            "2026-06-01",
		StartMinutes:   13 * 60,
		EndMinutes:     15 * 60,
		CategoryID:     &catID,
		Description:    "Feature work",
		WorkType:       "worked",
		ProjectID:      &projectID,
		BillableStatus: "billable",
	}); err != nil {
		t.Fatal(err)
	}

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateDetailEntriesCSV)
	if err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		"Start,End,Category,Key,Hours,Title,Description,Work type,Project,Project key,Billable",
		"2026-06-01T13:00,2026-06-01T15:00,Deep Work," + deepWork.Key + ",2.00,Feature work,Feature work,worked,Apollo,apollo,billable",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestBuildPeriodExport_EntryDescriptions(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings service.Category
	for _, c := range cats {
		if c.Name == "Meetings" {
			meetings = c
		}
	}
	if meetings.ID == 0 {
		t.Fatal("seeded Meetings category missing")
	}

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z", "Calendar agenda")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: meetings.ID, Valid: true}, "Longer work notes", false)

	model, err := e.svc.BuildPeriodExport(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Entries) != 1 {
		t.Fatalf("entries = %d want 1 confirmed time entry", len(model.Entries))
	}

	timeEntry := model.Entries[0]
	if timeEntry.Description != "Longer work notes" {
		t.Fatalf("time entry description = %q want Longer work notes", timeEntry.Description)
	}
	if timeEntry.Title != "Longer work notes" {
		t.Fatalf("time entry title = %q want Longer work notes", timeEntry.Title)
	}
}

func TestRenderPeriodExport_TextSummaryShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings, deepWork service.Category
	for _, c := range cats {
		switch c.Name {
		case "Meetings":
			meetings = c
		case "Deep Work":
			deepWork = c
		}
	}

	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T17:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T17:00:00Z", "2026-06-01T19:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)
	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T14:00:00Z", "2026-06-02T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateTextSummary)
	if err != nil {
		t.Fatal(err)
	}
	if render.Format != "text" {
		t.Fatalf("format = %q", render.Format)
	}
	if render.Filename != "shiet-2026-06-01-to-2026-06-02.txt" {
		t.Fatalf("filename = %q", render.Filename)
	}

	want := strings.Join([]string{
		"Period: Jun 1-Jun 2",
		"2026-06-01 to 2026-06-02",
		"",
		"Target: 16h",
		"Actual: 4h",
		"Variance: -12h",
		"",
		"Totals by category:",
		"  Deep Work: 4h",
		"",
		"Daily breakdown:",
		"2026-06-01 — 2h / 8h target",
		"  Deep Work: 2h",
		"",
		"2026-06-02 — 2h / 8h target",
		"  Deep Work: 2h",
	}, "\n")
	if render.Content != want {
		t.Fatalf("text mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_TextSummaryEmptyDay(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T14:00:00Z", "2026-06-01T15:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, service.ExportTemplateTextSummary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render.Content, "2026-06-02 — 0m / 8h target") {
		t.Fatalf("missing empty day line:\n%s", render.Content)
	}
	if !strings.Contains(render.Content, "(no tracked time)") {
		t.Fatalf("missing empty-day marker:\n%s", render.Content)
	}
}

func TestBuildPeriodExport_CalendarEventNotPayable(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	e.addEvent(t, "bare", "2026-06-01T15:00:00Z", "2026-06-01T16:00:00Z")

	model, err := e.svc.BuildPeriodExport(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if model.ActualMinutes != 0 {
		t.Fatalf("actualMinutes = %d want 0", model.ActualMinutes)
	}
	if len(model.Entries) != 0 {
		t.Fatalf("entries = %+v want none", model.Entries)
	}
	if len(model.PeriodTotals) != 0 {
		t.Fatalf("periodTotals = %+v want empty", model.PeriodTotals)
	}
}

func categoryMinutesByName(day service.ExportDayTotals) map[string]int {
	out := map[string]int{}
	for _, c := range day.Categories {
		out[c.Category.Name] = c.Minutes
	}
	return out
}

func TestExportTemplateCRUD_CustomTabular(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	body, err := json.Marshal(service.TabularTemplateSpec{
		Version:   1,
		Grain:     service.ExportGrainRollup,
		Layout:    service.ExportLayoutFlat,
		Delimiter: ",",
		Columns: []service.TabularColumnSpec{
			{Field: service.ExportFieldDate, Header: "Day"},
			{Field: service.ExportFieldCategoryKey, Header: "Code"},
			{Field: service.ExportFieldHours, Header: "Hrs"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := e.svc.CreateExportTemplate(ctx, service.CreateExportTemplateInput{
		Name:        "Custom flat",
		Description: "Test custom",
		Format:      "csv",
		Body:        string(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Builtin {
		t.Fatal("custom template should not be builtin")
	}
	if created.Key == "" {
		t.Fatal("expected generated key")
	}

	preview, err := e.svc.PreviewExport(ctx, service.PreviewExportInput{
		PeriodID:    e.periodID,
		TemplateKey: created.Key,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(preview.Content, "\n")
	if lines[0] != "Day,Code,Hrs" {
		t.Fatalf("header = %q", lines[0])
	}
	if !strings.Contains(preview.Content, "2026-06-01,"+deepWork.Key+",2.00") {
		t.Fatalf("missing custom row:\n%s", preview.Content)
	}

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, created.Key)
	if err != nil {
		t.Fatal(err)
	}
	if render.Content != preview.Content {
		t.Fatalf("render/preview mismatch\nrender:\n%s\npreview:\n%s", render.Content, preview.Content)
	}

	updatedBody, err := json.Marshal(service.TabularTemplateSpec{
		Version:   1,
		Grain:     service.ExportGrainRollup,
		Layout:    service.ExportLayoutFlat,
		Delimiter: "\t",
		Columns: []service.TabularColumnSpec{
			{Field: service.ExportFieldCategoryName, Header: "Name"},
			{Field: service.ExportFieldMinutes, Header: "Mins"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := e.svc.UpdateExportTemplate(ctx, service.UpdateExportTemplateInput{
		ID:          created.ID,
		Name:        "Custom flat TSV",
		Description: "tsv",
		Format:      "tsv",
		Body:        string(updatedBody),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Format != "tsv" {
		t.Fatalf("format = %q", updated.Format)
	}

	dup, err := e.svc.DuplicateExportTemplate(ctx, service.ExportTemplateMatrixCSV)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Builtin {
		t.Fatal("duplicate should be custom")
	}
	if !strings.Contains(dup.Name, "(copy)") {
		t.Fatalf("name = %q", dup.Name)
	}

	matrix, err := e.svc.GetExportTemplate(ctx, service.ExportTemplateMatrixCSV)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateExportTemplate(ctx, service.UpdateExportTemplateInput{
		ID:     matrix.ID,
		Name:   "nope",
		Format: "csv",
		Body:   string(body),
	}); !errors.Is(err, service.ErrExportTemplateBuiltin) {
		t.Fatalf("want ErrExportTemplateBuiltin on update, got %v", err)
	}
	if err := e.svc.DeleteExportTemplate(ctx, matrix.ID); !errors.Is(err, service.ErrExportTemplateBuiltin) {
		t.Fatalf("want ErrExportTemplateBuiltin on delete, got %v", err)
	}

	if err := e.svc.DeleteExportTemplate(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.GetExportTemplate(ctx, created.Key); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPreviewExport_DraftBody(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meetings service.Category
	for _, c := range cats {
		if c.Name == "Meetings" {
			meetings = c
		}
	}
	e.addEvent(t, "meet-1", "2026-06-01T15:00:00Z", "2026-06-01T16:00:00Z")
	if _, err := e.q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: "meet-1",
		CategoryID: sql.NullInt64{Int64: meetings.ID, Valid: true},
		Kind:       "category",
	}); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(service.TabularTemplateSpec{
		Version: 1,
		Grain:   service.ExportGrainDetail,
		Layout:  service.ExportLayoutFlat,
		Columns: []service.TabularColumnSpec{
			{Field: service.ExportFieldTitle, Header: "What"},
			{Field: service.ExportFieldHours, Header: "Hrs"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	preview, err := e.svc.PreviewExport(ctx, service.PreviewExportInput{
		PeriodID: e.periodID,
		Format:   "csv",
		Body:     string(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Soft-demote: calendar-only period has no payable detail rows.
	if preview.Content != "What,Hrs" {
		t.Fatalf("content = %q", preview.Content)
	}
}

func TestExportTemplateCRUD_CustomText(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto")
	ctx := context.Background()

	cats, err := e.svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var deepWork service.Category
	for _, c := range cats {
		if c.Name == "Deep Work" {
			deepWork = c
		}
	}
	insertTimeEntry(t, e.q, e.periodID, "2026-06-01", "2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z", sql.NullInt64{Int64: deepWork.ID, Valid: true}, "", false)

	body := "Custom: {{.PeriodLabel}} — {{duration .ActualMinutes}}"
	created, err := e.svc.CreateExportTemplate(ctx, service.CreateExportTemplateInput{
		Name:        "Custom text",
		Description: "short summary",
		Format:      "text",
		Body:        body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Builtin {
		t.Fatal("custom text template should not be builtin")
	}
	if created.Format != "text" {
		t.Fatalf("format = %q want text", created.Format)
	}
	if created.Body != body {
		t.Fatalf("body = %q want %q", created.Body, body)
	}

	preview, err := e.svc.PreviewExport(ctx, service.PreviewExportInput{
		PeriodID:    e.periodID,
		TemplateKey: created.Key,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Custom: Jun 1-Jun 2 — 2h"
	if preview.Content != want {
		t.Fatalf("preview = %q want %q", preview.Content, want)
	}
	if preview.Format != "text" {
		t.Fatalf("preview format = %q", preview.Format)
	}

	render, err := e.svc.RenderPeriodExport(ctx, e.periodID, created.Key)
	if err != nil {
		t.Fatal(err)
	}
	if render.Content != want {
		t.Fatalf("render = %q want %q", render.Content, want)
	}

	draftPreview, err := e.svc.PreviewExport(ctx, service.PreviewExportInput{
		PeriodID: e.periodID,
		Format:   "text",
		Body:     "Draft {{duration .ActualMinutes}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draftPreview.Content != "Draft 2h" {
		t.Fatalf("draft preview = %q", draftPreview.Content)
	}

	if _, err := e.svc.CreateExportTemplate(ctx, service.CreateExportTemplateInput{
		Name:   "Broken text",
		Format: "text",
		Body:   "{{.PeriodLabel",
	}); err == nil {
		t.Fatal("expected invalid template create to fail")
	}

	if _, err := e.svc.PreviewExport(ctx, service.PreviewExportInput{
		PeriodID: e.periodID,
		Format:   "text",
		Body:     "{{.Nope",
	}); err == nil {
		t.Fatal("expected invalid draft preview to fail")
	}

	if _, err := e.svc.CreateExportTemplate(ctx, service.CreateExportTemplateInput{
		Name:   "Empty text",
		Format: "text",
		Body:   "   ",
	}); err == nil {
		t.Fatal("expected empty text body create to fail")
	}

	dup, err := e.svc.DuplicateExportTemplate(ctx, service.ExportTemplateTextSummary)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Builtin {
		t.Fatal("duplicate should be custom")
	}
	if dup.Format != "text" {
		t.Fatalf("dup format = %q", dup.Format)
	}
	if strings.TrimSpace(dup.Body) == "" {
		t.Fatal("duplicated text body should not be empty")
	}

	builtin, err := e.svc.GetExportTemplate(ctx, service.ExportTemplateTextSummary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateExportTemplate(ctx, service.UpdateExportTemplateInput{
		ID:     builtin.ID,
		Name:   "nope",
		Format: "text",
		Body:   body,
	}); !errors.Is(err, service.ErrExportTemplateBuiltin) {
		t.Fatalf("want ErrExportTemplateBuiltin on update, got %v", err)
	}
	if err := e.svc.DeleteExportTemplate(ctx, builtin.ID); !errors.Is(err, service.ErrExportTemplateBuiltin) {
		t.Fatalf("want ErrExportTemplateBuiltin on delete, got %v", err)
	}

	updated, err := e.svc.UpdateExportTemplate(ctx, service.UpdateExportTemplateInput{
		ID:          created.ID,
		Name:        "Custom text v2",
		Description: "updated",
		Format:      "text",
		Body:        "Updated {{.StartDate}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Custom text v2" {
		t.Fatalf("name = %q", updated.Name)
	}
	updatedRender, err := e.svc.RenderPeriodExport(ctx, e.periodID, created.Key)
	if err != nil {
		t.Fatal(err)
	}
	if updatedRender.Content != "Updated 2026-06-01" {
		t.Fatalf("updated render = %q", updatedRender.Content)
	}

	if err := e.svc.DeleteExportTemplate(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}
