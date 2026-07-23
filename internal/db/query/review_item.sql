-- name: GetReviewItem :one
SELECT * FROM review_item WHERE id = ?;

-- name: GetReviewItemByConflictKey :one
SELECT * FROM review_item
WHERE period_id = ? AND kind = ? AND conflict_key = ?
ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, created_at DESC
LIMIT 1;

-- name: ListOpenReviewItems :many
SELECT * FROM review_item WHERE period_id = ? AND status = 'open' ORDER BY created_at;

-- name: CreateReviewItem :one
INSERT INTO review_item (period_id, kind, event_id, payload, conflict_key)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateOpenReviewItemPayload :execrows
UPDATE review_item
SET payload = ?
WHERE id = ? AND status = 'open';

-- name: ReopenReviewItem :execrows
UPDATE review_item
SET status = 'open',
    payload = ?,
    event_id = ?,
    decision_action = '',
    decision_payload = '{}',
    resolved_at = NULL
WHERE id = ? AND status IN ('resolved', 'dismissed');

-- name: ResolveReviewItem :exec
UPDATE review_item
SET status = ?,
    decision_action = ?,
    decision_payload = ?,
    resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
