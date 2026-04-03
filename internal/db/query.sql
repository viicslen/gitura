-- name: GetPRState :one
SELECT owner, repo, number, local_path
FROM pr_state
WHERE owner = ? AND repo = ? AND number = ?;

-- name: UpsertPRLocalPath :exec
INSERT INTO pr_state (owner, repo, number, local_path)
VALUES (?, ?, ?, ?)
ON CONFLICT (owner, repo, number) DO UPDATE SET local_path = excluded.local_path;
