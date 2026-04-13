-- name: CreateDeck :one
INSERT INTO decks (
  user_id,
  name,
  description,
  is_public,
  status,
  settings
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetDeck :one
SELECT * FROM decks
WHERE deck_id = $1 LIMIT 1;

-- name: ListUserDecks :many
SELECT * FROM decks
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPublicActiveDecks :many
SELECT * FROM decks
WHERE is_public = true
  AND status = 'active'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateDeck :one
UPDATE decks
SET name = COALESCE(sqlc.narg(name), name),
    description = COALESCE(sqlc.narg(description), description),
    is_public = COALESCE(sqlc.narg(is_public), is_public),
    status = COALESCE(sqlc.narg(status), status),
    settings = COALESCE(sqlc.narg(settings), settings),
    updated_at = CURRENT_TIMESTAMP
WHERE deck_id = sqlc.arg(deck_id)
RETURNING *;
