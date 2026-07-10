-- name: ListGitHubRepos :many
SELECT * FROM github_repo ORDER BY full_name;

-- name: ListGitHubReposByAccount :many
SELECT * FROM github_repo WHERE account_id = ? ORDER BY full_name;

-- name: ListSelectedGitHubRepos :many
SELECT * FROM github_repo WHERE selected = 1 ORDER BY full_name;

-- name: GetGitHubRepo :one
SELECT * FROM github_repo WHERE id = ?;

-- name: UpsertGitHubRepo :one
INSERT INTO github_repo (account_id, external_id, name, full_name, private, selected)
VALUES (?, ?, ?, ?, ?, CASE WHEN ? = 1 THEN 1 ELSE 0 END)
ON CONFLICT (account_id, external_id) DO UPDATE SET
    name = excluded.name,
    full_name = excluded.full_name,
    private = excluded.private
RETURNING *;

-- name: SetGitHubRepoSelected :exec
UPDATE github_repo SET selected = ? WHERE id = ?;

-- name: DeleteGitHubReposByAccount :exec
DELETE FROM github_repo WHERE account_id = ?;
