-- name: ListBitbucketRepos :many
SELECT * FROM bitbucket_repo ORDER BY full_name;

-- name: ListBitbucketReposByAccount :many
SELECT * FROM bitbucket_repo WHERE account_id = ? ORDER BY full_name;

-- name: ListSelectedBitbucketRepos :many
SELECT * FROM bitbucket_repo WHERE selected = 1 ORDER BY full_name;

-- name: UpsertBitbucketRepo :one
INSERT INTO bitbucket_repo (account_id, workspace_uuid, external_id, name, full_name, private, selected)
VALUES (?, ?, ?, ?, ?, ?, CASE WHEN ? = 1 THEN 1 ELSE 0 END)
ON CONFLICT (account_id, external_id) DO UPDATE SET
    workspace_uuid = excluded.workspace_uuid,
    name = excluded.name,
    full_name = excluded.full_name,
    private = excluded.private
RETURNING *;

-- name: SetBitbucketRepoSelected :exec
UPDATE bitbucket_repo SET selected = ? WHERE id = ?;

-- name: DeleteBitbucketReposByAccount :exec
DELETE FROM bitbucket_repo WHERE account_id = ?;
