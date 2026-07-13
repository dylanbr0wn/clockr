-- +goose Up
-- +goose StatementBegin

-- Project master (Category-like lifecycle: unused → hard delete; referenced → archive).
-- Optional color; no effective dates / external IDs in v1.
CREATE TABLE project (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE,
    key         TEXT    NOT NULL DEFAULT '',
    color       TEXT,                             -- optional; NULL = unset
    archived_at TEXT,                             -- RFC3339 UTC when archived; NULL = active
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE UNIQUE INDEX idx_project_key ON project (key);

-- Allocation dims on TimeEntry. Defaults backfill existing rows.
ALTER TABLE time_entry ADD COLUMN work_type TEXT NOT NULL DEFAULT 'worked'
    CHECK (work_type IN (
        'worked',
        'paid_leave',
        'unpaid_leave',
        'holiday',
        'break',
        'adjustment'
    ));
ALTER TABLE time_entry ADD COLUMN project_id INTEGER REFERENCES project (id) ON DELETE SET NULL;
ALTER TABLE time_entry ADD COLUMN billable_status TEXT NOT NULL DEFAULT 'unset'
    CHECK (billable_status IN ('unset', 'billable', 'non_billable'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite cannot drop columns without rebuilding the table; leave added columns in place.
DROP INDEX IF EXISTS idx_project_key;
DROP TABLE IF EXISTS project;

-- +goose StatementEnd
