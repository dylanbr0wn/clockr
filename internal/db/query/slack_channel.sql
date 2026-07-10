-- name: ListSlackChannels :many
SELECT * FROM slack_channel ORDER BY name;

-- name: ListSlackChannelsByAccount :many
SELECT * FROM slack_channel WHERE account_id = ? ORDER BY name;

-- name: ListSelectedSlackChannels :many
SELECT * FROM slack_channel WHERE selected = 1 ORDER BY name;

-- name: GetSlackChannel :one
SELECT * FROM slack_channel WHERE id = ?;

-- name: UpsertSlackChannel :one
INSERT INTO slack_channel (account_id, external_id, name, is_private, selected)
VALUES (?, ?, ?, ?, CASE WHEN ? = 1 THEN 1 ELSE 0 END)
ON CONFLICT (account_id, external_id) DO UPDATE SET
    name = excluded.name,
    is_private = excluded.is_private
RETURNING *;

-- name: SetSlackChannelSelected :exec
UPDATE slack_channel SET selected = ? WHERE id = ?;

-- name: DeleteSlackChannelsByAccount :exec
DELETE FROM slack_channel WHERE account_id = ?;
