package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Tabular grain / layout values stored in export_template.body JSON.
const (
	ExportGrainRollup = "rollup"
	ExportGrainDetail = "detail"

	ExportLayoutMatrix = "matrix"
	ExportLayoutFlat   = "flat"
)

// Tabular field identifiers (layout-only; category identity is name/key).
const (
	ExportFieldDate         = "date"
	ExportFieldCategoryName = "category_name"
	ExportFieldCategoryKey  = "category_key"
	ExportFieldHours        = "hours"
	ExportFieldMinutes      = "minutes"
	ExportFieldStart        = "start"
	ExportFieldEnd          = "end"
	ExportFieldTitle        = "title"
	ExportFieldDescription  = "description"
	ExportFieldSource       = "source"
	ExportFieldDayActual    = "day_actual_hours"
	ExportFieldDayTarget    = "day_target_hours"
	ExportFieldTotal        = "total"
)

// TabularColumnSpec is one selected column in a declarative tabular template.
type TabularColumnSpec struct {
	Field  string `json:"field"`
	Header string `json:"header"`
}

// TabularTemplateSpec is the declarative body for CSV/TSV export templates.
type TabularTemplateSpec struct {
	Version   int                 `json:"version"`
	Grain     string              `json:"grain"`
	Layout    string              `json:"layout"`
	Delimiter string              `json:"delimiter"`
	Columns   []TabularColumnSpec `json:"columns"`
}

