-- +goose Up
-- +goose StatementBegin

UPDATE export_template
SET body = '{"version":1,"grain":"detail","layout":"flat","delimiter":",","columns":[{"field":"start","header":"Start"},{"field":"end","header":"End"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"},{"field":"title","header":"Title"},{"field":"description","header":"Description"},{"field":"work_type","header":"Work type"},{"field":"project_name","header":"Project"},{"field":"project_key","header":"Project key"},{"field":"billable_status","header":"Billable"}]}',
    description = 'One row per event or time entry with start, end, category, duration, title, and allocation fields.'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE export_template
SET body = '{"version":1,"grain":"detail","layout":"flat","delimiter":",","columns":[{"field":"start","header":"Start"},{"field":"end","header":"End"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"},{"field":"title","header":"Title"},{"field":"description","header":"Description"}]}',
    description = 'One row per event or time entry with start, end, category, duration, and title.'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd
