-- name: GetPeriod :one
SELECT * FROM period WHERE id = ?;

-- name: GetPeriodByRange :one
SELECT * FROM period WHERE start_date = ? AND end_date = ?;

-- name: ListPeriods :many
SELECT * FROM period ORDER BY start_date DESC;

-- name: CreatePeriod :one
INSERT INTO period (start_date, end_date, cadence, anchor_date)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: TouchPeriodSynced :exec
UPDATE period SET last_synced_at = ? WHERE id = ?;

-- name: DeletePeriod :exec
DELETE FROM period WHERE id = ?;

-- name: ListTzSegments :many
SELECT * FROM tz_segment WHERE period_id = ? ORDER BY effective_from_date;

-- name: UpsertTzSegment :one
INSERT INTO tz_segment (period_id, effective_from_date, iana_tz)
VALUES (?, ?, ?)
ON CONFLICT (period_id, effective_from_date) DO UPDATE SET iana_tz = excluded.iana_tz
RETURNING *;

-- name: DeleteTzSegment :exec
DELETE FROM tz_segment WHERE id = ?;
