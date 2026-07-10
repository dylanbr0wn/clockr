-- +goose Up
-- +goose StatementBegin

-- Slack channels available as evidence sources for a connected workspace.
-- Selected channels are used by later tickets when fetching message history.
-- Tokens live in the OS keychain; this table is non-secret metadata only.
CREATE TABLE slack_channel (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id  TEXT    NOT NULL,
    external_id TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    is_private  INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0, 1)),
    selected    INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (account_id, external_id)
);

CREATE INDEX idx_slack_channel_account
    ON slack_channel (account_id);

CREATE INDEX idx_slack_channel_selected
    ON slack_channel (account_id, selected)
    WHERE selected = 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS slack_channel;

-- +goose StatementEnd
