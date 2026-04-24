-- name: CreateFolder :one
INSERT INTO folders (user_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetFolderByID :one
SELECT * FROM folders WHERE folder_id = $1 LIMIT 1;

-- name: ListFoldersByUser :many
SELECT * FROM folders WHERE user_id = $1 ORDER BY created_at DESC;

-- name: UpdateFolder :one
UPDATE folders
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE folder_id = sqlc.arg('folder_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: DeleteFolder :exec
DELETE FROM folders WHERE folder_id = $1 AND user_id = $2;
