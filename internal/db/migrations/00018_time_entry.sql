-- +goose Up
-- +goose StatementBegin

-- Canonical interval-based time ledger. Replaces gap_fill via destructive cutover
-- (no row migrate). Provenance columns are optional; attestation is draft|confirmed.
CREATE TABLE time_entry (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id         INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    start_instant     TEXT    NOT NULL,                          -- RFC3339 UTC
    end_instant       TEXT    NOT NULL,                          -- RFC3339 UTC
    duration_minutes  INTEGER NOT NULL,
    local_work_date   TEXT    NOT NULL,                          -- YYYY-MM-DD (start's local date)
    category_id       INTEGER REFERENCES category (id) ON DELETE SET NULL,
    description       TEXT    NOT NULL DEFAULT '',
    attestation       TEXT    NOT NULL CHECK (attestation IN ('draft', 'confirmed')),
    source_kind       TEXT,                                      -- optional provenance
    source_id         TEXT,
    source_revision   TEXT,
    method            TEXT,                                      -- e.g. gap_fill when stamping gap origin
    created_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_time_entry_period_date ON time_entry (period_id, local_work_date);

DROP TABLE gap_fill;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Destructive cutover: down recreates an empty gap_fill shell and drops time_entry.
-- Row data is not restored.
CREATE TABLE gap_fill (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id   INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    day         TEXT    NOT NULL,
    start_utc   TEXT    NOT NULL,
    end_utc     TEXT    NOT NULL,
    category_id INTEGER REFERENCES category (id) ON DELETE SET NULL,
    note        TEXT    NOT NULL DEFAULT '',
    description TEXT    NOT NULL DEFAULT '',
    source      TEXT    NOT NULL DEFAULT 'gap' CHECK (source IN ('gap', 'manual', 'extend')),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_gap_fill_period_day ON gap_fill (period_id, day);

DROP TABLE time_entry;

-- +goose StatementEnd
