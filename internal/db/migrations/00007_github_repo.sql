-- +goose Up
-- +goose StatementBegin

-- GitHub repos available as evidence sources for a connected GitHub account.
-- Selected repos are used by later tickets (DYL-57+) when fetching commits/PRs.
-- Tokens live in the OS keychain; this table is non-secret metadata only.
CREATE TABLE github_repo (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id  TEXT    NOT NULL,
    external_id TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    full_name   TEXT    NOT NULL,
    private     INTEGER NOT NULL DEFAULT 0 CHECK (private IN (0, 1)),
    selected    INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (account_id, external_id)
);

CREATE INDEX idx_github_repo_account
    ON github_repo (account_id);

CREATE INDEX idx_github_repo_selected
    ON github_repo (account_id, selected)
    WHERE selected = 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS github_repo;

-- +goose StatementEnd
