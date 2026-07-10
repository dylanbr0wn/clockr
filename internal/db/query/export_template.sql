-- name: ListExportTemplates :many
SELECT * FROM export_template ORDER BY builtin DESC, name;

-- name: GetExportTemplateByKey :one
SELECT * FROM export_template WHERE key = ?;

-- name: GetExportTemplate :one
SELECT * FROM export_template WHERE id = ?;

-- name: CreateExportTemplate :one
INSERT INTO export_template (key, name, description, format, builtin, body)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateExportTemplate :one
UPDATE export_template
SET name = ?, description = ?, format = ?, body = ?
WHERE id = ? AND builtin = 0
RETURNING *;

-- name: DeleteExportTemplate :execrows
DELETE FROM export_template WHERE id = ? AND builtin = 0;

-- name: ExportTemplateKeyExists :one
SELECT COUNT(*) FROM export_template WHERE key = ?;
