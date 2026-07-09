-- +goose Up
-- +goose StatementBegin
ALTER TABLE category ADD COLUMN color TEXT NOT NULL DEFAULT '#64748B';
UPDATE category SET color = CASE name
	WHEN 'Meetings' THEN '#0EA5E9'
	WHEN 'Deep Work' THEN '#8B5CF6'
	WHEN 'Admin' THEN '#F59E0B'
	WHEN 'Email & Comms' THEN '#14B8A6'
	WHEN 'Breaks' THEN '#64748B'
	ELSE '#64748B'
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite cannot drop columns without rebuilding the table; leave added column in place on down.
-- +goose StatementEnd
