-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, full_name, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE user_id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET full_name  = COALESCE(sqlc.narg('full_name'), full_name),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    updated_at = now()
WHERE user_id = sqlc.arg('user_id')
RETURNING *;

-- name: UpdatePassword :exec
UPDATE users
SET password_hash = $2,
    updated_at    = now()
WHERE user_id = $1;

-- name: UpdateLastLogin :exec
UPDATE users
SET last_login_at = now(),
    updated_at    = now()
WHERE user_id = $1;

-- name: MarkEmailVerified :exec
UPDATE users
SET email_verified = TRUE,
    updated_at     = now()
WHERE user_id = $1;

-- name: BanUser :exec
UPDATE users
SET is_banned     = TRUE,
    banned_at     = now(),
    banned_reason = $2,
    updated_at    = now()
WHERE user_id = $1;

-- name: UnbanUser :exec
UPDATE users
SET is_banned     = FALSE,
    banned_at     = NULL,
    banned_reason = NULL,
    updated_at    = now()
WHERE user_id = $1;
