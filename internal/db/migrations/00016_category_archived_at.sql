-- +goose Up
-- +goose StatementBegin
ALTER TABLE category ADD COLUMN archived_at TEXT; -- RFC3339 UTC when archived; NULL = active
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite cannot drop columns without rebuilding the table; leave added column in place on down.
-- +goose StatementEnd
