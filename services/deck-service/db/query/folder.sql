-- name: CreateFolder :one
INSERT INTO folders (user_id, name, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFolderByID :one
SELECT * FROM folders WHERE folder_id = $1 LIMIT 1;

-- name: ListFoldersByUser :many
SELECT * FROM folders WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListPublicFolders :many
SELECT * FROM folders
WHERE is_public = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicFolders :one
SELECT COUNT(*) FROM folders WHERE is_public = TRUE;

-- name: ListPublicFoldersByUser :many
SELECT * FROM folders
WHERE user_id = $1 AND is_public = TRUE
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPublicFoldersByUser :one
SELECT COUNT(*) FROM folders WHERE user_id = $1 AND is_public = TRUE;

-- name: UpdateFolder :one
UPDATE folders
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE folder_id = sqlc.arg('folder_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: UpdateFolderVisibility :one
UPDATE folders
SET is_public  = sqlc.arg('is_public'),
    updated_at = now()
WHERE folder_id = sqlc.arg('folder_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: DeleteFolder :exec
DELETE FROM folders WHERE folder_id = $1 AND user_id = $2;
