-- name: ListProjects :many
SELECT * FROM project WHERE archived_at IS NULL ORDER BY name;

-- name: ListAllProjects :many
SELECT * FROM project ORDER BY name;

-- name: GetProject :one
SELECT * FROM project WHERE id = ?;

-- name: GetProjectByKey :one
SELECT * FROM project WHERE key = ? AND archived_at IS NULL;

-- name: CreateProject :one
INSERT INTO project (name, key, color)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateProject :one
UPDATE project
SET name = ?, key = ?, color = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: ArchiveProject :one
UPDATE project
SET archived_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM project WHERE id = ?;

-- name: CountTimeEntryReferencesToProject :one
SELECT COUNT(*) FROM time_entry WHERE project_id = ?;
