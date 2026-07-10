-- +goose Up
-- +goose StatementBegin

INSERT INTO export_template (key, name, description, format, builtin, body)
VALUES (
    'detail_entries_csv',
    'Detail entries',
    'One row per event or gap fill with start, end, category, duration, and title.',
    'csv',
    1,
    ''
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM export_template WHERE key = 'detail_entries_csv';
-- +goose StatementEnd
