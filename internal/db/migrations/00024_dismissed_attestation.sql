-- +goose Up
-- +goose StatementBegin

-- Widen time_entry.attestation to include dismissed (soft-reject).
-- SQLite cannot ALTER CHECK constraints; recreate the table.

CREATE TABLE time_entry_new (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id         INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    start_instant     TEXT    NOT NULL,                          -- RFC3339 UTC
    end_instant       TEXT    NOT NULL,                          -- RFC3339 UTC
    duration_minutes  INTEGER NOT NULL,
    local_work_date   TEXT    NOT NULL,                          -- YYYY-MM-DD (start's local date)
    category_id       INTEGER REFERENCES category (id) ON DELETE SET NULL,
    description       TEXT    NOT NULL DEFAULT '',
    attestation       TEXT    NOT NULL CHECK (attestation IN ('draft', 'confirmed', 'dismissed')),
    source_kind       TEXT,                                      -- optional provenance
    source_id         TEXT,
    source_revision   TEXT,
    method            TEXT,                                      -- e.g. gap_fill when stamping gap origin
    created_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    work_type         TEXT    NOT NULL DEFAULT 'worked'
        CHECK (work_type IN (
            'worked',
            'paid_leave',
            'unpaid_leave',
            'holiday',
            'break',
            'adjustment'
        )),
    project_id        INTEGER REFERENCES project (id) ON DELETE SET NULL,
    billable_status   TEXT    NOT NULL DEFAULT 'unset'
        CHECK (billable_status IN ('unset', 'billable', 'non_billable'))
);

INSERT INTO time_entry_new (
    id, period_id, start_instant, end_instant, duration_minutes, local_work_date,
    category_id, description, attestation, source_kind, source_id, source_revision,
    method, created_at, updated_at, work_type, project_id, billable_status
)
SELECT
    id, period_id, start_instant, end_instant, duration_minutes, local_work_date,
    category_id, description, attestation, source_kind, source_id, source_revision,
    method, created_at, updated_at, work_type, project_id, billable_status
FROM time_entry;

DROP TABLE time_entry;
ALTER TABLE time_entry_new RENAME TO time_entry;
CREATE INDEX idx_time_entry_period_date ON time_entry (period_id, local_work_date);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Narrow attestation back to draft|confirmed. Rows with dismissed cannot survive.
CREATE TABLE time_entry_old (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id         INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    start_instant     TEXT    NOT NULL,
    end_instant       TEXT    NOT NULL,
    duration_minutes  INTEGER NOT NULL,
    local_work_date   TEXT    NOT NULL,
    category_id       INTEGER REFERENCES category (id) ON DELETE SET NULL,
    description       TEXT    NOT NULL DEFAULT '',
    attestation       TEXT    NOT NULL CHECK (attestation IN ('draft', 'confirmed')),
    source_kind       TEXT,
    source_id         TEXT,
    source_revision   TEXT,
    method            TEXT,
    created_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    work_type         TEXT    NOT NULL DEFAULT 'worked'
        CHECK (work_type IN (
            'worked',
            'paid_leave',
            'unpaid_leave',
            'holiday',
            'break',
            'adjustment'
        )),
    project_id        INTEGER REFERENCES project (id) ON DELETE SET NULL,
    billable_status   TEXT    NOT NULL DEFAULT 'unset'
        CHECK (billable_status IN ('unset', 'billable', 'non_billable'))
);

INSERT INTO time_entry_old (
    id, period_id, start_instant, end_instant, duration_minutes, local_work_date,
    category_id, description, attestation, source_kind, source_id, source_revision,
    method, created_at, updated_at, work_type, project_id, billable_status
)
SELECT
    id, period_id, start_instant, end_instant, duration_minutes, local_work_date,
    category_id, description, attestation, source_kind, source_id, source_revision,
    method, created_at, updated_at, work_type, project_id, billable_status
FROM time_entry
WHERE attestation IN ('draft', 'confirmed');

DROP TABLE time_entry;
ALTER TABLE time_entry_old RENAME TO time_entry;
CREATE INDEX idx_time_entry_period_date ON time_entry (period_id, local_work_date);

-- +goose StatementEnd
