-- name: ListBitbucketWorkspaces :many
SELECT * FROM bitbucket_workspace ORDER BY name;

-- name: ListBitbucketWorkspacesByAccount :many
SELECT * FROM bitbucket_workspace WHERE account_id = ? ORDER BY name;

-- name: UpsertBitbucketWorkspace :one
INSERT INTO bitbucket_workspace (account_id, external_id, slug, name, selected)
VALUES (?, ?, ?, ?, CASE WHEN ? = 1 THEN 1 ELSE 0 END)
ON CONFLICT (account_id, external_id) DO UPDATE SET
    slug = excluded.slug,
    name = excluded.name
RETURNING *;

-- name: SetBitbucketWorkspaceSelected :exec
UPDATE bitbucket_workspace SET selected = ? WHERE id = ?;

-- name: DeleteBitbucketWorkspacesByAccount :exec
DELETE FROM bitbucket_workspace WHERE account_id = ?;
