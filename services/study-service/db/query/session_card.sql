-- name: InsertSessionCard :one
INSERT INTO session_cards (session_id, position, card_id, user_card_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListSessionCards :many
SELECT * FROM session_cards
WHERE session_id = $1
ORDER BY position;

-- name: MarkSessionCardReviewed :one
UPDATE session_cards SET
    reviewed_at = NOW(),
    rating      = $3
WHERE session_id = $1 AND card_id = $2
RETURNING *;

-- name: GetSessionCardByCard :one
SELECT * FROM session_cards
WHERE session_id = $1 AND card_id = $2;
