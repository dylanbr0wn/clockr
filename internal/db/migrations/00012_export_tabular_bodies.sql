-- +goose Up
-- +goose StatementBegin

-- Allow TSV as a first-class tabular format alongside CSV.
-- SQLite cannot ALTER CHECK constraints; recreate the table.
CREATE TABLE export_template_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    format      TEXT    NOT NULL CHECK (format IN ('csv', 'tsv', 'text')),
    builtin     INTEGER NOT NULL DEFAULT 0 CHECK (builtin IN (0, 1)),
    body        TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO export_template_new (id, key, name, description, format, builtin, body, created_at)
SELECT id, key, name, description, format, builtin, body, created_at
FROM export_template;

DROP TABLE export_template;
ALTER TABLE export_template_new RENAME TO export_template;

-- Seed declarative tabular specs for builtin CSV presets (layout-only; no remapping).
UPDATE export_template
SET body = '{"version":1,"grain":"rollup","layout":"matrix","delimiter":",","columns":[{"field":"category_name","header":"Category"},{"field":"total","header":"Total"}]}'
WHERE key = 'matrix_csv';

UPDATE export_template
SET body = '{"version":1,"grain":"rollup","layout":"flat","delimiter":",","columns":[{"field":"date","header":"Date"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"}]}'
WHERE key = 'flat_daily_csv';

UPDATE export_template
SET body = '{"version":1,"grain":"detail","layout":"flat","delimiter":",","columns":[{"field":"start","header":"Start"},{"field":"end","header":"End"},{"field":"category_name","header":"Category"},{"field":"category_key","header":"Key"},{"field":"hours","header":"Hours"},{"field":"title","header":"Title"}]}'
WHERE key = 'detail_entries_csv';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE export_template SET body = '' WHERE key IN ('matrix_csv', 'flat_daily_csv', 'detail_entries_csv');

CREATE TABLE export_template_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    format      TEXT    NOT NULL CHECK (format IN ('csv', 'text')),
    builtin     INTEGER NOT NULL DEFAULT 0 CHECK (builtin IN (0, 1)),
    body        TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO export_template_old (id, key, name, description, format, builtin, body, created_at)
SELECT id, key, name, description, format, builtin, body, created_at
FROM export_template
WHERE format IN ('csv', 'text');

DROP TABLE export_template;
ALTER TABLE export_template_old RENAME TO export_template;

-- +goose StatementEnd
