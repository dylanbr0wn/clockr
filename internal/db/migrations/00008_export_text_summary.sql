-- +goose Up
-- +goose StatementBegin

INSERT INTO export_template (key, name, description, format, builtin, body)
VALUES (
    'text_summary',
    'Text summary',
    'Human-readable period summary for clipboard copy.',
    'text',
    1,
    'Period: {{.PeriodLabel}}
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
{{end}}'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM export_template WHERE key = 'text_summary';
-- +goose StatementEnd
