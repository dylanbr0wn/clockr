package service

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"
)

// Builtin export template keys.
const (
	ExportTemplateMatrixCSV        = "matrix_csv"
	ExportTemplateFlatDailyCSV     = "flat_daily_csv"
	ExportTemplateDetailEntriesCSV = "detail_entries_csv"
	ExportTemplateTextSummary      = "text_summary"
)

// Fallback when the seeded text_summary body is empty.
const builtinTextSummaryTemplate = `Period: {{.PeriodLabel}}
{{.StartDate}} to {{.EndDate}}

Target: {{duration .TargetMinutes}}
Actual: {{duration .ActualMinutes}}
Variance: {{signedDuration .VarianceMinutes}}

Totals by category:
{{range .PeriodTotals}}  {{.Category.Name}}: {{duration .Minutes}}
{{end}}
Daily breakdown:
{{range .DailyTotals}}{{.Date}} — {{duration .ActualMinutes}} / {{duration .TargetMinutes}} target
{{if .Categories}}{{range .Categories}}  {{.Category.Name}}: {{duration .Minutes}}
{{end}}{{else}}  (no tracked time)
{{end}}
{{end}}`

const (
	scheduleStartMinutes = 0
	scheduleEndMinutes   = 24 * 60
	minBlockMinutes      = 15
)

const unassignedCategoryName = "Unassigned"

// ExportTemplate is a named export preset (builtin or user-defined).
type ExportTemplate struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"`
	Builtin     bool   `json:"builtin"`
	Body        string `json:"body"`
}

// ExportCategory is category identity on an export row or rollup.
type ExportCategory struct {
	ID    *int64 `json:"id,omitempty"`
	Name  string `json:"name"`
	Key   string `json:"key"`
	Color string `json:"color,omitempty"`
}

