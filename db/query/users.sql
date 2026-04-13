-- name: CreateUser :one
INSERT INTO users (
  username,
  email,
  password_hash,
  full_name,
  avatar_url,
  role
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE user_id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateUserProfile :one
UPDATE users
SET full_name = COALESCE(sqlc.narg(full_name), full_name),
    avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
    last_login = COALESCE(sqlc.narg(last_login), last_login)
WHERE user_id = sqlc.arg(user_id)
RETURNING *;
