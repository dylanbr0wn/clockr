-- name: GetReviewItem :one
SELECT * FROM review_item WHERE id = ?;

-- name: ListOpenReviewItems :many
SELECT * FROM review_item WHERE period_id = ? AND status = 'open' ORDER BY created_at;

-- name: CreateReviewItem :one
INSERT INTO review_item (period_id, kind, event_id, payload)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ResolveReviewItem :exec
UPDATE review_item
SET status = ?, resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
