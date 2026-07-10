export type ExportGrain = "rollup" | "detail";
export type ExportLayout = "matrix" | "flat";

export interface TabularColumnSpec {
  field: string;
  header: string;
}

export interface TabularTemplateSpec {
  version: number;
  grain: ExportGrain;
  layout: ExportLayout;
  delimiter: "," | "\t";
  columns: TabularColumnSpec[];
}

export interface ExportFieldInfo {
  field: string;
  label: string;
  description: string;
}

const DEFAULT_HEADERS: Record<string, string> = {
  date: "Date",
  category_name: "Category",
  category_key: "Key",
  hours: "Hours",
  minutes: "Minutes",
  start: "Start",
  end: "End",
  title: "Title",
  source: "Source",
  day_actual_hours: "Day actual",
  day_target_hours: "Day target",
  total: "Total",
};

export function defaultFieldHeader(field: string): string {
  return DEFAULT_HEADERS[field] ?? field;
}

export function fieldCatalog(
  grain: ExportGrain,
  layout: ExportLayout,
): ExportFieldInfo[] {
  if (grain === "detail") {
    return [
      { field: "date", label: "Date", description: "Entry day (YYYY-MM-DD)" },
      { field: "start", label: "Start", description: "Start datetime" },
      { field: "end", label: "End", description: "End datetime" },
      {
        field: "category_name",
        label: "Category name",
        description: "Category display name",
      },
      {
        field: "category_key",
        label: "Category key",
        description: "Category key (falls back to name)",
      },
      {
        field: "hours",
        label: "Hours",
        description: "Duration as decimal hours",
      },
      {
        field: "minutes",
        label: "Minutes",
        description: "Duration as whole minutes",
      },
      {
        field: "title",
        label: "Title",
        description: "Event title or gap-fill note",
      },
      { field: "source", label: "Source", description: "event or gap_fill" },
    ];
  }
  if (layout === "matrix") {
    return [
      {
        field: "category_name",
        label: "Category name",
        description: "Row label (category name)",
      },
      {
        field: "category_key",
        label: "Category key",
        description: "Row label (category key)",
      },
      {
        field: "total",
        label: "Total",
        description: "Row total across days",
      },
    ];
  }
  return [
    { field: "date", label: "Date", description: "Day (YYYY-MM-DD)" },
    {
      field: "category_name",
      label: "Category name",
      description: "Category display name",
    },
    {
      field: "category_key",
      label: "Category key",
      description: "Category key (falls back to name)",
    },
    {
      field: "hours",
      label: "Hours",
      description: "Category minutes as decimal hours",
    },
    { field: "minutes", label: "Minutes", description: "Category minutes" },
    {
      field: "day_actual_hours",
      label: "Day actual hours",
      description: "Total tracked hours that day",
    },
    {
      field: "day_target_hours",
      label: "Day target hours",
      description: "Target hours that day",
    },
  ];
}

export function defaultTabularSpec(
  grain: ExportGrain = "rollup",
  layout: ExportLayout = "flat",
): TabularTemplateSpec {
  const resolvedLayout = grain === "detail" ? "flat" : layout;
  const columns = fieldCatalog(grain, resolvedLayout)
    .filter((field) => {
      if (grain === "detail") {
        return ["start", "end", "category_name", "category_key", "hours", "title"].includes(
          field.field,
        );
      }
      if (resolvedLayout === "matrix") {
        return ["category_name", "total"].includes(field.field);
      }
      return ["date", "category_name", "category_key", "hours"].includes(
        field.field,
      );
    })
    .map((field) => ({
      field: field.field,
      header: defaultFieldHeader(field.field),
    }));

  return {
    version: 1,
    grain,
    layout: resolvedLayout,
    delimiter: ",",
    columns,
  };
}

export function parseTabularSpec(body: string): TabularTemplateSpec | null {
  const trimmed = body.trim();
  if (!trimmed) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed) as TabularTemplateSpec;
    if (!parsed || !Array.isArray(parsed.columns) || parsed.columns.length === 0) {
      return null;
    }
    return {
      version: parsed.version || 1,
      grain: parsed.grain === "detail" ? "detail" : "rollup",
      layout: parsed.layout === "matrix" ? "matrix" : "flat",
      delimiter: parsed.delimiter === "\t" ? "\t" : ",",
      columns: parsed.columns.map((column) => ({
        field: column.field,
        header: column.header?.trim() || defaultFieldHeader(column.field),
      })),
    };
  } catch {
    return null;
  }
}

export function encodeTabularSpec(spec: TabularTemplateSpec): string {
  return JSON.stringify({
    version: 1,
    grain: spec.grain,
    layout: spec.layout,
    delimiter: spec.delimiter,
    columns: spec.columns,
  });
}

export function formatFromSpec(spec: TabularTemplateSpec): "csv" | "tsv" {
  return spec.delimiter === "\t" ? "tsv" : "csv";
}

export function isTabularFormat(format: string): boolean {
  return format === "csv" || format === "tsv";
}

export function isTextFormat(format: string): boolean {
  return format === "text";
}

/** Starter body for new custom text templates (Go text/template). */
export const DEFAULT_TEXT_TEMPLATE_BODY = `Period: {{.PeriodLabel}}
{{.StartDate}} to {{.EndDate}}

Target: {{duration .TargetMinutes}} ({{hoursPerDay .TargetHoursPerDay}}h/day)
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
{{end}}`;
