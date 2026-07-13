-- +goose Up
-- +goose StatementBegin

UPDATE export_template
SET description = 'One row per event or time entry with start, end, category, duration, and title.'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE export_template
SET description = 'One row per event or gap fill with start, end, category, duration, and title.'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd
