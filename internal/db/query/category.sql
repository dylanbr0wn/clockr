-- name: ListCategories :many
SELECT * FROM category WHERE archived_at IS NULL ORDER BY name;

-- name: ListAllCategories :many
SELECT * FROM category ORDER BY name;

-- name: GetCategory :one
SELECT * FROM category WHERE id = ?;

-- name: GetCategoryByKey :one
SELECT * FROM category WHERE key = ? AND archived_at IS NULL;

-- name: GetDefaultGapCategory :one
SELECT * FROM category WHERE is_default_gap = 1 AND archived_at IS NULL;

-- name: CreateCategory :one
INSERT INTO category (name, description, key, is_default_gap, color)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateCategory :exec
UPDATE category
SET name = ?, description = ?, key = ?, color = ?
WHERE id = ?;

-- name: ClearDefaultGap :exec
UPDATE category SET is_default_gap = 0 WHERE is_default_gap = 1;

-- name: SetDefaultGap :exec
UPDATE category SET is_default_gap = 1 WHERE id = ?;

-- name: ArchiveCategory :one
UPDATE category
SET archived_at = ?, is_default_gap = 0
WHERE id = ?
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM category WHERE id = ?;

-- name: CountOverlayReferencesToCategory :one
SELECT COUNT(*) FROM overlay WHERE category_id = ?;

-- name: CountMemoryReferencesToCategory :one
SELECT COUNT(*) FROM memory WHERE category_id = ?;

-- name: CountCalendarReferencesToCategory :one
SELECT COUNT(*) FROM calendar WHERE default_category_id = ?;

-- name: CountTimeEntryReferencesToCategory :one
SELECT COUNT(*) FROM time_entry WHERE category_id = ?;
