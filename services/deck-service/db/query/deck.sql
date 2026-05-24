-- name: CreateDeck :one
INSERT INTO decks (user_id, name, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE deck_id = $1 LIMIT 1;

-- name: ListDecksByUser :many
SELECT * FROM decks
WHERE user_id = $1 AND status != 'deleted'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDecksByUser :one
SELECT COUNT(*) FROM decks WHERE user_id = $1 AND status != 'deleted';

-- name: ListPublicDecks :many
SELECT * FROM decks
WHERE is_public = TRUE AND status = 'active'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicDecks :one
SELECT COUNT(*) FROM decks WHERE is_public = TRUE AND status = 'active';

-- name: ListPublicDecksByUser :many
SELECT * FROM decks
WHERE user_id = $1 AND is_public = TRUE AND status = 'active'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPublicDecksByUser :one
SELECT COUNT(*) FROM decks WHERE user_id = $1 AND is_public = TRUE AND status = 'active';

-- name: UpdateDeck :one
UPDATE decks
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE deck_id = sqlc.arg('deck_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: UpdateDeckSettings :one
UPDATE decks
SET settings   = sqlc.arg('settings')::jsonb,
    updated_at = now()
WHERE deck_id = sqlc.arg('deck_id')
RETURNING *;

-- name: UpdateDeckVisibility :one
UPDATE decks
SET is_public  = sqlc.arg('is_public'),
    updated_at = now()
WHERE deck_id = sqlc.arg('deck_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: SoftDeleteDeck :exec
UPDATE decks
SET status     = 'deleted',
    updated_at = now()
WHERE deck_id = $1 AND user_id = $2;

-- name: AdminUpdateDeckStatus :one
UPDATE decks
SET status     = $2,
    updated_at = now()
WHERE deck_id = $1
RETURNING *;

-- name: AdminListDecks :many
SELECT * FROM decks
WHERE status::text = COALESCE(sqlc.narg('status_filter')::text, status::text)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountDecks :one
SELECT COUNT(*) FROM decks
WHERE status::text = COALESCE(sqlc.narg('status_filter')::text, status::text);

-- name: IncrementCardCount :exec
UPDATE decks SET card_count = card_count + 1, updated_at = now() WHERE deck_id = $1;

-- name: DecrementCardCount :exec
UPDATE decks SET card_count = GREATEST(card_count - 1, 0), updated_at = now() WHERE deck_id = $1;

-- name: CloneDeck :one
INSERT INTO decks (user_id, name, description, is_public, cloned_from)
VALUES ($1, $2, $3, FALSE, $4)
RETURNING *;