// ExportEntry is one timed interval contributing to the period export.
type ExportEntry struct {
	Source         string         `json:"source"` // event | time_entry
	SourceID       int64          `json:"sourceId"`
	Day            string         `json:"day"` // YYYY-MM-DD
	StartMinutes   int            `json:"startMinutes"`
	EndMinutes     int            `json:"endMinutes"`
	Minutes        int            `json:"minutes"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Category       ExportCategory `json:"category"`
	WorkType       string         `json:"workType,omitempty"`       // time_entry only; empty for calendar events
	ProjectName    string         `json:"projectName,omitempty"`    // time_entry only
	ProjectKey     string         `json:"projectKey,omitempty"`     // time_entry only
	BillableStatus string         `json:"billableStatus,omitempty"` // time_entry only
}

// ExportCategoryMinutes is minutes for one category within a day or period.
type ExportCategoryMinutes struct {
	Category ExportCategory `json:"category"`
	Minutes  int            `json:"minutes"`
}

// ExportDayTotals is per-day rollup across categories.
type ExportDayTotals struct {
	Date          string                  `json:"date"`
	Categories    []ExportCategoryMinutes `json:"categories"`
	ActualMinutes int                     `json:"actualMinutes"`
	TargetMinutes int                     `json:"targetMinutes"`
}

// PeriodExportModel is the intermediate period export aggregation.
type PeriodExportModel struct {
	PeriodID      int64                   `json:"periodId"`
	PeriodLabel   string                  `json:"periodLabel"`
	StartDate     string                  `json:"startDate"`
	EndDate       string                  `json:"endDate"`
	TargetMinutes int                     `json:"targetMinutes"`
	ActualMinutes int                     `json:"actualMinutes"`
	Days          []string                `json:"days"`
	Entries       []ExportEntry           `json:"entries"`
	DailyTotals   []ExportDayTotals       `json:"dailyTotals"`
	PeriodTotals  []ExportCategoryMinutes `json:"periodTotals"`
}

// PeriodExportRender is rendered export content ready to save.
type PeriodExportRender struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Format   string `json:"format"`
}

func exportTemplateFromRow(id int64, key, name, description, format string, builtin int64, body string) ExportTemplate {
	return ExportTemplate{
		ID:          id,
		Key:         key,
		Name:        name,
		Description: description,
		Format:      format,
		Builtin:     builtin != 0,
		Body:        body,
	}
}

// ListExportTemplates returns all export presets.
func (s *Service) ListExportTemplates(ctx context.Context) ([]ExportTemplate, error) {
	rows, err := s.q.ListExportTemplates(ctx)
	if err != nil {
		return nil, mapErr("list export templates", err)
	}
	out := make([]ExportTemplate, len(rows))
	for i, r := range rows {
		out[i] = exportTemplateFromRow(r.ID, r.Key, r.Name, r.Description, r.Format, r.Builtin, r.Body)
	}
	return out, nil
}

// GetExportTemplate returns one preset by key.
func (s *Service) GetExportTemplate(ctx context.Context, key string) (ExportTemplate, error) {
	r, err := s.q.GetExportTemplateByKey(ctx, key)
	if err != nil {
		return ExportTemplate{}, mapErr("get export template", err)
	}
	return exportTemplateFromRow(r.ID, r.Key, r.Name, r.Description, r.Format, r.Builtin, r.Body), nil
}

// BuildPeriodExport aggregates live period data into the export intermediate model.
func (s *Service) BuildPeriodExport(ctx context.Context, periodID int64) (PeriodExportModel, error) {
	period, err := s.GetPeriod(ctx, periodID)
	if err != nil {
		return PeriodExportModel{}, err
	}
	segs, err := s.ListTzSegments(ctx, periodID)
	if err != nil {
		return PeriodExportModel{}, err
	}
	events, err := s.ListEvents(ctx, periodID)
	if err != nil {
		return PeriodExportModel{}, err
	}
	timeEntries, err := s.ListTimeEntries(ctx, periodID)
	if err != nil {
		return PeriodExportModel{}, err
	}
	overlays, err := s.ListEventCategoryOverlays(ctx, periodID)
	if err != nil {
		return PeriodExportModel{}, err
	}
	categories, err := s.ListAllCategories(ctx)
	if err != nil {
		return PeriodExportModel{}, err
	}
	projects, err := s.ListAllProjects(ctx)
	if err != nil {
		return PeriodExportModel{}, err
	}

	catsByID := make(map[int64]Category, len(categories))
	for _, c := range categories {
		catsByID[c.ID] = c
	}
	projectsByID := make(map[int64]Project, len(projects))
	for _, p := range projects {
		projectsByID[p.ID] = p
	}
	overlayByKey := make(map[string]int64, len(overlays))
	for _, o := range overlays {
		overlayByKey[eventOverlayKey(o.Provider, o.ExternalID, o.InstanceID)] = o.CategoryID
	}

	locCache := map[string]*time.Location{}
	entries := make([]ExportEntry, 0, len(events)+len(timeEntries))

	for _, event := range events {
		entry, ok, err := eventToExportEntry(event, segs, catsByID, overlayByKey, locCache)
		if err != nil {
			return PeriodExportModel{}, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	for _, te := range timeEntries {
		entry, ok, err := timeEntryToExportEntry(te, segs, catsByID, projectsByID, locCache)
		if err != nil {
			return PeriodExportModel{}, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}

	days, err := periodDateRange(period.StartDate, period.EndDate)
	if err != nil {
		return PeriodExportModel{}, err
	}
	expected, err := s.ExpectedTimeForRange(ctx, period.StartDate, period.EndDate)
	if err != nil {
		return PeriodExportModel{}, err
	}
	expectedByDate := make(map[string]int, len(expected))
	targetMinutes := 0
	for _, et := range expected {
		expectedByDate[et.Date] = et.ExpectedMinutes
		targetMinutes += et.ExpectedMinutes
	}

	periodTotalsMap := map[string]ExportCategoryMinutes{}
	dailyMaps := make([]map[string]ExportCategoryMinutes, len(days))
	dayIndex := make(map[string]int, len(days))
	for i, day := range days {
		dayIndex[day] = i
		dailyMaps[i] = map[string]ExportCategoryMinutes{}
	}

	actualMinutes := 0
	for _, entry := range entries {
		actualMinutes += entry.Minutes
		identity := categoryIdentity(entry.Category)
		addCategoryMinutes(periodTotalsMap, identity, entry.Category, entry.Minutes)
		if idx, ok := dayIndex[entry.Day]; ok {
			addCategoryMinutes(dailyMaps[idx], identity, entry.Category, entry.Minutes)
		}
	}

	dailyTotals := make([]ExportDayTotals, len(days))
	for i, day := range days {
		cats := sortedCategoryMinutes(dailyMaps[i])
		dayActual := 0
		for _, c := range cats {
			dayActual += c.Minutes
		}
		dailyTotals[i] = ExportDayTotals{
			Date:          day,
			Categories:    cats,
			ActualMinutes: dayActual,
			TargetMinutes: expectedByDate[day],
		}
	}

	return PeriodExportModel{
		PeriodID:      period.ID,
		PeriodLabel:   formatPeriodLabel(period.StartDate, period.EndDate),
		StartDate:     period.StartDate,
		EndDate:       period.EndDate,
		TargetMinutes: targetMinutes,
		ActualMinutes: actualMinutes,
		Days:          days,
		Entries:       entries,
		DailyTotals:   dailyTotals,
		PeriodTotals:  sortedCategoryMinutes(periodTotalsMap),
	}, nil
}

// RenderPeriodExport builds the model and renders it with the named template.
func (s *Service) RenderPeriodExport(ctx context.Context, periodID int64, templateKey string) (PeriodExportRender, error) {
	if templateKey == "" {
		templateKey = ExportTemplateMatrixCSV
	}
	tmpl, err := s.GetExportTemplate(ctx, templateKey)
	if err != nil {
		return PeriodExportRender{}, err
	}
	model, err := s.BuildPeriodExport(ctx, periodID)
	if err != nil {
		return PeriodExportRender{}, err
	}
	return renderExportTemplate(model, tmpl)
}

func renderTextSummary(model PeriodExportModel, body, templateKey string) (string, error) {
	if strings.TrimSpace(body) == "" {
		if templateKey == ExportTemplateTextSummary {
			body = builtinTextSummaryTemplate
		} else {
			return "", fmt.Errorf("text template body is required")
		}
	}
	tmpl, err := template.New("text_summary").Funcs(exportTemplateFuncs()).Parse(body)
	if err != nil {
		return "", fmt.Errorf("parse text summary template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, PlainPeriodExportData(model)); err != nil {
		return "", fmt.Errorf("execute text summary template: %w", err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func exportTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"duration":       formatExportDuration,
		"signedDuration": formatSignedExportDuration,
	}
}

func formatExportDuration(totalMinutes int) string {
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func formatSignedExportDuration(minutes int) string {
	sign := "+"
	if minutes < 0 {
		sign = "-"
	}
	return sign + formatExportDuration(absInt(minutes))
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func eventOverlayKey(provider, externalID, instanceID string) string {
	return provider + "|" + externalID + "|" + instanceID
}

func resolveExportCategory(categoryID *int64, catsByID map[int64]Category) ExportCategory {
	if categoryID == nil {
		return ExportCategory{Name: unassignedCategoryName, Key: unassignedCategoryName}
	}
	cat, ok := catsByID[*categoryID]
	if !ok {
		return ExportCategory{Name: unassignedCategoryName, Key: unassignedCategoryName}
	}
	id := cat.ID
	return ExportCategory{
		ID:    &id,
		Name:  cat.Name,
		Key:   cat.Key,
		Color: cat.Color,
	}
}

func eventToExportEntry(
	event Event,
	segs []TzSegment,
	catsByID map[int64]Category,
	overlayByKey map[string]int64,
	locCache map[string]*time.Location,
) (ExportEntry, bool, error) {
	var categoryID *int64
	if id, ok := overlayByKey[eventOverlayKey(event.Provider, event.ExternalID, event.InstanceID)]; ok {
		categoryID = &id
	}
	category := resolveExportCategory(categoryID, catsByID)
	title := event.Title
	if title == "" {
		title = "Untitled event"
	}

	if event.AllDay {
		if event.StartDate == "" {
			return ExportEntry{}, false, nil
		}
		startMinutes := scheduleStartMinutes
		endMinutes := scheduleEndMinutes
		return ExportEntry{
			Source:       "event",
			SourceID:     event.ID,
			Day:          event.StartDate,
			StartMinutes: startMinutes,
			EndMinutes:   endMinutes,
			Minutes:      endMinutes - startMinutes,
			Title:        title,
			Description:  event.Description,
			Category:     category,
		}, true, nil
	}

	if event.Start == nil || event.End == nil {
		return ExportEntry{}, false, nil
	}
	startDay, startMinutes, err := zonedPosition(*event.Start, segs, locCache)
	if err != nil {
		return ExportEntry{}, false, err
	}
	endDay, endMinutes, err := zonedPosition(*event.End, segs, locCache)
	if err != nil {
		return ExportEntry{}, false, err
	}
	if endDay != startDay {
		endMinutes = scheduleEndMinutes
	}
	endMinutes = maxInt(startMinutes+minBlockMinutes, endMinutes)
	return ExportEntry{
		Source:       "event",
		SourceID:     event.ID,
		Day:          startDay,
		StartMinutes: startMinutes,
		EndMinutes:   endMinutes,
		Minutes:      endMinutes - startMinutes,
		Title:        title,
		Description:  event.Description,
		Category:     category,
	}, true, nil
}

func timeEntryToExportEntry(
	entry TimeEntry,
	segs []TzSegment,
	catsByID map[int64]Category,
	projectsByID map[int64]Project,
	locCache map[string]*time.Location,
) (ExportEntry, bool, error) {
	start := parseTime(entry.Start)
	end := parseTime(entry.End)
	if start.IsZero() || end.IsZero() {
		return ExportEntry{}, false, nil
	}

	day := entry.LocalWorkDate
	tzName := "UTC"
	if len(segs) > 0 {
		if day == "" {
			// Resolve day from start using first segment, then re-resolve active TZ.
			initialLoc, err := loadLoc(locCache, segs[0].IanaTz)
			if err != nil {
				return ExportEntry{}, false, err
			}
			day, _ = zonedDateTimeParts(start, initialLoc)
		}
		tzName = activeSegment(segs, day).IanaTz
	}
	loc, err := loadLoc(locCache, tzName)
	if err != nil {
		return ExportEntry{}, false, err
	}
	startPartsDay, startMinutes := zonedDateTimeParts(start, loc)
	endPartsDay, endMinutes := zonedDateTimeParts(end, loc)
	if day == "" {
		day = startPartsDay
	}
	if endPartsDay != startPartsDay {
		endMinutes = scheduleEndMinutes
	}
	endMinutes = maxInt(startMinutes+minBlockMinutes, endMinutes)
	category := resolveExportCategory(entry.CategoryID, catsByID)
	title := entry.Description
	if title == "" {
		title = category.Name
	}
	projectName, projectKey := resolveExportProject(entry.ProjectID, projectsByID)
	return ExportEntry{
		Source:         "time_entry",
		SourceID:       entry.ID,
		Day:            day,
		StartMinutes:   startMinutes,
		EndMinutes:     endMinutes,
		Minutes:        endMinutes - startMinutes,
		Title:          title,
		Description:    entry.Description,
		Category:       category,
		WorkType:       entry.WorkType,
		ProjectName:    projectName,
		ProjectKey:     projectKey,
		BillableStatus: entry.BillableStatus,
	}, true, nil
}

func resolveExportProject(projectID *int64, projectsByID map[int64]Project) (name, key string) {
	if projectID == nil {
		return "", ""
	}
	project, ok := projectsByID[*projectID]
	if !ok {
		return "", ""
	}
	return project.Name, project.Key
}

func zonedPosition(t time.Time, segs []TzSegment, locCache map[string]*time.Location) (day string, minutes int, err error) {
	if len(segs) == 0 {
		day, minutes = zonedDateTimeParts(t.UTC(), time.UTC)
		return day, minutes, nil
	}
	initialLoc, err := loadLoc(locCache, segs[0].IanaTz)
	if err != nil {
		return "", 0, err
	}
	day, minutes = zonedDateTimeParts(t, initialLoc)
	active := activeSegment(segs, day)
	if active.IanaTz == segs[0].IanaTz {
		return day, minutes, nil
	}
	activeLoc, err := loadLoc(locCache, active.IanaTz)
	if err != nil {
		return "", 0, err
	}
	day, minutes = zonedDateTimeParts(t, activeLoc)
	return day, minutes, nil
}

func zonedDateTimeParts(t time.Time, loc *time.Location) (day string, minutes int) {
	local := t.In(loc)
	return local.Format("2006-01-02"), local.Hour()*60 + local.Minute()
}

func periodDateRange(startDate, endDate string) ([]string, error) {
	start, err := parseDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("period start_date: %w", err)
	}
	end, err := parseDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("period end_date: %w", err)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("period end_date before start_date")
	}
	out := make([]string, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}

func categoryIdentity(cat ExportCategory) string {
	if cat.Key != "" {
		return cat.Key
	}
	return cat.Name
}

func addCategoryMinutes(dst map[string]ExportCategoryMinutes, identity string, cat ExportCategory, minutes int) {
	cur, ok := dst[identity]
	if !ok {
		dst[identity] = ExportCategoryMinutes{Category: cat, Minutes: minutes}
		return
	}
	cur.Minutes += minutes
	dst[identity] = cur
}

func sortedCategoryMinutes(src map[string]ExportCategoryMinutes) []ExportCategoryMinutes {
	out := make([]ExportCategoryMinutes, 0, len(src))
	for _, v := range src {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Category.Name < out[j].Category.Name
	})
	return out
}

func formatPeriodLabel(startDate, endDate string) string {
	return formatShortDate(startDate) + "-" + formatShortDate(endDate)
}

func formatShortDate(dateStr string) string {
	t, err := parseDate(dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 2")
}

func defaultExportFilename(model PeriodExportModel) string {
	return fmt.Sprintf("shiet-%s-to-%s.csv", model.StartDate, model.EndDate)
}

func defaultExportTextFilename(model PeriodExportModel) string {
	return fmt.Sprintf("shiet-%s-to-%s.txt", model.StartDate, model.EndDate)
}

func renderMatrixCSV(model PeriodExportModel) string {
	return renderTabular(model, DefaultTabularSpec(ExportGrainRollup, ExportLayoutMatrix))
}

func renderFlatDailyCSV(model PeriodExportModel) string {
	return renderTabular(model, DefaultTabularSpec(ExportGrainRollup, ExportLayoutFlat))
}

func renderDetailEntriesCSV(model PeriodExportModel) string {
	return renderTabular(model, DefaultTabularSpec(ExportGrainDetail, ExportLayoutFlat))
}

func exportEntryDateTime(day string, minutes int) string {
	h := minutes / 60
	m := minutes % 60
	return fmt.Sprintf("%sT%02d:%02d", day, h, m)
}

func minutesToDecimalHours(minutes int) string {
	return fmt.Sprintf("%.2f", float64(minutes)/60.0)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
