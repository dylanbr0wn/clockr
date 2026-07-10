-- +goose Up
-- +goose StatementBegin
ALTER TABLE gap_fill ADD COLUMN description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite cannot drop columns without rebuilding the table; leave added column in place on down.
-- +goose StatementEnd
