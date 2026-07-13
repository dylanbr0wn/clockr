-- +goose Up
-- +goose StatementBegin

-- Bitbucket workspaces and repos available as evidence sources for a connected account.
-- Selected workspaces/repos are used by later tickets when fetching commits.
-- Tokens live in the OS keychain; these tables are non-secret metadata only.
CREATE TABLE bitbucket_workspace (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id  TEXT    NOT NULL,
    external_id TEXT    NOT NULL,
    slug        TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    selected    INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (account_id, external_id)
);

CREATE INDEX idx_bitbucket_workspace_account
    ON bitbucket_workspace (account_id);

CREATE INDEX idx_bitbucket_workspace_selected
    ON bitbucket_workspace (account_id, selected)
    WHERE selected = 1;

CREATE TABLE bitbucket_repo (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id     TEXT    NOT NULL,
    workspace_uuid TEXT    NOT NULL,
    external_id    TEXT    NOT NULL,
    name           TEXT    NOT NULL,
    full_name      TEXT    NOT NULL,
    private        INTEGER NOT NULL DEFAULT 0 CHECK (private IN (0, 1)),
    selected       INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (account_id, external_id)
);

CREATE INDEX idx_bitbucket_repo_account
    ON bitbucket_repo (account_id);

CREATE INDEX idx_bitbucket_repo_workspace
    ON bitbucket_repo (account_id, workspace_uuid);

CREATE INDEX idx_bitbucket_repo_selected
    ON bitbucket_repo (account_id, selected)
    WHERE selected = 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS bitbucket_repo;
DROP TABLE IF EXISTS bitbucket_workspace;

-- +goose StatementEnd
