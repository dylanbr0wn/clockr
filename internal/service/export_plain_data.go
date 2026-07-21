package service

// PlainPeriodExportData flattens a period export model into nested PascalCase
// maps/slices/scalars for text/template execution. No structs or method-bearing
// types — SSTI hardening for custom text export templates (DYL-182).
//
// Keys mirror PeriodExportModel exported field names (plus root VarianceMinutes)
// so existing template field paths keep working without migration.
func PlainPeriodExportData(model PeriodExportModel) map[string]any {
	return map[string]any{
		"PeriodID":        model.PeriodID,
		"PeriodLabel":     model.PeriodLabel,
		"StartDate":       model.StartDate,
		"EndDate":         model.EndDate,
		"TargetMinutes":   model.TargetMinutes,
		"ActualMinutes":   model.ActualMinutes,
		"VarianceMinutes": model.ActualMinutes - model.TargetMinutes,
		"Days":            plainStringSlice(model.Days),
		"Entries":         plainExportEntries(model.Entries),
		"DailyTotals":     plainExportDayTotals(model.DailyTotals),
		"PeriodTotals":    plainExportCategoryMinutes(model.PeriodTotals),
	}
}

func plainStringSlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func plainExportEntries(in []ExportEntry) []any {
	out := make([]any, len(in))
	for i, e := range in {
		out[i] = map[string]any{
			"Source":         e.Source,
			"SourceID":       e.SourceID,
			"Day":            e.Day,
			"StartMinutes":   e.StartMinutes,
			"EndMinutes":     e.EndMinutes,
			"Minutes":        e.Minutes,
			"Title":          e.Title,
			"Description":    e.Description,
			"Category":       plainExportCategory(e.Category),
			"WorkType":       e.WorkType,
			"ProjectName":    e.ProjectName,
			"ProjectKey":     e.ProjectKey,
			"BillableStatus": e.BillableStatus,
		}
	}
	return out
}

func plainExportDayTotals(in []ExportDayTotals) []any {
	out := make([]any, len(in))
	for i, d := range in {
		out[i] = map[string]any{
			"Date":          d.Date,
			"Categories":    plainExportCategoryMinutes(d.Categories),
			"ActualMinutes": d.ActualMinutes,
			"TargetMinutes": d.TargetMinutes,
		}
	}
	return out
}

func plainExportCategoryMinutes(in []ExportCategoryMinutes) []any {
	out := make([]any, len(in))
	for i, c := range in {
		out[i] = map[string]any{
			"Category": plainExportCategory(c.Category),
			"Minutes":  c.Minutes,
		}
	}
	return out
}

func plainExportCategory(c ExportCategory) map[string]any {
	var id any
	if c.ID != nil {
		id = *c.ID
	}
	return map[string]any{
		"ID":    id,
		"Name":  c.Name,
		"Key":   c.Key,
		"Color": c.Color,
	}
}
