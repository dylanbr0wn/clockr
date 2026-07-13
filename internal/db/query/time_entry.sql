-- name: ListTimeEntriesForPeriod :many
SELECT * FROM time_entry WHERE period_id = ? ORDER BY local_work_date, start_instant;

-- name: ListTimeEntriesForDay :many
SELECT * FROM time_entry WHERE period_id = ? AND local_work_date = ? ORDER BY start_instant;

-- name: GetTimeEntry :one
SELECT * FROM time_entry WHERE id = ? AND period_id = ?;

-- name: CreateTimeEntry :one
INSERT INTO time_entry (
    period_id,
    start_instant,
    end_instant,
    duration_minutes,
    local_work_date,
    category_id,
    description,
    attestation,
    source_kind,
    source_id,
    source_revision,
    method,
    work_type,
    project_id,
    billable_status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTimeEntry :one
UPDATE time_entry SET
    start_instant     = ?,
    end_instant       = ?,
    duration_minutes  = ?,
    local_work_date   = ?,
    category_id       = ?,
    description       = ?,
    work_type         = ?,
    project_id        = ?,
    billable_status   = ?,
    updated_at        = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND period_id = ?
RETURNING *;

-- name: UpdateTimeEntrySpan :one
UPDATE time_entry SET
    start_instant     = ?,
    end_instant       = ?,
    duration_minutes  = ?,
    local_work_date   = ?,
    updated_at        = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND period_id = ?
RETURNING *;

-- name: DeleteTimeEntry :execrows
DELETE FROM time_entry WHERE id = ? AND period_id = ?;
