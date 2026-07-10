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
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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

	// 11:00–13:00 EDT = 15:00–17:00Z → 2h on 2026-06-01
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

	// Gap fill 13:00–15:00 EDT = 17:00–19:00Z → 2h Deep Work on day 1
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T17:00:00Z",
		EndUtc:     "2026-06-01T19:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Note:       "Focus",
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

	// Gap fill day 2: 10:00–12:00 EDT = 14:00–16:00Z → 2h Deep Work
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T14:00:00Z",
		EndUtc:     "2026-06-02T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Note:       "Planning",
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
	if model.ActualMinutes != 360 {
		t.Fatalf("actualMinutes = %d want 360", model.ActualMinutes)
	}
	if model.TargetMinutes != 8*60*2 {
		t.Fatalf("targetMinutes = %d want %d", model.TargetMinutes, 8*60*2)
	}
	if len(model.Entries) != 3 {
		t.Fatalf("entries = %d want 3", len(model.Entries))
	}

	byName := map[string]service.ExportCategoryMinutes{}
	for _, total := range model.PeriodTotals {
		byName[total.Category.Name] = total
		if total.Category.Key == "" {
			t.Fatalf("category %q missing key", total.Category.Name)
		}
		if total.Category.Name == "Meetings" && total.Category.Key != meetings.Key {
			t.Fatalf("Meetings key = %q want %q", total.Category.Key, meetings.Key)
		}
	}
	if byName["Meetings"].Minutes != 120 {
		t.Fatalf("Meetings total = %d want 120", byName["Meetings"].Minutes)
	}
	if byName["Deep Work"].Minutes != 240 {
		t.Fatalf("Deep Work total = %d want 240", byName["Deep Work"].Minutes)
	}

	if len(model.DailyTotals) != 2 {
		t.Fatalf("dailyTotals len = %d", len(model.DailyTotals))
	}
	day1 := categoryMinutesByName(model.DailyTotals[0])
	if day1["Meetings"] != 120 || day1["Deep Work"] != 120 {
		t.Fatalf("day1 categories = %+v", day1)
	}
	day2 := categoryMinutesByName(model.DailyTotals[1])
	if day2["Deep Work"] != 120 {
		t.Fatalf("day2 categories = %+v", day2)
	}
}

func TestRenderPeriodExport_MatrixCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T17:00:00Z",
		EndUtc:     "2026-06-01T19:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T14:00:00Z",
		EndUtc:     "2026-06-02T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
		"Meetings,2.00,0.00,2.00",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_FlatDailyCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T17:00:00Z",
		EndUtc:     "2026-06-01T19:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T14:00:00Z",
		EndUtc:     "2026-06-02T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
		"2026-06-01,Meetings," + meetings.Key + ",2.00",
		"2026-06-02,Deep Work," + deepWork.Key + ",2.00",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_DetailEntriesCSVShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:    e.periodID,
		Day:         "2026-06-01",
		StartUtc:    "2026-06-01T17:00:00Z",
		EndUtc:      "2026-06-01T19:00:00Z",
		CategoryID:  sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Note:        "Focus",
		Description: "User notes",
		Source:      "manual",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T14:00:00Z",
		EndUtc:     "2026-06-02T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Note:       "Planning",
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
		"Start,End,Category,Key,Hours,Title,Description",
		"2026-06-01T11:00,2026-06-01T13:00,Meetings," + meetings.Key + ",2.00,meet-1,Google notes",
		"2026-06-01T13:00,2026-06-01T15:00,Deep Work," + deepWork.Key + ",2.00,Focus,User notes",
		"2026-06-02T10:00,2026-06-02T12:00,Deep Work," + deepWork.Key + ",2.00,Planning,",
	}, "\n")
	if render.Content != want {
		t.Fatalf("csv mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestBuildPeriodExport_EntryDescriptions(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:    e.periodID,
		Day:         "2026-06-01",
		StartUtc:    "2026-06-01T17:00:00Z",
		EndUtc:      "2026-06-01T19:00:00Z",
		CategoryID:  sql.NullInt64{Int64: meetings.ID, Valid: true},
		Note:        "Short title",
		Description: "Longer work notes",
		Source:      "manual",
	}); err != nil {
		t.Fatal(err)
	}

	model, err := e.svc.BuildPeriodExport(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Entries) != 2 {
		t.Fatalf("entries = %d want 2", len(model.Entries))
	}

	var eventEntry, gapEntry *service.ExportEntry
	for i := range model.Entries {
		switch model.Entries[i].Source {
		case "event":
			eventEntry = &model.Entries[i]
		case "gap_fill":
			gapEntry = &model.Entries[i]
		}
	}
	if eventEntry == nil || gapEntry == nil {
		t.Fatalf("entries = %+v", model.Entries)
	}
	if eventEntry.Description != "Calendar agenda" {
		t.Fatalf("event description = %q want Calendar agenda", eventEntry.Description)
	}
	if eventEntry.Title != "meet-1" {
		t.Fatalf("event title = %q want meet-1", eventEntry.Title)
	}
	if gapEntry.Description != "Longer work notes" {
		t.Fatalf("gap description = %q want Longer work notes", gapEntry.Description)
	}
	if gapEntry.Title != "Short title" {
		t.Fatalf("gap title = %q want Short title", gapEntry.Title)
	}
}

func TestRenderPeriodExport_TextSummaryShape(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T17:00:00Z",
		EndUtc:     "2026-06-01T19:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T14:00:00Z",
		EndUtc:     "2026-06-02T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
		"Target: 16h (8h/day)",
		"Actual: 6h",
		"Variance: -10h",
		"",
		"Totals by category:",
		"  Deep Work: 4h",
		"  Meetings: 2h",
		"",
		"Daily breakdown:",
		"2026-06-01 — 4h / 8h target",
		"  Deep Work: 2h",
		"  Meetings: 2h",
		"",
		"2026-06-02 — 2h / 8h target",
		"  Deep Work: 2h",
	}, "\n")
	if render.Content != want {
		t.Fatalf("text mismatch\ngot:\n%s\nwant:\n%s", render.Content, want)
	}
}

func TestRenderPeriodExport_TextSummaryEmptyDay(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T14:00:00Z",
		EndUtc:     "2026-06-01T15:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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

func TestBuildPeriodExport_UnassignedWithoutOverlay(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	e.addEvent(t, "bare", "2026-06-01T15:00:00Z", "2026-06-01T16:00:00Z")

	model, err := e.svc.BuildPeriodExport(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.PeriodTotals) != 1 {
		t.Fatalf("periodTotals = %+v", model.PeriodTotals)
	}
	total := model.PeriodTotals[0]
	if total.Category.Name != "Unassigned" || total.Category.Key != "Unassigned" {
		t.Fatalf("category = %+v want Unassigned", total.Category)
	}
	if total.Minutes != 60 {
		t.Fatalf("minutes = %d want 60", total.Minutes)
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
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T14:00:00Z",
		EndUtc:     "2026-06-01T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
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
	if preview.Content != "What,Hrs\nmeet-1,1.00" {
		t.Fatalf("content = %q", preview.Content)
	}
}

func TestExportTemplateCRUD_CustomText(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-02", "America/Toronto", 8)
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
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-01",
		StartUtc:   "2026-06-01T14:00:00Z",
		EndUtc:     "2026-06-01T16:00:00Z",
		CategoryID: sql.NullInt64{Int64: deepWork.ID, Valid: true},
		Source:     "manual",
	}); err != nil {
		t.Fatal(err)
	}

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