// ExportFieldInfo describes a catalog field available for a grain/layout.
type ExportFieldInfo struct {
	Field       string `json:"field"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// DefaultTabularSpec returns a sensible starter spec for the form builder.
func DefaultTabularSpec(grain, layout string) TabularTemplateSpec {
	grain = strings.TrimSpace(grain)
	layout = strings.TrimSpace(layout)
	if grain == "" {
		grain = ExportGrainRollup
	}
	if layout == "" {
		layout = ExportLayoutFlat
	}
	if grain == ExportGrainDetail {
		layout = ExportLayoutFlat
	}
	spec := TabularTemplateSpec{
		Version:   1,
		Grain:     grain,
		Layout:    layout,
		Delimiter: ",",
	}
	switch {
	case grain == ExportGrainDetail:
		spec.Columns = []TabularColumnSpec{
			{Field: ExportFieldStart, Header: "Start"},
			{Field: ExportFieldEnd, Header: "End"},
			{Field: ExportFieldCategoryName, Header: "Category"},
			{Field: ExportFieldCategoryKey, Header: "Key"},
			{Field: ExportFieldHours, Header: "Hours"},
			{Field: ExportFieldTitle, Header: "Title"},
			{Field: ExportFieldDescription, Header: "Description"},
		}
	case layout == ExportLayoutMatrix:
		spec.Columns = []TabularColumnSpec{
			{Field: ExportFieldCategoryName, Header: "Category"},
			{Field: ExportFieldTotal, Header: "Total"},
		}
	default:
		spec.Columns = []TabularColumnSpec{
			{Field: ExportFieldDate, Header: "Date"},
			{Field: ExportFieldCategoryName, Header: "Category"},
			{Field: ExportFieldCategoryKey, Header: "Key"},
			{Field: ExportFieldHours, Header: "Hours"},
		}
	}
	return spec
}

// ListExportFieldCatalog returns fields valid for the given grain/layout.
func ListExportFieldCatalog(grain, layout string) ([]ExportFieldInfo, error) {
	grain = strings.TrimSpace(grain)
	layout = strings.TrimSpace(layout)
	if grain == "" {
		grain = ExportGrainRollup
	}
	if layout == "" {
		layout = ExportLayoutFlat
	}
	if err := validateGrainLayout(grain, layout); err != nil {
		return nil, invalidInputf("list export field catalog: %v", err)
	}
	return fieldCatalog(grain, layout), nil
}

func fieldCatalog(grain, layout string) []ExportFieldInfo {
	switch {
	case grain == ExportGrainDetail:
		return []ExportFieldInfo{
			{Field: ExportFieldDate, Label: "Date", Description: "Entry day (YYYY-MM-DD)"},
			{Field: ExportFieldStart, Label: "Start", Description: "Start datetime"},
			{Field: ExportFieldEnd, Label: "End", Description: "End datetime"},
			{Field: ExportFieldCategoryName, Label: "Category name", Description: "Category display name"},
			{Field: ExportFieldCategoryKey, Label: "Category key", Description: "Category key (falls back to name)"},
			{Field: ExportFieldHours, Label: "Hours", Description: "Duration as decimal hours"},
			{Field: ExportFieldMinutes, Label: "Minutes", Description: "Duration as whole minutes"},
			{Field: ExportFieldTitle, Label: "Title", Description: "Event title or gap-fill note"},
			{Field: ExportFieldDescription, Label: "Description", Description: "Event or gap-fill work notes"},
			{Field: ExportFieldSource, Label: "Source", Description: "event or gap_fill"},
		}
	case layout == ExportLayoutMatrix:
		return []ExportFieldInfo{
			{Field: ExportFieldCategoryName, Label: "Category name", Description: "Row label (category name)"},
			{Field: ExportFieldCategoryKey, Label: "Category key", Description: "Row label (category key)"},
			{Field: ExportFieldTotal, Label: "Total", Description: "Row total across days"},
		}
	default:
		return []ExportFieldInfo{
			{Field: ExportFieldDate, Label: "Date", Description: "Day (YYYY-MM-DD)"},
			{Field: ExportFieldCategoryName, Label: "Category name", Description: "Category display name"},
			{Field: ExportFieldCategoryKey, Label: "Category key", Description: "Category key (falls back to name)"},
			{Field: ExportFieldHours, Label: "Hours", Description: "Category minutes as decimal hours"},
			{Field: ExportFieldMinutes, Label: "Minutes", Description: "Category minutes"},
			{Field: ExportFieldDayActual, Label: "Day actual hours", Description: "Total tracked hours that day"},
			{Field: ExportFieldDayTarget, Label: "Day target hours", Description: "Target hours that day"},
		}
	}
}

func validateGrainLayout(grain, layout string) error {
	switch grain {
	case ExportGrainRollup, ExportGrainDetail:
	default:
		return fmt.Errorf("invalid grain %q (want rollup or detail)", grain)
	}
	switch layout {
	case ExportLayoutMatrix, ExportLayoutFlat:
	default:
		return fmt.Errorf("invalid layout %q (want matrix or flat)", layout)
	}
	if grain == ExportGrainDetail && layout != ExportLayoutFlat {
		return fmt.Errorf("detail grain only supports flat layout")
	}
	return nil
}

func parseTabularSpec(body string) (TabularTemplateSpec, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return TabularTemplateSpec{}, fmt.Errorf("tabular template body is empty")
	}
	var spec TabularTemplateSpec
	if err := json.Unmarshal([]byte(body), &spec); err != nil {
		return TabularTemplateSpec{}, fmt.Errorf("parse tabular template body: %w", err)
	}
	if err := validateTabularSpec(spec); err != nil {
		return TabularTemplateSpec{}, err
	}
	return normalizeTabularSpec(spec), nil
}

func validateTabularSpec(spec TabularTemplateSpec) error {
	if spec.Version != 0 && spec.Version != 1 {
		return fmt.Errorf("unsupported tabular template version %d", spec.Version)
	}
	if err := validateGrainLayout(spec.Grain, spec.Layout); err != nil {
		return err
	}
	if len(spec.Columns) == 0 {
		return fmt.Errorf("tabular template requires at least one column")
	}
	allowed := map[string]struct{}{}
	for _, f := range fieldCatalog(spec.Grain, spec.Layout) {
		allowed[f.Field] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, col := range spec.Columns {
		field := strings.TrimSpace(col.Field)
		if field == "" {
			return fmt.Errorf("column field is required")
		}
		if _, ok := allowed[field]; !ok {
			return fmt.Errorf("field %q is not valid for grain=%s layout=%s", field, spec.Grain, spec.Layout)
		}
		if _, dup := seen[field]; dup {
			return fmt.Errorf("duplicate column field %q", field)
		}
		seen[field] = struct{}{}
	}
	if spec.Layout == ExportLayoutMatrix {
		hasRowLabel := false
		for _, col := range spec.Columns {
			switch col.Field {
			case ExportFieldCategoryName, ExportFieldCategoryKey:
				hasRowLabel = true
			}
		}
		if !hasRowLabel {
			return fmt.Errorf("matrix layout requires category_name or category_key")
		}
	}
	switch spec.Delimiter {
	case "", ",", "\t":
	default:
		return fmt.Errorf("invalid delimiter %q (want comma or tab)", spec.Delimiter)
	}
	return nil
}

func normalizeTabularSpec(spec TabularTemplateSpec) TabularTemplateSpec {
	if spec.Version == 0 {
		spec.Version = 1
	}
	if spec.Delimiter == "" {
		spec.Delimiter = ","
	}
	out := make([]TabularColumnSpec, len(spec.Columns))
	for i, col := range spec.Columns {
		header := strings.TrimSpace(col.Header)
		if header == "" {
			header = defaultFieldHeader(col.Field)
		}
		out[i] = TabularColumnSpec{Field: strings.TrimSpace(col.Field), Header: header}
	}
	spec.Columns = out
	return spec
}

func defaultFieldHeader(field string) string {
	switch field {
	case ExportFieldDate:
		return "Date"
	case ExportFieldCategoryName:
		return "Category"
	case ExportFieldCategoryKey:
		return "Key"
	case ExportFieldHours:
		return "Hours"
	case ExportFieldMinutes:
		return "Minutes"
	case ExportFieldStart:
		return "Start"
	case ExportFieldEnd:
		return "End"
	case ExportFieldTitle:
		return "Title"
	case ExportFieldSource:
		return "Source"
	case ExportFieldDayActual:
		return "Day actual"
	case ExportFieldDayTarget:
		return "Day target"
	case ExportFieldTotal:
		return "Total"
	default:
		return field
	}
}

func encodeTabularSpec(spec TabularTemplateSpec) (string, error) {
	spec = normalizeTabularSpec(spec)
	if err := validateTabularSpec(spec); err != nil {
		return "", err
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("encode tabular template body: %w", err)
	}
	return string(b), nil
}

func formatFromDelimiter(delimiter string) string {
	if delimiter == "\t" {
		return "tsv"
	}
	return "csv"
}

func delimiterFromFormat(format string) string {
	if format == "tsv" {
		return "\t"
	}
	return ","
}

func renderTabular(model PeriodExportModel, spec TabularTemplateSpec) string {
	spec = normalizeTabularSpec(spec)
	switch {
	case spec.Grain == ExportGrainDetail:
		return renderDetailTabular(model, spec)
	case spec.Layout == ExportLayoutMatrix:
		return renderMatrixTabular(model, spec)
	default:
		return renderFlatRollupTabular(model, spec)
	}
}

func joinDelimited(cells []string, delimiter string) string {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = escapeDelimitedCell(cell, delimiter)
	}
	return strings.Join(escaped, delimiter)
}

func escapeDelimitedCell(value, delimiter string) string {
	needsQuotes := strings.Contains(value, "\"") ||
		strings.Contains(value, "\n") ||
		strings.Contains(value, delimiter)
	if !needsQuotes {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func renderMatrixTabular(model PeriodExportModel, spec TabularTemplateSpec) string {
	delimiter := spec.Delimiter
	leading := make([]TabularColumnSpec, 0, len(spec.Columns))
	trailing := make([]TabularColumnSpec, 0, len(spec.Columns))
	for _, col := range spec.Columns {
		if col.Field == ExportFieldTotal {
			trailing = append(trailing, col)
			continue
		}
		leading = append(leading, col)
	}

	header := make([]string, 0, len(leading)+len(model.Days)+len(trailing))
	for _, col := range leading {
		header = append(header, col.Header)
	}
	header = append(header, model.Days...)
	for _, col := range trailing {
		header = append(header, col.Header)
	}

	lines := make([]string, 0, 1+len(model.PeriodTotals))
	lines = append(lines, joinDelimited(header, delimiter))

	dayMinutes := make([]map[string]int, len(model.DailyTotals))
	for i, day := range model.DailyTotals {
		m := make(map[string]int, len(day.Categories))
		for _, c := range day.Categories {
			m[categoryIdentity(c.Category)] = c.Minutes
		}
		dayMinutes[i] = m
	}

	for _, total := range model.PeriodTotals {
		identity := categoryIdentity(total.Category)
		row := make([]string, 0, len(header))
		for _, col := range leading {
			row = append(row, matrixRowField(col.Field, total.Category))
		}
		sum := 0
		for _, dayMap := range dayMinutes {
			minutes := dayMap[identity]
			sum += minutes
			row = append(row, minutesToDecimalHours(minutes))
		}
		for range trailing {
			row = append(row, minutesToDecimalHours(sum))
		}
		lines = append(lines, joinDelimited(row, delimiter))
	}
	return strings.Join(lines, "\n")
}

func matrixRowField(field string, cat ExportCategory) string {
	switch field {
	case ExportFieldCategoryName:
		return cat.Name
	case ExportFieldCategoryKey:
		if cat.Key != "" {
			return cat.Key
		}
		return cat.Name
	default:
		return ""
	}
}

func renderFlatRollupTabular(model PeriodExportModel, spec TabularTemplateSpec) string {
	delimiter := spec.Delimiter
	header := make([]string, len(spec.Columns))
	for i, col := range spec.Columns {
		header[i] = col.Header
	}
	lines := make([]string, 0, 1)
	lines = append(lines, joinDelimited(header, delimiter))

	for _, day := range model.DailyTotals {
		for _, cat := range day.Categories {
			row := make([]string, len(spec.Columns))
			for i, col := range spec.Columns {
				row[i] = flatRollupField(col.Field, day, cat)
			}
			lines = append(lines, joinDelimited(row, delimiter))
		}
	}
	return strings.Join(lines, "\n")
}

func flatRollupField(field string, day ExportDayTotals, cat ExportCategoryMinutes) string {
	switch field {
	case ExportFieldDate:
		return day.Date
	case ExportFieldCategoryName:
		return cat.Category.Name
	case ExportFieldCategoryKey:
		if cat.Category.Key != "" {
			return cat.Category.Key
		}
		return cat.Category.Name
	case ExportFieldHours:
		return minutesToDecimalHours(cat.Minutes)
	case ExportFieldMinutes:
		return fmt.Sprintf("%d", cat.Minutes)
	case ExportFieldDayActual:
		return minutesToDecimalHours(day.ActualMinutes)
	case ExportFieldDayTarget:
		return minutesToDecimalHours(day.TargetMinutes)
	default:
		return ""
	}
}

func renderDetailTabular(model PeriodExportModel, spec TabularTemplateSpec) string {
	delimiter := spec.Delimiter
	header := make([]string, len(spec.Columns))
	for i, col := range spec.Columns {
		header[i] = col.Header
	}
	lines := make([]string, 0, 1+len(model.Entries))
	lines = append(lines, joinDelimited(header, delimiter))

	entries := append([]ExportEntry(nil), model.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		if entries[i].StartMinutes != entries[j].StartMinutes {
			return entries[i].StartMinutes < entries[j].StartMinutes
		}
		if entries[i].Source != entries[j].Source {
			return entries[i].Source < entries[j].Source
		}
		return entries[i].SourceID < entries[j].SourceID
	})

	for _, entry := range entries {
		row := make([]string, len(spec.Columns))
		for i, col := range spec.Columns {
			row[i] = detailField(col.Field, entry)
		}
		lines = append(lines, joinDelimited(row, delimiter))
	}
	return strings.Join(lines, "\n")
}

func detailField(field string, entry ExportEntry) string {
	switch field {
	case ExportFieldDate:
		return entry.Day
	case ExportFieldStart:
		return exportEntryDateTime(entry.Day, entry.StartMinutes)
	case ExportFieldEnd:
		return exportEntryDateTime(entry.Day, entry.EndMinutes)
	case ExportFieldCategoryName:
		return entry.Category.Name
	case ExportFieldCategoryKey:
		if entry.Category.Key != "" {
			return entry.Category.Key
		}
		return entry.Category.Name
	case ExportFieldHours:
		return minutesToDecimalHours(entry.Minutes)
	case ExportFieldMinutes:
		return fmt.Sprintf("%d", entry.Minutes)
	case ExportFieldTitle:
		return entry.Title
	case ExportFieldDescription:
		return entry.Description
	case ExportFieldSource:
		return entry.Source
	default:
		return ""
	}
}

// Builtin tabular specs used when a seeded body is empty (older DBs).
func builtinTabularSpec(key string) (TabularTemplateSpec, bool) {
	switch key {
	case ExportTemplateMatrixCSV:
		return DefaultTabularSpec(ExportGrainRollup, ExportLayoutMatrix), true
	case ExportTemplateFlatDailyCSV:
		return DefaultTabularSpec(ExportGrainRollup, ExportLayoutFlat), true
	case ExportTemplateDetailEntriesCSV:
		return DefaultTabularSpec(ExportGrainDetail, ExportLayoutFlat), true
	default:
		return TabularTemplateSpec{}, false
	}
}
