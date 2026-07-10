-- +goose Up
-- +goose StatementBegin

-- Named export presets. Builtin rows are code-rendered (body may be empty);
-- later tickets add user templates and text/template bodies.
CREATE TABLE export_template (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    format      TEXT    NOT NULL CHECK (format IN ('csv', 'text')),
    builtin     INTEGER NOT NULL DEFAULT 0 CHECK (builtin IN (0, 1)),
    body        TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO export_template (key, name, description, format, builtin, body)
VALUES (
    'matrix_csv',
    'Category × day matrix',
    'Category rows by day columns with decimal hours and a Total column.',
    'csv',
    1,
    ''
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS export_template;
-- +goose StatementEnd
