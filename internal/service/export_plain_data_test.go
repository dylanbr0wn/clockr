package service_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestPlainPeriodExportData_PascalCaseTree(t *testing.T) {
	catID := int64(42)
	model := service.PeriodExportModel{
		PeriodID:      9,
		PeriodLabel:   "Jun 1-Jun 2",
		StartDate:     "2026-06-01",
		EndDate:       "2026-06-02",
		TargetMinutes: 960,
		ActualMinutes: 360,
		Days:          []string{"2026-06-01", "2026-06-02"},
		Entries: []service.ExportEntry{{
			Source:       "time_entry",
			SourceID:     11,
			Day:          "2026-06-01",
			StartMinutes: 540,
			EndMinutes:   660,
			Minutes:      120,
			Title:        "Focus",
			Description:  "deep",
			Category: service.ExportCategory{
				ID:    &catID,
				Name:  "Deep Work",
				Key:   "deep_work",
				Color: "#111",
			},
			WorkType:       "development",
			ProjectName:    "Shiet",
			ProjectKey:     "SHIET",
			BillableStatus: "billable",
		}},
		DailyTotals: []service.ExportDayTotals{{
			Date: "2026-06-01",
			Categories: []service.ExportCategoryMinutes{{
				Category: service.ExportCategory{Name: "Deep Work", Key: "deep_work"},
				Minutes:  120,
			}},
			ActualMinutes: 120,
			TargetMinutes: 480,
		}},
		PeriodTotals: []service.ExportCategoryMinutes{{
			Category: service.ExportCategory{ID: &catID, Name: "Deep Work", Key: "deep_work"},
			Minutes:  120,
		}},
	}

	data := service.PlainPeriodExportData(model)

	if data["PeriodID"] != int64(9) {
		t.Fatalf("PeriodID = %v", data["PeriodID"])
	}
	if data["PeriodLabel"] != "Jun 1-Jun 2" {
		t.Fatalf("PeriodLabel = %v", data["PeriodLabel"])
	}
	if data["StartDate"] != "2026-06-01" || data["EndDate"] != "2026-06-02" {
		t.Fatalf("dates = %v %v", data["StartDate"], data["EndDate"])
	}
	if data["TargetMinutes"] != 960 || data["ActualMinutes"] != 360 {
		t.Fatalf("minutes = %v %v", data["TargetMinutes"], data["ActualMinutes"])
	}
	if data["VarianceMinutes"] != -600 {
		t.Fatalf("VarianceMinutes = %v want -600", data["VarianceMinutes"])
	}

	days, ok := data["Days"].([]any)
	if !ok || len(days) != 2 || days[0] != "2026-06-01" {
		t.Fatalf("Days = %#v", data["Days"])
	}

	entries, ok := data["Entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("Entries = %#v", data["Entries"])
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("Entries[0] type = %T", entries[0])
	}
	if entry["Title"] != "Focus" || entry["SourceID"] != int64(11) {
		t.Fatalf("entry = %#v", entry)
	}
	cat, ok := entry["Category"].(map[string]any)
	if !ok || cat["Name"] != "Deep Work" || cat["ID"] != int64(42) {
		t.Fatalf("entry.Category = %#v", entry["Category"])
	}

	periodTotals, ok := data["PeriodTotals"].([]any)
	if !ok || len(periodTotals) != 1 {
		t.Fatalf("PeriodTotals = %#v", data["PeriodTotals"])
	}
	pt, ok := periodTotals[0].(map[string]any)
	if !ok {
		t.Fatalf("PeriodTotals[0] type = %T", periodTotals[0])
	}
	ptCat, ok := pt["Category"].(map[string]any)
	if !ok || ptCat["Key"] != "deep_work" || pt["Minutes"] != 120 {
		t.Fatalf("PeriodTotals[0] = %#v", pt)
	}

	daily, ok := data["DailyTotals"].([]any)
	if !ok || len(daily) != 1 {
		t.Fatalf("DailyTotals = %#v", data["DailyTotals"])
	}
	day, ok := daily[0].(map[string]any)
	if !ok || day["Date"] != "2026-06-01" {
		t.Fatalf("DailyTotals[0] = %#v", daily[0])
	}
}

