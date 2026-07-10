-- +goose Up
-- +goose StatementBegin

UPDATE export_template
SET body = '{"version":1,"grain":"detail","layout":"flat","delimiter":",","columns":[{"field":"start","header":"Start"},{"field":"end","header":"End"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"},{"field":"title","header":"Title"},{"field":"description","header":"Description"}]}'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE export_template
SET body = '{"version":1,"grain":"detail","layout":"flat","delimiter":",","columns":[{"field":"start","header":"Start"},{"field":"end","header":"End"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"},{"field":"title","header":"Title"}]}'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd
