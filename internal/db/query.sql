-- name: GetPRState :one
SELECT owner, repo, number, local_path
FROM pr_state
WHERE owner = ? AND repo = ? AND number = ?;

-- name: GetRepoLocalPath :one
SELECT local_path
FROM pr_state
WHERE owner = ? AND repo = ? AND local_path != ''
ORDER BY rowid DESC
LIMIT 1;

-- name: UpsertPRLocalPath :exec
INSERT INTO pr_state (owner, repo, number, local_path)
VALUES (?, ?, ?, ?)
ON CONFLICT (owner, repo, number) DO UPDATE SET local_path = excluded.local_path;

-- name: ListSubmoduleViewed :many
SELECT submodule, path, pointer, fingerprint
FROM submodule_file_viewed
WHERE owner = ? AND repo = ? AND number = ?;

-- name: UpsertSubmoduleViewed :exec
INSERT INTO submodule_file_viewed (owner, repo, number, submodule, path, pointer, fingerprint)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (owner, repo, number, submodule, path, pointer) DO UPDATE SET fingerprint = excluded.fingerprint;

-- name: DeleteSubmoduleViewed :exec
DELETE FROM submodule_file_viewed
WHERE owner = ? AND repo = ? AND number = ? AND submodule = ? AND path = ? AND pointer = ?;
