-- +goose Up
-- +goose StatementBegin

INSERT INTO export_template (key, name, description, format, builtin, body)
VALUES (
    'flat_daily_csv',
    'Flat daily totals',
    'One row per category per day with date, name, key, and decimal hours.',
    'csv',
    1,
    ''
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM export_template WHERE key = 'flat_daily_csv';
-- +goose StatementEnd
