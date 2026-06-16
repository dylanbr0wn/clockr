-- name: ListGapFillsForPeriod :many
SELECT * FROM gap_fill WHERE period_id = ? ORDER BY day, start_utc;

-- name: ListGapFillsForDay :many
SELECT * FROM gap_fill WHERE period_id = ? AND day = ? ORDER BY start_utc;

-- name: CreateGapFill :one
INSERT INTO gap_fill (period_id, day, start_utc, end_utc, category_id, note, source)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateGapFill :one
UPDATE gap_fill SET
    day         = ?,
    start_utc   = ?,
    end_utc     = ?,
    category_id = ?,
    note        = ?,
    updated_at  = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND period_id = ? AND source = 'manual'
RETURNING *;

-- name: DeleteManualGapFill :execrows
DELETE FROM gap_fill WHERE id = ? AND period_id = ? AND source = 'manual';
