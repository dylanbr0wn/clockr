-- +goose Up
-- +goose StatementBegin

-- Add source_drift to review_item.kind CHECK.
-- SQLite cannot ALTER CHECK constraints; recreate the table.

CREATE TABLE review_item_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id        INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    kind             TEXT    NOT NULL CHECK (kind IN (
                         'new_in_gap', 'title_changed', 'deleted_categorized',
                         'dedup_ambiguous', 'overlap', 'tentative', 'all_day',
                         'source_drift')),
    event_id         INTEGER REFERENCES event (id) ON DELETE CASCADE,
    payload          TEXT    NOT NULL DEFAULT '{}',
    status           TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    resolved_at      TEXT,
    conflict_key     TEXT    NOT NULL DEFAULT '',
    decision_action  TEXT    NOT NULL DEFAULT '',
    decision_payload TEXT    NOT NULL DEFAULT '{}'
);

INSERT INTO review_item_new (
    id, period_id, kind, event_id, payload, status, created_at, resolved_at,
    conflict_key, decision_action, decision_payload
)
SELECT
    id, period_id, kind, event_id, payload, status, created_at, resolved_at,
    conflict_key, decision_action, decision_payload
FROM review_item;

DROP TABLE review_item;
ALTER TABLE review_item_new RENAME TO review_item;

CREATE INDEX idx_review_item_period_status ON review_item (period_id, status);
CREATE UNIQUE INDEX idx_review_item_period_kind_conflict_key
    ON review_item (period_id, kind, conflict_key)
    WHERE conflict_key <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE review_item_old (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    period_id        INTEGER NOT NULL REFERENCES period (id) ON DELETE CASCADE,
    kind             TEXT    NOT NULL CHECK (kind IN (
                         'new_in_gap', 'title_changed', 'deleted_categorized',
                         'dedup_ambiguous', 'overlap', 'tentative', 'all_day')),
    event_id         INTEGER REFERENCES event (id) ON DELETE CASCADE,
    payload          TEXT    NOT NULL DEFAULT '{}',
    status           TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    resolved_at      TEXT,
    conflict_key     TEXT    NOT NULL DEFAULT '',
    decision_action  TEXT    NOT NULL DEFAULT '',
    decision_payload TEXT    NOT NULL DEFAULT '{}'
);

INSERT INTO review_item_old (
    id, period_id, kind, event_id, payload, status, created_at, resolved_at,
    conflict_key, decision_action, decision_payload
)
SELECT
    id, period_id, kind, event_id, payload, status, created_at, resolved_at,
    conflict_key, decision_action, decision_payload
FROM review_item
WHERE kind <> 'source_drift';

DROP TABLE review_item;
ALTER TABLE review_item_old RENAME TO review_item;

CREATE INDEX idx_review_item_period_status ON review_item (period_id, status);
CREATE UNIQUE INDEX idx_review_item_period_kind_conflict_key
    ON review_item (period_id, kind, conflict_key)
    WHERE conflict_key <> '';

-- +goose StatementEnd
