-- +goose Up
-- +goose StatementBegin
ALTER TABLE category ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE category ADD COLUMN key TEXT NOT NULL DEFAULT '';
UPDATE category SET key = name WHERE key = '';
CREATE UNIQUE INDEX idx_category_key ON category (key);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_category_key;
-- SQLite cannot drop columns without rebuilding the table; leave added columns in place on down.
-- +goose StatementEnd
