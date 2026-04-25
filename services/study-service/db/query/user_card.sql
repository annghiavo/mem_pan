-- name: UpsertUserCard :one
INSERT INTO user_cards (user_id, card_id, deck_id)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, card_id) DO UPDATE
    SET updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetUserCardByID :one
SELECT * FROM user_cards WHERE user_card_id = $1;

-- name: GetUserCardByUserAndCard :one
SELECT * FROM user_cards WHERE user_id = $1 AND card_id = $2;

-- name: UpdateUserCardFSRS :one
UPDATE user_cards SET
    state            = $2,
    stability        = $3,
    difficulty       = $4,
    reps             = $5,
    lapses           = $6,
    scheduled_days   = $7,
    next_review_date = $8,
    last_review_date = $9,
    updated_at       = CURRENT_TIMESTAMP
WHERE user_card_id = $1
RETURNING *;

-- name: ListDueUserCards :many
SELECT * FROM user_cards
WHERE user_id = $1
  AND state != 'new'
  AND next_review_date <= NOW()
ORDER BY next_review_date
LIMIT $2;

-- name: ListDueUserCardsByDeck :many
SELECT * FROM user_cards
WHERE user_id = $1
  AND deck_id = $2
  AND state != 'new'
  AND next_review_date <= NOW()
ORDER BY next_review_date
LIMIT $3;

-- name: ListNewUserCardsByDeck :many
SELECT * FROM user_cards
WHERE user_id = $1
  AND deck_id = $2
  AND state = 'new'
ORDER BY created_at
LIMIT $3;

-- name: ListUserCardsByDeck :many
SELECT * FROM user_cards
WHERE user_id = $1 AND deck_id = $2
ORDER BY state, created_at;