func TestPlainPeriodExportData_PlainTypesOnly(t *testing.T) {
	catID := int64(1)
	model := service.PeriodExportModel{
		PeriodLabel:   "x",
		TargetMinutes: 60,
		ActualMinutes: 30,
		Days:          []string{"2026-06-01"},
		Entries: []service.ExportEntry{{
			Source:   "event",
			SourceID: 1,
			Day:      "2026-06-01",
			Minutes:  30,
			Title:    "m",
			Category: service.ExportCategory{ID: &catID, Name: "Meetings", Key: "meetings"},
		}},
		DailyTotals: []service.ExportDayTotals{{
			Date:          "2026-06-01",
			ActualMinutes: 30,
			TargetMinutes: 60,
			Categories: []service.ExportCategoryMinutes{{
				Category: service.ExportCategory{Name: "Meetings", Key: "meetings"},
				Minutes:  30,
			}},
		}},
		PeriodTotals: []service.ExportCategoryMinutes{{
			Category: service.ExportCategory{Name: "Meetings", Key: "meetings"},
			Minutes:  30,
		}},
	}

	assertPlainDataTree(t, service.PlainPeriodExportData(model), "root")
}

func TestPlainPeriodExportData_NilCategoryID(t *testing.T) {
	model := service.PeriodExportModel{
		PeriodTotals: []service.ExportCategoryMinutes{{
			Category: service.ExportCategory{Name: "Unassigned", Key: "Unassigned"},
			Minutes:  15,
		}},
	}
	data := service.PlainPeriodExportData(model)
	pt := data["PeriodTotals"].([]any)[0].(map[string]any)
	cat := pt["Category"].(map[string]any)
	if cat["ID"] != nil {
		t.Fatalf("nil Category.ID should be nil, got %#v", cat["ID"])
	}
}

func TestPlainPeriodExportData_TemplateFieldPaths(t *testing.T) {
	catID := int64(3)
	model := service.PeriodExportModel{
		PeriodLabel:   "Jun 1",
		StartDate:     "2026-06-01",
		EndDate:       "2026-06-01",
		TargetMinutes: 480,
		ActualMinutes: 120,
		PeriodTotals: []service.ExportCategoryMinutes{{
			Category: service.ExportCategory{ID: &catID, Name: "Meetings", Key: "meetings"},
			Minutes:  120,
		}},
		DailyTotals: []service.ExportDayTotals{{
			Date:          "2026-06-01",
			ActualMinutes: 120,
			TargetMinutes: 480,
			Categories: []service.ExportCategoryMinutes{{
				Category: service.ExportCategory{Name: "Meetings", Key: "meetings"},
				Minutes:  120,
			}},
		}},
	}

	tmpl, err := template.New("t").Funcs(template.FuncMap{
		"duration": func(m int) string { return fmt.Sprintf("%dm", m) },
		"signedDuration": func(m int) string {
			if m < 0 {
				return fmt.Sprintf("-%dm", -m)
			}
			return fmt.Sprintf("+%dm", m)
		},
	}).Parse(`{{.PeriodLabel}}|{{duration .ActualMinutes}}|{{signedDuration .VarianceMinutes}}|{{range .PeriodTotals}}{{.Category.Name}}:{{duration .Minutes}}{{end}}|{{range .DailyTotals}}{{.Date}}{{end}}`)
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, service.PlainPeriodExportData(model)); err != nil {
		t.Fatal(err)
	}
	want := "Jun 1|120m|-360m|Meetings:120m|2026-06-01"
	if buf.String() != want {
		t.Fatalf("got %q want %q", buf.String(), want)
	}
}

func assertPlainDataTree(t *testing.T, v any, path string) {
	t.Helper()
	if v == nil {
		return
	}
	switch x := v.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return
	case map[string]any:
		for k, child := range x {
			assertPlainDataTree(t, child, path+"."+k)
		}
	case []any:
		for i, child := range x {
			assertPlainDataTree(t, child, fmt.Sprintf("%s[%d]", path, i))
		}
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Struct, reflect.Ptr, reflect.Interface:
			t.Fatalf("%s: method-bearing/non-plain type %T", path, v)
		default:
			t.Fatalf("%s: unexpected type %T", path, v)
		}
	}
}
